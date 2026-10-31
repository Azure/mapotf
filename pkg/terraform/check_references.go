package terraform

import (
	"fmt"
	"sort"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/zclconf/go-cty/cty"
)

var referenceStepType = cty.Object(map[string]cty.Type{
	"kind":       cty.String,
	"value":      cty.String,
	"value_type": cty.String,
})

var referenceType = cty.Object(map[string]cty.Type{
	"root":      cty.String,
	"traversal": cty.String,
	"steps":     cty.List(referenceStepType),
	"range": cty.Object(map[string]cty.Type{
		"file_name":    cty.String,
		"start_line":   cty.Number,
		"start_column": cty.Number,
		"end_line":     cty.Number,
		"end_column":   cty.Number,
	}),
})

func (b *RootBlock) CheckReferences() (cty.Value, error) {
	var traversals []hcl.Traversal
	var visit func(*hclsyntax.Body)
	visit = func(body *hclsyntax.Body) {
		for _, attribute := range body.Attributes {
			traversals = append(traversals, attribute.Expr.Variables()...)
		}
		for _, block := range body.Blocks {
			visit(block.Body)
		}
	}
	visit(b.Block.Body)
	sort.SliceStable(traversals, func(i, j int) bool {
		return traversals[i].SourceRange().Start.Byte < traversals[j].SourceRange().Start.Byte
	})

	references := make([]cty.Value, 0, len(traversals))
	for _, traversal := range traversals {
		r := traversal.SourceRange()
		if r.Start.Byte < 0 || r.End.Byte < r.Start.Byte || r.End.Byte > len(b.source) {
			return cty.NilVal, fmt.Errorf("cannot reflect reference at %s: source range is unavailable", r.String())
		}
		steps := make([]cty.Value, 0, len(traversal)-1)
		for _, step := range traversal[1:] {
			switch s := step.(type) {
			case hcl.TraverseAttr:
				steps = append(steps, referenceStep("attribute", s.Name, "string"))
			case hcl.TraverseIndex:
				value := string(hclwrite.TokensForValue(s.Key).Bytes())
				valueType := s.Key.Type().FriendlyName()
				if s.Key.Type() == cty.String {
					value = s.Key.AsString()
				}
				steps = append(steps, referenceStep("index", value, valueType))
			default:
				return cty.NilVal, fmt.Errorf("cannot reflect reference at %s: unsupported traversal step %T", r.String(), step)
			}
		}
		stepList := cty.ListValEmpty(referenceStepType)
		if len(steps) != 0 {
			stepList = cty.ListVal(steps)
		}
		references = append(references, cty.ObjectVal(map[string]cty.Value{
			"root":      cty.StringVal(traversal.RootName()),
			"traversal": cty.StringVal(b.source[r.Start.Byte:r.End.Byte]),
			"steps":     stepList,
			"range": cty.ObjectVal(map[string]cty.Value{
				"file_name":    cty.StringVal(r.Filename),
				"start_line":   cty.NumberIntVal(int64(r.Start.Line)),
				"start_column": cty.NumberIntVal(int64(r.Start.Column)),
				"end_line":     cty.NumberIntVal(int64(r.End.Line)),
				"end_column":   cty.NumberIntVal(int64(r.End.Column)),
			}),
		}))
	}
	if len(references) == 0 {
		return cty.ListValEmpty(referenceType), nil
	}
	return cty.ListVal(references), nil
}

func referenceStep(kind, value, valueType string) cty.Value {
	return cty.ObjectVal(map[string]cty.Value{
		"kind":       cty.StringVal(kind),
		"value":      cty.StringVal(value),
		"value_type": cty.StringVal(valueType),
	})
}
