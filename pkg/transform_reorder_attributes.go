package pkg

import (
	"fmt"
	"sort"
	"strings"

	"github.com/Azure/golden"
	"github.com/Azure/mapotf/pkg/terraform"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/hashicorp/hcl/v2/hclwrite"
)

var _ Transform = &ReorderAttributesTransform{}

// ReorderAttributesTransform re-orders the attributes and nested blocks of one
// root block.
//
// Semantics:
//   - "Element" = an attribute (name = value) or a nested block. Nested blocks
//     are identified by their type name, except for `dynamic "X"` blocks which
//     are identified by their label (`X`), so they line up with the static
//     block they emit.
//   - Elements named in `head_attributes` come first, in the listed order.
//   - Elements named in `foot_attributes` come last, in the listed order.
//   - Elements named in `body_attributes` come first within the body section,
//     in the listed order. Every other element of the body section is then
//     appended; by default that remainder is sorted alphabetically by name.
//     Set `sort_body_alphabetically = false` to preserve original source order
//     for the unlisted body remainder instead.
//   - When `head_foot_line_breaks` is true (default) a blank line is inserted
//     between the head section and the body, and between the body and the
//     foot section. Set to `false` to suppress these blank lines.
//   - Every nested block in the output is preceded by a blank line, except
//     when the previous element is a nested block sharing the same orderable
//     name. Adjacent same-kind siblings (e.g. two `validation {}` blocks of
//     a variable, two `dynamic "subnet" {}` blocks of a resource) stay
//     grouped without a blank line between them, matching typical Terraform
//     formatting. If a section boundary (head/foot) coincides with the
//     adjacency the section blank line still wins.
//   - Names in `head_attributes` / `body_attributes` / `foot_attributes` that
//     are not present on the block are silently skipped.
//   - The same name appearing in more than one of `head_attributes`,
//     `body_attributes`, or `foot_attributes` is a configuration error.
//
// This transform never adds, removes, or mutates attribute values — only the
// layout changes.
type ReorderAttributesTransform struct {
	*golden.BaseBlock
	*BaseTransform
	TargetBlockAddress     string   `hcl:"target_block_address"`
	HeadAttributes         []string `hcl:"head_attributes,optional"`
	BodyAttributes         []string `hcl:"body_attributes,optional"`
	FootAttributes         []string `hcl:"foot_attributes,optional"`
	HeadFootLineBreaks     *bool    `hcl:"head_foot_line_breaks,optional"`
	SortBodyAlphabetically *bool    `hcl:"sort_body_alphabetically,optional"`
}

func (r *ReorderAttributesTransform) Type() string {
	return "reorder_attributes"
}

func (r *ReorderAttributesTransform) Apply() error {
	cfg := r.Config().(*MetaProgrammingTFConfig)
	block := cfg.RootBlock(r.TargetBlockAddress)
	if block == nil {
		return fmt.Errorf("cannot find block: %s", r.TargetBlockAddress)
	}

	if err := validateReorderSectionOverlap(r.HeadAttributes, r.BodyAttributes, r.FootAttributes); err != nil {
		return err
	}

	body := block.WriteBlock.Body()
	writeAttrs := body.Attributes()
	writeBlocks := body.Blocks()
	if len(writeAttrs) == 0 && len(writeBlocks) == 0 {
		return nil
	}

	elements := buildReorderElements(writeAttrs, writeBlocks, block)

	headElems, listedBodyElems, unlistedBodyElems, footElems := partitionReorderElements(elements, r.HeadAttributes, r.BodyAttributes, r.FootAttributes)
	sortReorderBody(unlistedBodyElems, r.useSortBodyAlphabetically())
	bodyElems := append(listedBodyElems, unlistedBodyElems...)

	final := make([]reorderElement, 0, len(headElems)+len(bodyElems)+len(footElems))
	final = append(final, headElems...)
	final = append(final, bodyElems...)
	final = append(final, footElems...)

	body.Clear()
	body.AppendNewline()
	emitReorderElements(body, final, len(headElems), len(headElems)+len(bodyElems), r.useHeadFootLineBreaks())
	return nil
}

func (r *ReorderAttributesTransform) useHeadFootLineBreaks() bool {
	if r.HeadFootLineBreaks == nil {
		return true
	}
	return *r.HeadFootLineBreaks
}

func (r *ReorderAttributesTransform) useSortBodyAlphabetically() bool {
	if r.SortBodyAlphabetically == nil {
		return true
	}
	return *r.SortBodyAlphabetically
}

// reorderElement represents one orderable item in a block body — either an
// attribute or a nested block. Source position is captured when known so the
// "preserve source order" body mode can sort attributes and nested blocks by
// the line/column they originally appeared on.
type reorderElement struct {
	name      string
	isNested  bool
	attr      *hclwrite.Attribute
	block     *hclwrite.Block
	hasSource bool
	line      int
	col       int
}

