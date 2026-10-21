package pkg

import (
	"fmt"
	"slices"

	"github.com/Azure/golden"
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/zclconf/go-cty/cty"
)

var _ Transform = &RemoveObjectAttributesTransform{}
var _ golden.CustomDecode = &RemoveObjectAttributesTransform{}

type RemoveObjectAttributesTransform struct {
	*golden.BaseBlock
	*BaseTransform
	TargetBlockAddress   string   `hcl:"target_block_address"`
	AttributePath        []string `hcl:"attribute_path"`
	Keys                 []string `hcl:"keys"`
	RemoveEmptyAttribute bool     `hcl:"remove_empty_attribute,optional"`
}

func (r *RemoveObjectAttributesTransform) Type() string {
	return "remove_object_attributes"
}

func (r *RemoveObjectAttributesTransform) Decode(block *golden.HclBlock, context *hcl.EvalContext) error {
	target, ok := block.Attributes()["target_block_address"]
	if !ok {
		return fmt.Errorf("`target_block_address` is required")
	}
	value, err := target.Value(context)
	if err != nil {
		return err
	}
	if value.IsNull() || !value.IsKnown() || value.Type() != cty.String || value.AsString() == "" {
		return fmt.Errorf("`target_block_address` must be a known, non-null, nonempty string")
	}
	r.TargetBlockAddress = value.AsString()
	for _, field := range []struct {
		name  string
		value *[]string
	}{
		{"attribute_path", &r.AttributePath},
		{"keys", &r.Keys},
	} {
		attr, ok := block.Attributes()[field.name]
		if !ok {
			return fmt.Errorf("`%s` is required", field.name)
		}
		value, err := attr.Value(context)
		if err != nil {
			return fmt.Errorf("`%s`: %w", field.name, err)
		}
		if value.IsNull() || !value.IsWhollyKnown() || (!value.Type().IsTupleType() && !value.Type().IsListType()) {
			return fmt.Errorf("`%s` must be a known, non-null list of strings", field.name)
		}
		*field.value = nil
		for it := value.ElementIterator(); it.Next(); {
			_, element := it.Element()
			if element.IsNull() || element.Type() != cty.String {
				return fmt.Errorf("`%s` must contain only non-null strings", field.name)
			}
			*field.value = append(*field.value, element.AsString())
		}
	}
	if len(r.AttributePath) == 0 {
		return fmt.Errorf("`attribute_path` must contain at least one segment")
	}
	r.RemoveEmptyAttribute = false
	if attr, ok := block.Attributes()["remove_empty_attribute"]; ok {
		value, err := attr.Value(context)
		if err != nil {
			return fmt.Errorf("`remove_empty_attribute`: %w", err)
		}
		if value.IsNull() || !value.IsKnown() || value.Type() != cty.Bool {
			return fmt.Errorf("`remove_empty_attribute` must be a known, non-null bool")
		}
		r.RemoveEmptyAttribute = value.True()
	}
	if r.RemoveEmptyAttribute && len(r.AttributePath) != 1 {
		return fmt.Errorf("`remove_empty_attribute` requires a top-level attribute_path")
	}
	return nil
}

func (r *RemoveObjectAttributesTransform) Apply() error {
	cfg := r.Config().(*MetaProgrammingTFConfig)
	block := cfg.RootBlock(r.TargetBlockAddress)
	if block == nil {
		return fmt.Errorf("%s: cannot find block: %s", r.Address(), r.TargetBlockAddress)
	}
	name := r.AttributePath[0]
	attr := block.WriteBody().GetAttribute(name)
	if attr == nil {
		return nil
	}
	tokens, empty, err := removeObjectAttributes(attr.Expr().BuildTokens(nil), r.AttributePath[1:], r.Keys)
	if err != nil {
		return fmt.Errorf("%s: attribute_path %q: %w", r.Address(), r.AttributePath, err)
	}
	if empty && r.RemoveEmptyAttribute {
		block.RemoveContent(name)
	} else {
		block.SetAttributeRaw(name, tokens)
	}
	return nil
}

func removeObjectAttributes(tokens hclwrite.Tokens, path, keys []string) (hclwrite.Tokens, bool, error) {
	source := tokens.Bytes()
	object, err := parseLiteralObject(source)
	if err != nil {
		return nil, false, err
	}
	for {
		items, err := staticObjectItems(object)
		if err != nil {
			return nil, false, err
		}
		if len(path) == 0 {
			break
		}
		item, exists := items[path[0]]
		if !exists {
			return tokens, false, nil
		}
		object, _ = unparenthesizedExpression(item.ValueExpr).(*hclsyntax.ObjectConsExpr)
		if object == nil {
			return nil, false, fmt.Errorf("path segment %q must contain a literal object", path[0])
		}
		path = path[1:]
	}
	nativeTokens, diag := hclsyntax.LexExpression(source, "object-expression", hcl.InitialPos)
	if diag.HasErrors() {
		return nil, false, fmt.Errorf("cannot tokenize object expression: %s", diag.Error())
	}
	type removal struct{ start, end int }
	var removals []removal
	remaining := len(object.Items)
	for i, item := range object.Items {
		key, _ := staticObjectKey(item.KeyExpr)
		if !slices.Contains(keys, key) {
			continue
		}
		remaining--
		removals = append(removals, removal{item.KeyExpr.Range().Start.Byte, item.ValueExpr.Range().End.Byte})
		gapEnd := object.Range().End.Byte - 1
		if i+1 < len(object.Items) {
			gapEnd = object.Items[i+1].KeyExpr.Range().Start.Byte
		}
		// Keep all inter-entry comments; only the removed entry's separator goes.
		for _, token := range nativeTokens {
			if token.Type == hclsyntax.TokenComma && token.Range.Start.Byte >= item.ValueExpr.Range().End.Byte && token.Range.End.Byte <= gapEnd {
				removals = append(removals, removal{token.Range.Start.Byte, token.Range.End.Byte})
			}
		}
	}
	if len(removals) == 0 {
		return tokens, remaining == 0, nil
	}
	var result []byte
	cursor := 0
	for _, span := range removals {
		result = append(result, source[cursor:span.start]...)
		cursor = span.end
	}
	result = append(result, source[cursor:]...)
	if _, diag := hclsyntax.ParseExpression(result, "trimmed-object", hcl.InitialPos); diag.HasErrors() {
		return nil, false, fmt.Errorf("cannot safely remove object entries: %s", diag.Error())
	}
	updated, diag := hclsyntax.LexExpression(result, "trimmed-object", hcl.InitialPos)
	if diag.HasErrors() {
		return nil, false, fmt.Errorf("cannot tokenize trimmed object: %s", diag.Error())
	}
	return writerTokens(updated[:len(updated)-1]), remaining == 0, nil
}
