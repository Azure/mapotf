package pkg

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/Azure/golden"
	"github.com/Azure/mapotf/pkg/terraform"
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/zclconf/go-cty/cty"
)

var _ Transform = &UpdateInPlaceTransform{}
var _ golden.CustomDecode = &UpdateInPlaceTransform{}

type UpdateInPlaceTransform struct {
	*golden.BaseBlock
	*BaseTransform
	TargetBlockAddress    string `hcl:"target_block_address" validate:"required"`
	DynamicBlockBody      string `hcl:"dynamic_block_body,optional"`
	MergeObjectAttributes bool   `hcl:"merge_object_attributes,optional"`
	updateBlock           *hclwrite.Block
	targetBlock           *terraform.RootBlock
}

func (u *UpdateInPlaceTransform) Type() string {
	return "update_in_place"
}

func (u *UpdateInPlaceTransform) Apply() error {
	if err := u.PatchWriteBlock(u.targetBlock, u.updateBlock); err != nil {
		return fmt.Errorf("%s: %w", u.Address(), err)
	}
	return nil
}

func (u *UpdateInPlaceTransform) Decode(block *golden.HclBlock, context *hcl.EvalContext) error {
	var err error
	u.TargetBlockAddress, err = getRequiredStringAttribute("target_block_address", block, context)
	if err != nil {
		return err
	}
	u.MergeObjectAttributes = false
	if attr, ok := block.Attributes()["merge_object_attributes"]; ok {
		value, err := attr.Value(context)
		if err != nil {
			return fmt.Errorf("`merge_object_attributes`: %w", err)
		}
		if value.Type() != cty.Bool || value.IsNull() || !value.IsKnown() {
			return fmt.Errorf("`merge_object_attributes` must be a known, non-null bool")
		}
		u.MergeObjectAttributes = value.True()
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

func (u *UpdateInPlaceTransform) PatchWriteBlock(dest terraform.Block, patch *hclwrite.Block) error {
	if u.MergeObjectAttributes {
		if err := validateObjectMergePatch(patch); err != nil {
			return err
		}
		// Validate the complete patch on a copy before changing any target tokens.
		body := dest.WriteBody().BuildTokens(nil).Bytes()
		source := []byte("patch {\n" + string(body) + "\n}")
		if dest.Range().Start.Line == dest.Range().End.Line && !bytes.Contains(body, []byte("\n")) {
			source = []byte("patch {" + string(body) + "}")
		}
		read, diag := hclsyntax.ParseConfig(source, dest.Range().Filename, hcl.InitialPos)
		readBlock, err := singleMergeSyntaxBlock(read, diag)
		if err != nil {
			return fmt.Errorf("cannot parse object merge target: %w", err)
		}
		write, diag := hclwrite.ParseConfig(source, dest.Range().Filename, hcl.InitialPos)
		writeBlock, err := singleMergeWriteBlock(write, diag)
		if err != nil {
			return fmt.Errorf("cannot parse object merge target: %w", err)
		}
		copy := terraform.NewBlock(nil, readBlock, writeBlock)
		if err := u.patchWriteBlock(copy, patch); err != nil {
			return err
		}
		if _, diag := hclsyntax.ParseConfig(copy.WriteBlock.BuildTokens(nil).Bytes(), dest.Range().Filename, hcl.InitialPos); diag.HasErrors() {
			return fmt.Errorf("cannot safely apply object merge patch: %s", diag.Error())
		}
	}
	return u.patchWriteBlock(dest, patch)
}

func (u *UpdateInPlaceTransform) patchWriteBlock(dest terraform.Block, patch *hclwrite.Block) error {
	// we cannot patch one-line block
	singleLine := dest.Range().Start.Line == dest.Range().End.Line
	if u.MergeObjectAttributes {
		singleLine = len(dest.WriteBody().Attributes()) == 0 && !bytes.Contains(dest.WriteBody().BuildTokens(nil).Bytes(), []byte("\n"))
	}
	if singleLine {
		dest.WriteBody().AppendNewline()
	}
	for name, attr := range patch.Body().Attributes() {
		tokens := attr.Expr().BuildTokens(nil)
		if u.MergeObjectAttributes {
			var current hclwrite.Tokens
			if existing := dest.WriteBody().GetAttribute(name); existing != nil {
				current = existing.Expr().BuildTokens(nil)
			}
			var err error
			tokens, err = mergeObjectAttribute(current, tokens)
			if err != nil {
				return fmt.Errorf("cannot merge attribute %q at %s: %w", name, dest.Range(), err)
			}
		}
		dest.SetAttributeRaw(name, tokens)
	}
	// Handle nested blocks
	for _, patchNestedBlock := range patch.Body().Blocks() {
		destNestedBlocks := dest.GetNestedBlocks()[patchNestedBlock.Type()]
		if u.MergeObjectAttributes {
			var err error
			destNestedBlocks, err = currentWriteNestedBlocks(dest, patchNestedBlock.Type())
			if err != nil {
				return err
			}
		}
		if len(destNestedBlocks) == 0 {
			// If the nested block does not exist in dest, add it
			newBlock := patchNestedBlock
			if u.MergeObjectAttributes {
				file, diag := hclwrite.ParseConfig(newBlock.BuildTokens(nil).Bytes(), dest.Range().Filename, hcl.InitialPos)
				parsedBlock, err := singleMergeWriteBlock(file, diag)
				if err != nil {
					return fmt.Errorf("cannot parse new nested block: %w", err)
				}
				newBlock = parsedBlock
			}
			dest.AppendBlock(newBlock)
		} else {
			for _, nb := range destNestedBlocks {
				if err := u.patchWriteBlock(nb, patchNestedBlock); err != nil {
					return fmt.Errorf("nested block %q: %w", patchNestedBlock.Type(), err)
				}
			}
		}
	}
	return nil
}

func currentWriteNestedBlocks(dest terraform.Block, blockType string) ([]*terraform.NestedBlock, error) {
	var blocks []*terraform.NestedBlock
	for _, block := range dest.WriteBody().Blocks() {
		effectiveType := block.Type()
		if effectiveType == "dynamic" && len(block.Labels()) == 1 {
			effectiveType = block.Labels()[0]
		}
		if effectiveType != blockType {
			continue
		}
		read, diag := hclsyntax.ParseConfig(block.BuildTokens(nil).Bytes(), dest.Range().Filename, hcl.InitialPos)
		readBlock, err := singleMergeSyntaxBlock(read, diag)
		if err != nil {
			return nil, fmt.Errorf("cannot parse current nested block %q: %w", blockType, err)
		}
		blocks = append(blocks, terraform.NewNestedBlock(readBlock, block))
	}
	return blocks, nil
}

func singleMergeSyntaxBlock(file *hcl.File, diag hcl.Diagnostics) (*hclsyntax.Block, error) {
	if diag.HasErrors() {
		return nil, diag
	}
	if file == nil {
		return nil, fmt.Errorf("expected a non-nil HCL syntax file")
	}
	body, ok := file.Body.(*hclsyntax.Body)
	if !ok || body == nil {
		return nil, fmt.Errorf("expected a non-nil HCL syntax body")
	}
	if len(body.Blocks) != 1 {
		return nil, fmt.Errorf("expected exactly one HCL syntax block, got %d", len(body.Blocks))
	}
	block := body.Blocks[0]
	if block == nil || block.Body == nil {
		return nil, fmt.Errorf("expected a non-nil HCL syntax block and body")
	}
	return block, nil
}

func singleMergeWriteBlock(file *hclwrite.File, diag hcl.Diagnostics) (*hclwrite.Block, error) {
	if diag.HasErrors() {
		return nil, diag
	}
	if file == nil {
		return nil, fmt.Errorf("expected a non-nil HCL write file")
	}
	body := file.Body()
	if body == nil {
		return nil, fmt.Errorf("expected a non-nil HCL write body")
	}
	blocks := body.Blocks()
	if len(blocks) != 1 {
		return nil, fmt.Errorf("expected exactly one HCL write block, got %d", len(blocks))
	}
	block := blocks[0]
	if block == nil || block.Body() == nil {
		return nil, fmt.Errorf("expected a non-nil HCL write block and body")
	}
	return block, nil
}

func (u *UpdateInPlaceTransform) String() string {
	content := make(map[string]any)
	content["id"] = u.Id()
	content["target_block_address"] = u.TargetBlockAddress
	if u.MergeObjectAttributes {
		content["merge_object_attributes"] = true
	}
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
