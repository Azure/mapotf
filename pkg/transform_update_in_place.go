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

var _ Transform = &UpdateInPlaceTransform{}

type UpdateInPlaceTransform struct {
	*golden.BaseBlock
	*BaseTransform
	TargetBlockAddress string `hcl:"target_block_address"`
	updateBlock        *hclwrite.Block
	targetBlock        *hclwrite.Block
}

func (u *UpdateInPlaceTransform) Type() string {
	return "update_in_place"
}

func (u *UpdateInPlaceTransform) Apply() error {
	//TODO implement me
	panic("implement me")
}

func (u *UpdateInPlaceTransform) Decode(block *golden.HclBlock, context *hcl.EvalContext) error {
	targetBlockAddress, ok := block.Attributes()["target_block_address"]
	if !ok {
		return fmt.Errorf("`target_block_address` is required")
	}
	v, err := targetBlockAddress.Value(context)
	if err != nil {
		return err
	}
	if v.Type() != cty.String {
		return fmt.Errorf("`target_block_address` must be a string, like `resource.azurerm_resource_group.this`")
	}
	address := v.AsString()
	cfg := u.BaseBlock.Config().(*MetaProgrammingTFConfig)
	b := cfg.TerraformBlock(address)
	if b == nil {
		return fmt.Errorf("cannot find block: %s", address)
	}
	u.targetBlock = b.WriteBlock
	u.updateBlock = hclwrite.NewBlock("patch", []string{})
	for _, b := range block.NestedBlocks() {
		if b.Type == "asraw" {
			if err := u.decodeAsRawBlock(u.updateBlock, b); err != nil {
				return err
			}
		}
	}
	return u.decodeAsStringBlock(u.updateBlock, block, 0, context)
}

func (u *UpdateInPlaceTransform) UpdateBlock() *hclwrite.Block {
	return u.updateBlock
}

func (u *UpdateInPlaceTransform) decodeAsStringBlock(dest *hclwrite.Block, src *golden.HclBlock, depth int, context *hcl.EvalContext) error {
	for n, attribute := range src.Attributes() {
		if n == "target_block_address" && depth == 0 {
			continue
		}
		value, err := attribute.Value(context)
		if err != nil {
			return err
		}
		if value.Type() != cty.String {
			value, err = convert.Convert(value, cty.String)
			if err != nil {
				return fmt.Errorf("cannot convert value to string, got %s", value.Type().FriendlyName())
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
		if depth == 0 && blockType == "asraw" {
			continue
		}
		newNestedBlock := dest.Body().AppendNewBlock(blockType, b.Labels)
		if err := u.decodeAsStringBlock(newNestedBlock, b, depth+1, context); err != nil {
			return err
		}
	}
	return nil
}

func (u *UpdateInPlaceTransform) decodeAsRawBlock(dest *hclwrite.Block, src *golden.HclBlock) error {
	for n, attribute := range src.Attributes() {
		dest.Body().SetAttributeRaw(n, attribute.ExprTokens())
	}
	for _, b := range src.NestedBlocks() {
		blockType := b.Type
		newNestedBlock := dest.Body().AppendNewBlock(blockType, b.Labels)
		if err := u.decodeAsRawBlock(newNestedBlock, b); err != nil {
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

// Copy from https://github.com/hashicorp/hcl/blob/v2.20.1/hclwrite/parser.go#L478-L517
func writerTokens(nativeTokens hclsyntax.Tokens) hclwrite.Tokens {
	tokBuf := make([]hclwrite.Token, len(nativeTokens))
	var lastByteOffset int
	for i, mainToken := range nativeTokens {
		bytes := make([]byte, len(mainToken.Bytes))
		copy(bytes, mainToken.Bytes)

		tokBuf[i] = hclwrite.Token{
			Type:  mainToken.Type,
			Bytes: bytes,

			SpacesBefore: mainToken.Range.Start.Byte - lastByteOffset,
		}

		lastByteOffset = mainToken.Range.End.Byte
	}

	ret := make(hclwrite.Tokens, len(tokBuf))
	for i := range ret {
		ret[i] = &tokBuf[i]
	}

	return ret
}
