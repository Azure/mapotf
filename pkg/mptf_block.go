package pkg

import (
	"fmt"
	"github.com/Azure/golden"
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/zclconf/go-cty/cty"
	"github.com/zclconf/go-cty/cty/convert"
)

type mptfBlock interface {
	isReservedField(name string) bool
}

func decodeAsStringBlock(u mptfBlock, dest *hclwrite.Block, src *golden.HclBlock, depth int, context *hcl.EvalContext) error {
	for n, attribute := range src.Attributes() {
		if u.isReservedField(n) && depth == 0 {
			continue
		}
		value, err := attribute.Value(context)
		if err != nil {
			return err
		}
		valueType := value.Type()
		if valueType != cty.String {
			value, err = convert.Convert(value, cty.String)
			if err != nil {
				return fmt.Errorf("cannot convert value to string, got: %s", valueType.FriendlyName())
			}
		}
		tokens, err := stringToHclWriteTokens(value.AsString())
		if err != nil {
			return err
		}
		dest.Body().SetAttributeRaw(n, tokens)
	}
	for _, b := range src.NestedBlocks() {
		blockType := b.Type
		if u.isReservedField(blockType) && depth == 0 {
			continue
		}
		newNestedBlock := dest.Body().AppendNewBlock(blockType, b.Labels)
		if err := decodeAsStringBlock(u, newNestedBlock, b, depth+1, context); err != nil {
			return err
		}
	}
	return nil
}

func decodeAsRawBlock(dest *hclwrite.Block, src *golden.HclBlock) error {
	for n, attribute := range src.Attributes() {
		dest.Body().SetAttributeRaw(n, attribute.ExprTokens())
	}
	for _, b := range src.NestedBlocks() {
		blockType := b.Type
		newNestedBlock := dest.Body().AppendNewBlock(blockType, b.Labels)
		if err := decodeAsRawBlock(newNestedBlock, b); err != nil {
			return err
		}
	}
	return nil
}

func stringToHclWriteTokens(exp string) (hclwrite.Tokens, error) {
	tokens, diag := hclsyntax.LexExpression([]byte(exp), "", hcl.InitialPos)
	if diag.HasErrors() {
		return nil, diag
	}
	return writerTokens(tokens), nil
}