// buildReorderElements collects every attribute and every nested block from
// the write-side body into a unified element list. Source positions are
// resolved from the matching read-side `*hclsyntax.Body` when available.
// Elements added by a previous transform (no source-side counterpart) are
// flagged `hasSource = false` and sorted alphabetically among themselves in
// either body mode.
func buildReorderElements(writeAttrs map[string]*hclwrite.Attribute, writeBlocks []*hclwrite.Block, block *terraform.RootBlock) []reorderElement {
	elements := make([]reorderElement, 0, len(writeAttrs)+len(writeBlocks))

	for name, attr := range writeAttrs {
		el := reorderElement{name: name, attr: attr}
		if syn, ok := block.Body.Attributes[name]; ok && syn != nil {
			el.hasSource = true
			el.line = syn.SrcRange.Start.Line
			el.col = syn.SrcRange.Start.Column
		}
		elements = append(elements, el)
	}

	sourceByName := make(map[string][]*hclsyntax.Block)
	for _, b := range block.Body.Blocks {
		sourceByName[syntaxBlockName(b)] = append(sourceByName[syntaxBlockName(b)], b)
	}
	idxByName := make(map[string]int)
	for _, wb := range writeBlocks {
		name := writeBlockName(wb)
		el := reorderElement{name: name, isNested: true, block: wb}
		idx := idxByName[name]
		idxByName[name] = idx + 1
		if syns, ok := sourceByName[name]; ok && idx < len(syns) {
			el.hasSource = true
			el.line = syns[idx].Range().Start.Line
			el.col = syns[idx].Range().Start.Column
		}
		elements = append(elements, el)
	}

	return elements
}

// partitionReorderElements splits `elements` into head, listed-body,
// unlisted-body, and foot slices according to `head`, `body`, and `foot`
// name lists. Multiple elements that share a name (e.g. two `nested {}`
// blocks of the same type) are grouped together at the head, listed-body, or
// foot position, preserving their write-side order within the group.
// Elements whose names are not mentioned in any of the three lists land in
// `unlistedBodyElems` and are sorted later by `sortReorderBody`.
func partitionReorderElements(elements []reorderElement, head, body, foot []string) (headElems, listedBodyElems, unlistedBodyElems, footElems []reorderElement) {
	headSet := toNameSet(head)
	bodySet := toNameSet(body)
	footSet := toNameSet(foot)
	byName := make(map[string][]reorderElement)
	for _, el := range elements {
		if _, ok := headSet[el.name]; ok {
			byName[el.name] = append(byName[el.name], el)
			continue
		}
		if _, ok := footSet[el.name]; ok {
			byName[el.name] = append(byName[el.name], el)
			continue
		}
		if _, ok := bodySet[el.name]; ok {
			byName[el.name] = append(byName[el.name], el)
			continue
		}
		unlistedBodyElems = append(unlistedBodyElems, el)
	}
	for _, name := range head {
		headElems = append(headElems, byName[name]...)
	}
	for _, name := range body {
		listedBodyElems = append(listedBodyElems, byName[name]...)
	}
	for _, name := range foot {
		footElems = append(footElems, byName[name]...)
	}
	return
}

// validateReorderSectionOverlap returns an error if any name appears in more
// than one of head / body / foot. The error message names both sections so
// the user can fix the duplicate immediately.
func validateReorderSectionOverlap(head, body, foot []string) error {
	headSet := toNameSet(head)
	bodySet := toNameSet(body)
	footSet := toNameSet(foot)
	for name := range headSet {
		if _, ok := bodySet[name]; ok {
			return fmt.Errorf("reorder_attributes: attribute %q cannot be in both head_attributes and body_attributes", name)
		}
		if _, ok := footSet[name]; ok {
			return fmt.Errorf("reorder_attributes: attribute %q cannot be in both head_attributes and foot_attributes", name)
		}
	}
	for name := range bodySet {
		if _, ok := footSet[name]; ok {
			return fmt.Errorf("reorder_attributes: attribute %q cannot be in both body_attributes and foot_attributes", name)
		}
	}
	return nil
}

