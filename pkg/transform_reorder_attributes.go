package pkg

import (
	"fmt"
	"sort"

	"github.com/Azure/golden"
	"github.com/Azure/mapotf/pkg/terraform"
	"github.com/hashicorp/hcl/v2/hclwrite"
)

var _ Transform = &ReorderAttributesTransform{}

// ReorderAttributesTransform re-orders the attributes of one root block.
//
// Semantics:
//   - Attributes named in `head_attributes` come first, in the listed order.
//   - Attributes named in `tail_attributes` come last, in the listed order.
//   - Every other attribute is written between them, preserving its original
//     source-order position (or alphabetical for attributes added by other
//     transforms that have no source position).
//   - Names in `head_attributes` / `tail_attributes` that are not present on
//     the block are silently skipped.
//   - The same name in both `head_attributes` and `tail_attributes` is a
//     configuration error.
//   - Nested blocks are preserved and re-emitted after the attributes.
//
// This transform never adds, removes, or mutates attribute values — only the
// order changes.
type ReorderAttributesTransform struct {
	*golden.BaseBlock
	*BaseTransform
	TargetBlockAddress string   `hcl:"target_block_address"`
	HeadAttributes     []string `hcl:"head_attributes,optional"`
	TailAttributes     []string `hcl:"tail_attributes,optional"`
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
	if len(writeAttrs) == 0 {
		return nil
	}

	writeNestedBlocks := body.Blocks()
	finalOrder := computeReorderedAttributeNames(r.HeadAttributes, r.TailAttributes, writeAttrs, block)
	originalOrder := orderedAttributeNamesFromWriteBody(writeAttrs, block)
	if stringSlicesEqual(finalOrder, originalOrder) {
		return nil
	}

	body.Clear()
	body.AppendNewline()
	for _, name := range finalOrder {
		body.AppendUnstructuredTokens(writeAttrs[name].BuildTokens(nil))
	}
	if len(writeNestedBlocks) > 0 {
		body.AppendNewline()
		for _, nb := range writeNestedBlocks {
			body.AppendBlock(nb)
		}
	}
	return nil
}

// computeReorderedAttributeNames returns the final attribute order:
//
//   1. names from `head` that exist in `writeAttrs`, in the head order
//   2. middle attributes (those in `writeAttrs` but in neither head nor tail),
//      ordered by their source line; attributes that were added by a previous
//      transform (no source position) come after source-positioned attributes,
//      sorted alphabetically among themselves
//   3. names from `tail` that exist in `writeAttrs`, in the tail order
func computeReorderedAttributeNames(head, tail []string, writeAttrs map[string]*hclwrite.Attribute, block *terraform.RootBlock) []string {
	headSet := toNameSet(head)
	tailSet := toNameSet(tail)

	var out []string
	for _, name := range head {
		if _, ok := writeAttrs[name]; ok {
			out = append(out, name)
		}
	}

	type middleEntry struct {
		name      string
		hasSource bool
		line      int
		col       int
	}
	var middle []middleEntry
	for name := range writeAttrs {
		if _, isHead := headSet[name]; isHead {
			continue
		}
		if _, isTail := tailSet[name]; isTail {
			continue
		}
		entry := middleEntry{name: name}
		if syn, ok := block.Body.Attributes[name]; ok && syn != nil {
			entry.hasSource = true
			entry.line = syn.SrcRange.Start.Line
			entry.col = syn.SrcRange.Start.Column
		}
		middle = append(middle, entry)
	}
	sort.SliceStable(middle, func(i, j int) bool {
		a, b := middle[i], middle[j]
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
	for _, e := range middle {
		out = append(out, e.name)
	}

	for _, name := range tail {
		if _, ok := writeAttrs[name]; ok {
			out = append(out, name)
		}
	}
	return out
}

// orderedAttributeNamesFromWriteBody returns the names of all current write-side
// attributes in their source-order (alphabetical tiebreak for attributes without
// a source position). Used purely to detect a no-op reorder.
func orderedAttributeNamesFromWriteBody(writeAttrs map[string]*hclwrite.Attribute, block *terraform.RootBlock) []string {
	type entry struct {
		name      string
		hasSource bool
		line      int
		col       int
	}
	entries := make([]entry, 0, len(writeAttrs))
	for name := range writeAttrs {
		e := entry{name: name}
		if syn, ok := block.Body.Attributes[name]; ok && syn != nil {
			e.hasSource = true
			e.line = syn.SrcRange.Start.Line
			e.col = syn.SrcRange.Start.Column
		}
		entries = append(entries, e)
	}
	sort.SliceStable(entries, func(i, j int) bool {
		a, b := entries[i], entries[j]
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
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.name)
	}
	return names
}

func toNameSet(s []string) map[string]struct{} {
	m := make(map[string]struct{}, len(s))
	for _, v := range s {
		m[v] = struct{}{}
	}
	return m
}

func stringSlicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
