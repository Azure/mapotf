package pkg

import (
	"fmt"
	"strings"

	"github.com/Azure/golden"
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/hashicorp/hcl/v2/hclwrite"
	avmfix "github.com/lonegunmanb/avmfix/pkg"
	"github.com/zclconf/go-cty/cty"
)

var _ golden.ApplyBlock = &NewBlockTransform{}
var _ golden.CustomDecode = &NewBlockTransform{}

type NewBlockTransform struct {
	*golden.BaseBlock
	*BaseTransform
	NewBlockType  string   `hcl:"new_block_type"`
	FileName      string   `hcl:"filename" validate:"endswith=.tf"`
	Labels        []string `hcl:"labels,optional"`
	NewBody       string   `hcl:"body,optional"`
	newWriteBlock *hclwrite.Block
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
	bodyStr, err := getOptionalStringAttribute("body", block, context)
	if err != nil {
		return err
	}
	if bodyStr != nil {
		n.NewBody = *bodyStr
	}
	n.newWriteBlock = hclwrite.NewBlock(n.NewBlockType, n.Labels)
	decodeByNestedBlock := false
	for _, b := range block.NestedBlocks() {
		if b.Type == "asraw" {
			decodeByNestedBlock = true
			if err := decodeAsRawBlock(n.newWriteBlock, b); err != nil {
				return err
			}
			continue
		}
		if b.Type == "asstring" {
			decodeByNestedBlock = true
			if err = decodeAsStringBlock(n.newWriteBlock, b, 0, context); err != nil {
				return err
			}
			continue
		}
	}
	if decodeByNestedBlock && n.NewBody != "" {
		return fmt.Errorf("can only set either one of `asraw`, `asstring` or `body`")
	}
	if n.NewBody != "" {
		newBody, diag := hclwrite.ParseConfig([]byte(fmt.Sprintf(`%s %s {
%s
}`, n.NewBlockType, strings.Join(n.Labels, " "), n.NewBody)), "", hcl.InitialPos)
		if diag.HasErrors() {
			return fmt.Errorf("cannot decode body %s: %+v", n.NewBody, diag)
		}
		n.newWriteBlock = newBody.Body().Blocks()[0]
	}
	formattedBlock, err := n.Format(n.newWriteBlock)
	if err == nil {
		n.newWriteBlock = formattedBlock
	}
	return nil
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
		resourceBlock := avmfix.BuildBlockWithSchema(avmBlock, &hcl.File{})
		err := resourceBlock.AutoFix()
		return resourceBlock.HclBlock.WriteBlock, err
	}
	if block.Type() == "variable" {
		variableBlock := avmfix.BuildVariableBlock(&hcl.File{}, avmBlock)
		err := variableBlock.AutoFix()
		return variableBlock.Block.WriteBlock, err
	}
	return nil, nil
}

func getRequiredStringAttribute(name string, block *golden.HclBlock, context *hcl.EvalContext) (string, error) {
	attr, ok := block.Attributes()[name]
	if !ok {
		return "", fmt.Errorf("`%s` is required", name)
	}
	v, err := attr.Value(context)
	if err != nil {
		return "", err
	}
	if v.Type() != cty.String {
		return "", fmt.Errorf("`%s` must be a string", name)
	}
	return v.AsString(), nil
}

func getOptionalStringAttribute(name string, block *golden.HclBlock, context *hcl.EvalContext) (*string, error) {
	attr, ok := block.Attributes()[name]
	if !ok {
		return nil, nil
	}
	v, err := attr.Value(context)
	if err != nil {
		return nil, err
	}
	if v.Type() != cty.String {
		return nil, fmt.Errorf("`%s` must be a string", name)
	}
	asString := v.AsString()
	return &asString, nil
}
