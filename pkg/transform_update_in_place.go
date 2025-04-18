package pkg

import (
	"encoding/json"
	"fmt"
	"github.com/Azure/golden"
	"github.com/Azure/mapotf/pkg/terraform"
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/hashicorp/hcl/v2/hclwrite"
)

var _ Transform = &UpdateInPlaceTransform{}
var _ golden.CustomDecode = &UpdateInPlaceTransform{}

type UpdateInPlaceTransform struct {
	*golden.BaseBlock
	*BaseTransform
	TargetBlockAddress string `hcl:"target_block_address" validate:"required"`
	DynamicBlockBody   string `hcl:"dynamic_block_body,optional"`
	updateBlock        *hclwrite.Block
	targetBlock        *terraform.RootBlock
}

func (u *UpdateInPlaceTransform) Type() string {
	return "update_in_place"
}

func (u *UpdateInPlaceTransform) Apply() error {
	u.PatchWriteBlock(u.targetBlock, u.updateBlock)
	return nil
}

func (u *UpdateInPlaceTransform) Decode(block *golden.HclBlock, context *hcl.EvalContext) error {
	var err error
	u.TargetBlockAddress, err = getRequiredStringAttribute("target_block_address", block, context)
	if err != nil {
		return err
	}
	cfg := u.Config().(*MetaProgrammingTFConfig)
	b := cfg.RootBlock(u.TargetBlockAddress)
	if b == nil {
		return fmt.Errorf("cannot find block: %s", u.TargetBlockAddress)
	}
	u.targetBlock = b
	u.updateBlock = hclwrite.NewBlock("patch", []string{})
	dynamicBlockBody, err := getOptionalStringAttribute("dynamic_block_body", block, context)
	if err != nil {
		return err
	}
	if dynamicBlockBody != nil {
		u.DynamicBlockBody = *dynamicBlockBody
		patch, diag := hclwrite.ParseConfig([]byte(fmt.Sprintf("patch {\n%s\n}", u.DynamicBlockBody)), u.Address(), hcl.InitialPos)
		if diag.HasErrors() {
			return fmt.Errorf("error while parsing patch body: %s", diag.Error())
		}
		if err = decodeAsDynamicBlockBody(u.updateBlock, patch.Body().Blocks()[0]); err != nil {
			return err
		}
	}
	for _, b := range block.NestedBlocks() {
		switch b.Type {
		case "asraw":
			{
				if err = decodeAsRawBlock(u.updateBlock, b); err != nil {
					return err
				}
				continue
			}
		case "asstring":
			{
				if err = decodeAsStringBlock(u.updateBlock, b, 0, context); err != nil {
					return err
				}
				continue
			}
		}
	}
	return nil
}

func decodeAsDynamicBlockBody(dest *hclwrite.Block, patch *hclwrite.Block) error {
	for n, attribute := range patch.Body().Attributes() {
		dest.Body().SetAttributeRaw(n, attribute.Expr().BuildTokens(nil))
	}
	for _, b := range patch.Body().Blocks() {
		blockType := b.Type()
		newNestedBlock := dest.Body().AppendNewBlock(blockType, b.Labels())
		if err := decodeAsDynamicBlockBody(newNestedBlock, b); err != nil {
			return err
		}
	}
	return nil
}

func (u *UpdateInPlaceTransform) UpdateBlock() *hclwrite.Block {
	return u.updateBlock
}

func (u *UpdateInPlaceTransform) PatchWriteBlock(dest terraform.Block, patch *hclwrite.Block) {
	// we cannot patch one-line block
	if dest.Range().Start.Line == dest.Range().End.Line {
		dest.WriteBody().AppendNewline()
	}
	for name, attr := range patch.Body().Attributes() {
		dest.SetAttributeRaw(name, attr.Expr().BuildTokens(nil))
	}
	// Handle nested blocks
	for _, patchNestedBlock := range patch.Body().Blocks() {
		destNestedBlocks := dest.GetNestedBlocks()[patchNestedBlock.Type()]
		if len(destNestedBlocks) == 0 {
			// If the nested block does not exist in dest, add it
			dest.AppendBlock(patchNestedBlock)
		} else {
			for _, nb := range destNestedBlocks {
				u.PatchWriteBlock(nb, patchNestedBlock)
			}
		}
	}
}

func (u *UpdateInPlaceTransform) String() string {
	content := make(map[string]any)
	content["id"] = u.Id()
	content["target_block_address"] = u.TargetBlockAddress
	content["patch"] = string(u.updateBlock.BuildTokens(nil).Bytes())
	str, err := json.Marshal(content)
	if err != nil {
		panic(err.Error())
	}
	return string(str)
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
