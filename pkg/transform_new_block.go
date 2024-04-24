package pkg

import (
	"fmt"
	"github.com/Azure/golden"
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/hashicorp/hcl/v2/hclwrite"
	avmfix "github.com/lonegunmanb/avmfix/pkg"
	"github.com/zclconf/go-cty/cty"
)

var _ golden.ApplyBlock = &NewBlockTransform{}
var _ golden.CustomDecode = &NewBlockTransform{}
var _ mptfBlock = &NewBlockTransform{}

type NewBlockTransform struct {
	*golden.BaseBlock
	*BaseTransform
	NewBlockType  string   `hcl:"new_block_type"`
	FileName      string   `hcl:"filename" validate:"endswith=.tf"`
	Labels        []string `hcl:"labels,optional"`
	newWriteBlock *hclwrite.Block
}

func (n *NewBlockTransform) isReservedField(name string) bool {
	reserved := map[string]struct{}{
		"new_block_type": {},
		"for_each":       {},
		"asraw":          {},
		"asstring":       {},
		"labels":         {},
		"filename":       {},
	}
	_, ok := reserved[name]
	return ok
}

func (n *NewBlockTransform) Decode(block *golden.HclBlock, context *hcl.EvalContext) error {
	var err error
	n.NewBlockType, err = getRequiredStringAttribute("new_block_type", block, context)
	if err != nil {
		return err
	}
	n.FileName, err = getRequiredStringAttribute("filename", block, context)
	if err != nil {
		return err
	}
	var labels []string
	labelsAttr, ok := block.Attributes()["labels"]
	if ok {
		labelsValue, err := labelsAttr.Value(context)
		if err != nil {
			return fmt.Errorf("error while evaluating labels: %+v", err)
		}
		for i := 0; i < labelsValue.LengthInt(); i++ {
			labels = append(labels, labelsValue.Index(cty.NumberIntVal(int64(i))).AsString())
		}
	}
	n.Labels = labels
	n.newWriteBlock = hclwrite.NewBlock(n.NewBlockType, n.Labels)
	for _, b := range block.NestedBlocks() {
		if b.Type == "asraw" {
			if err := decodeAsRawBlock(n.newWriteBlock, b); err != nil {
				return err
			}
			continue
		}
		if b.Type == "asstring" {
			if err := decodeAsStringBlock(n, n.newWriteBlock, b, 0, context); err != nil {
				return err
			}
			continue
		}
	}
	n.newWriteBlock, err = n.Format(n.newWriteBlock)
	return err
}

func (n *NewBlockTransform) Type() string {
	return "new_block"
}

func (n *NewBlockTransform) Apply() error {
	n.Config().(*MetaProgrammingTFConfig).AddBlock(n.FileName, n.newWriteBlock)
	return nil
}

func (n *NewBlockTransform) NewWriteBlock() *hclwrite.Block {
	return n.newWriteBlock
}

func (n *NewBlockTransform) Format(block *hclwrite.Block) (*hclwrite.Block, error) {
	if block.Type() != "resource" && block.Type() != "data" && block.Type() != "variable" {
		return block, nil
	}
	bytes := block.BuildTokens(nil).Bytes()
	syntaxFile, diag := hclsyntax.ParseConfig(bytes, "dummy.hcl", hcl.InitialPos)
	if diag.HasErrors() {
		return nil, diag
	}
	syntaxBlock := syntaxFile.Body.(*hclsyntax.Body).Blocks[0]
	avmBlock := avmfix.NewHclBlock(syntaxBlock, block)
	if block.Type() == "resource" || block.Type() == "data" {
		resourceBlock := avmfix.BuildResourceBlock(avmBlock, &hcl.File{})
		resourceBlock.AutoFix()
		return resourceBlock.HclBlock.WriteBlock, nil
	}
	if block.Type() == "variable" {
		variableBlock := avmfix.BuildVariableBlock(&hcl.File{}, avmBlock)
		variableBlock.AutoFix()
		return variableBlock.Block.WriteBlock, nil
	}
	return nil, nil
}

func getRequiredStringAttribute(name string, block *golden.HclBlock, context *hcl.EvalContext) (string, error) {
	targetBlockAddress, ok := block.Attributes()[name]
	if !ok {
		return "", fmt.Errorf("`%s` is required", name)
	}
	v, err := targetBlockAddress.Value(context)
	if err != nil {
		return "", err
	}
	if v.Type() != cty.String {
		return "", fmt.Errorf("`%s` must be a string", name)
	}
	return v.AsString(), nil
}