// validateReorderComposition returns an error when two or more
// reorder_attributes transforms target the same address while at least
// one of them has `sort_body_alphabetically = false`.
//
// The collision is non-composable because `sortReorderBody(false)` orders
// body elements by source-side line/column positions, and those positions
// come from the parse-side `*hclsyntax.Body`. The parse-side body is never
// updated between Apply() calls — only the write-side `*hclwrite.Body` is
// mutated — so the second transform sorts by the ORIGINAL source layout,
// silently undoing the body order produced by the first transform.
//
// When every transform on the address uses the default
// `sort_body_alphabetically = true` they are composable (alphabetical
// sort is idempotent), so the validator only fires when at least one
// transform in the group uses the source-order mode explicitly.
//
// The error message lists every colliding transform with its config
// file:line citation so the user can find and fix all of them in one
// pass; the citations are sorted by address for deterministic output.
func validateReorderComposition(transforms []Transform) error {
	byAddress := map[string][]*ReorderAttributesTransform{}
	for _, t := range transforms {
		r, ok := t.(*ReorderAttributesTransform)
		if !ok {
			continue
		}
		byAddress[r.TargetBlockAddress] = append(byAddress[r.TargetBlockAddress], r)
	}

	addresses := make([]string, 0, len(byAddress))
	for addr := range byAddress {
		addresses = append(addresses, addr)
	}
	sort.Strings(addresses)

	for _, addr := range addresses {
		group := byAddress[addr]
		if len(group) < 2 {
			continue
		}
		anyExplicitFalse := false
		for _, r := range group {
			if !r.useSortBodyAlphabetically() {
				anyExplicitFalse = true
				break
			}
		}
		if !anyExplicitFalse {
			continue
		}

		sort.SliceStable(group, func(i, j int) bool {
			return group[i].Address() < group[j].Address()
		})
		citations := make([]string, 0, len(group))
		for _, r := range group {
			citations = append(citations, fmt.Sprintf("  - %s at %s (sort_body_alphabetically = %t)",
				r.Address(), r.HclBlock().Range().String(), r.useSortBodyAlphabetically()))
		}
		return fmt.Errorf("reorder_attributes: %d transforms target %q with at least one using sort_body_alphabetically = false; this combination is not composable because non-alphabetical sort reads parse-side source positions that are not updated between Apply() calls; set `sort_body_alphabetically = true` on every colliding transform, merge them into a single transform, or filter the `for_each` set of one to exclude addresses handled by the other; colliding transforms:\n%s",
			len(group), addr, strings.Join(citations, "\n"))
	}
	return nil
}

// sortReorderBody stably sorts the body slice in-place.When `alphabetical`
// is true the order is `name` ascending. When false the order is the
// original source position (line then column), with no-source elements
// (added by other transforms) appended afterwards in alphabetical order.
func sortReorderBody(elems []reorderElement, alphabetical bool) {
	if alphabetical {
		sort.SliceStable(elems, func(i, j int) bool {
			return elems[i].name < elems[j].name
		})
		return
	}
	sort.SliceStable(elems, func(i, j int) bool {
		a, b := elems[i], elems[j]
		if a.hasSource != b.hasSource {
			return a.hasSource && !b.hasSource
		}
		if !a.hasSource {
			return a.name < b.name
		}
		if a.line != b.line {
			return a.line < b.line
		}
		if a.col != b.col {
			return a.col < b.col
		}
		return a.name < b.name
	})
}

// emitReorderElements writes `elements` into `body`, inserting blank lines
// between sections and before nested blocks per the documented rules.
//
//   - `headEnd` is the index of the first non-head element.
//   - `footStart` is the index of the first foot element.
//   - `headFootLineBreaks` controls whether blank lines are emitted at the
//     head→body and body→foot section boundaries.
//
// A blank line is emitted before a nested block to match typical Terraform
// formatting, with one exception: when the previous element is a nested
// block sharing the same orderable name (e.g. two `validation {}` siblings,
// two `dynamic "subnet" {}` siblings), no blank line is inserted so the
// group stays visually adjacent. A section boundary still forces a blank
// line, even when it falls between same-kind siblings, because the user
// asked for that separator explicitly.
//
// When the head→body and body→foot boundaries collapse to the same index
// (empty body) only one blank line is emitted, because `needBlank` is a
// single boolean per iteration.
func emitReorderElements(body *hclwrite.Body, elements []reorderElement, headEnd, footStart int, headFootLineBreaks bool) {
	for i, el := range elements {
		if i > 0 {
			needBlank := false
			if headFootLineBreaks && i == headEnd && headEnd > 0 && headEnd < len(elements) {
				needBlank = true
			}
			if headFootLineBreaks && i == footStart && footStart > 0 && footStart < len(elements) {
				needBlank = true
			}
			if el.isNested {
				prev := elements[i-1]
				sameKindAdjacent := prev.isNested && prev.name == el.name
				if !sameKindAdjacent {
					needBlank = true
				}
			}
			if needBlank {
				body.AppendNewline()
			}
		}
		if el.isNested {
			body.AppendBlock(el.block)
		} else {
			body.AppendUnstructuredTokens(el.attr.BuildTokens(nil))
		}
	}
}

// writeBlockName returns the orderable name of a write-side nested block. For
// `dynamic "foo" {}` this is the label (`foo`) so users can address the
// dynamic block under the same name as the static block it generates.
func writeBlockName(b *hclwrite.Block) string {
	if b.Type() == "dynamic" {
		labels := b.Labels()
		if len(labels) > 0 {
			return labels[0]
		}
	}
	return b.Type()
}

// syntaxBlockName is the read-side analogue of writeBlockName, used to match
// write-side blocks back to their source positions when computing the order.
func syntaxBlockName(b *hclsyntax.Block) string {
	if b.Type == "dynamic" && len(b.Labels) > 0 {
		return b.Labels[0]
	}
	return b.Type
}

func toNameSet(s []string) map[string]struct{} {
	m := make(map[string]struct{}, len(s))
	for _, v := range s {
		m[v] = struct{}{}
	}
	return m
}
