package pkg

import (
	"fmt"
	"sort"

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
//   - Elements named in `tail_attributes` come last, in the listed order.
//   - Every other element is the "middle". By default the middle is sorted
//     alphabetically by name; set `sort_middle_alphabetically = false` to
//     preserve the original source order instead.
//   - When `head_tail_line_breaks` is true (default) a blank line is inserted
//     between the head section and the middle, and between the middle and the
//     tail section. Set to `false` to suppress these blank lines.
//   - Every nested block in the output is preceded by a blank line — if a
//     blank line is already there (because of the head/tail rule above) no
//     extra blank line is inserted.
//   - Names in `head_attributes` / `tail_attributes` that are not present on
//     the block are silently skipped.
//   - The same name in both `head_attributes` and `tail_attributes` is a
//     configuration error.
//
// This transform never adds, removes, or mutates attribute values — only the
// layout changes.
type ReorderAttributesTransform struct {
	*golden.BaseBlock
	*BaseTransform
	TargetBlockAddress       string   `hcl:"target_block_address"`
	HeadAttributes           []string `hcl:"head_attributes,optional"`
	TailAttributes           []string `hcl:"tail_attributes,optional"`
	HeadTailLineBreaks       *bool    `hcl:"head_tail_line_breaks,optional"`
	SortMiddleAlphabetically *bool    `hcl:"sort_middle_alphabetically,optional"`
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

	headSet := toNameSet(r.HeadAttributes)
	tailSet := toNameSet(r.TailAttributes)
	for name := range headSet {
		if _, conflict := tailSet[name]; conflict {
			return fmt.Errorf("reorder_attributes: attribute %q cannot be in both head_attributes and tail_attributes", name)
		}
	}

	body := block.WriteBlock.Body()
	writeAttrs := body.Attributes()
	writeBlocks := body.Blocks()
	if len(writeAttrs) == 0 && len(writeBlocks) == 0 {
		return nil
	}

	elements := buildReorderElements(writeAttrs, writeBlocks, block)

	headElems, midElems, tailElems := partitionReorderElements(elements, r.HeadAttributes, r.TailAttributes)
	sortReorderMiddle(midElems, r.useSortMiddleAlphabetically())

	final := make([]reorderElement, 0, len(headElems)+len(midElems)+len(tailElems))
	final = append(final, headElems...)
	final = append(final, midElems...)
	final = append(final, tailElems...)

	body.Clear()
	body.AppendNewline()
	emitReorderElements(body, final, len(headElems), len(headElems)+len(midElems), r.useHeadTailLineBreaks())
	return nil
}

func (r *ReorderAttributesTransform) useHeadTailLineBreaks() bool {
	if r.HeadTailLineBreaks == nil {
		return true
	}
	return *r.HeadTailLineBreaks
}

func (r *ReorderAttributesTransform) useSortMiddleAlphabetically() bool {
	if r.SortMiddleAlphabetically == nil {
		return true
	}
	return *r.SortMiddleAlphabetically
}

// reorderElement represents one orderable item in a block body — either an
// attribute or a nested block. Source position is captured when known so the
// "preserve source order" middle mode can sort attributes and nested blocks by
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
// either middle mode.
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

// partitionReorderElements splits `elements` into head, middle, and tail
// slices according to `head` and `tail` name lists. Multiple elements that
// share a name (e.g. two `nested {}` blocks of the same type) are grouped
// together at the head or tail position, preserving their write-side order
// within the group.
func partitionReorderElements(elements []reorderElement, head, tail []string) (headElems, midElems, tailElems []reorderElement) {
	headSet := toNameSet(head)
	tailSet := toNameSet(tail)
	byName := make(map[string][]reorderElement)
	for _, el := range elements {
		if _, ok := headSet[el.name]; ok {
			byName[el.name] = append(byName[el.name], el)
			continue
		}
		if _, ok := tailSet[el.name]; ok {
			byName[el.name] = append(byName[el.name], el)
			continue
		}
		midElems = append(midElems, el)
	}
	for _, name := range head {
		headElems = append(headElems, byName[name]...)
	}
	for _, name := range tail {
		tailElems = append(tailElems, byName[name]...)
	}
	return
}

// sortReorderMiddle stably sorts the middle slice in-place. When
// `alphabetical` is true the order is `name` ascending. When false the order
// is the original source position (line then column), with no-source elements
// (added by other transforms) appended afterwards in alphabetical order.
func sortReorderMiddle(elems []reorderElement, alphabetical bool) {
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
//   - `tailStart` is the index of the first tail element.
//   - `headTailLineBreaks` controls whether blank lines are emitted at the
//     head→middle and middle→tail section boundaries.
//
// A blank line is always emitted before a nested block (so user-facing
// readability matches typical Terraform style). When the head→middle and
// middle→tail boundaries collapse to the same index (empty middle) only one
// blank line is emitted, because `needBlank` is a single boolean per
// iteration.
func emitReorderElements(body *hclwrite.Body, elements []reorderElement, headEnd, tailStart int, headTailLineBreaks bool) {
	for i, el := range elements {
		if i > 0 {
			needBlank := false
			if headTailLineBreaks && i == headEnd && headEnd > 0 && headEnd < len(elements) {
				needBlank = true
			}
			if headTailLineBreaks && i == tailStart && tailStart > 0 && tailStart < len(elements) {
				needBlank = true
			}
			if el.isNested {
				needBlank = true
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
