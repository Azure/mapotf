package pkg

import (
	"fmt"
	"sort"
	"strings"

	"github.com/Azure/golden"
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/zclconf/go-cty/cty"
)

var _ golden.ApplyBlock = &NewBlockTransform{}
var _ golden.CustomDecode = &NewBlockTransform{}

// variableAttributePriorities mirrors the avmfix priority map used to order
// the attributes inside a `variable` block: lower number wins. Attributes not
// in this map fall back to math.MaxInt so they're emitted after the recognised
// ones, in alphabetical order.
var variableAttributePriorities = map[string]int{
	"type":        0,
	"default":     1,
	"description": 2,
	"nullable":    3,
	"sensitive":   4,
}

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
	if block.Type() != "variable" {
		// For resource / data blocks the legacy avmfix path called
		// BuildBlockWithSchema with an empty hcl.File, which made it a no-op.
		// Match that behaviour exactly.
		return block, nil
	}
	return formatVariableBlock(block)
}

// formatVariableBlock applies the avmfix-equivalent layout to a `variable`
// block: attributes ordered type → default → description → nullable → sensitive
// (unknown attrs after, alphabetically); `nullable = true` and `sensitive = false`
// literal defaults dropped; nested blocks preserved after the attributes with a
// blank-line separator.
func formatVariableBlock(block *hclwrite.Block) (*hclwrite.Block, error) {
	bytes := block.BuildTokens(nil).Bytes()
	syntaxFile, diag := hclsyntax.ParseConfig(bytes, "dummy.hcl", hcl.InitialPos)
	if diag.HasErrors() {
		return nil, diag
	}
	syntaxBlock := syntaxFile.Body.(*hclsyntax.Body).Blocks[0]

	body := block.Body()
	writeAttrs := body.Attributes()
	writeBlocks := body.Blocks()

	keep := make([]string, 0, len(writeAttrs))
	for name := range writeAttrs {
		if dropDefaultBoolLiteral(name, syntaxBlock.Body.Attributes) {
			continue
		}
		keep = append(keep, name)
	}
	sort.SliceStable(keep, func(i, j int) bool {
		pi, oki := variableAttributePriorities[keep[i]]
		pj, okj := variableAttributePriorities[keep[j]]
		if oki != okj {
			return oki && !okj
		}
		if pi != pj {
			return pi < pj
		}
		return keep[i] < keep[j]
	})

	body.Clear()
	for _, name := range keep {
		body.AppendUnstructuredTokens(writeAttrs[name].BuildTokens(nil))
	}
	if len(writeBlocks) > 0 {
		body.AppendNewline()
		for _, nb := range writeBlocks {
			body.AppendBlock(nb)
		}
	}
	return block, nil
}

// dropDefaultBoolLiteral returns true when the attribute represents a
// redundant default that avmfix used to strip: `nullable = true` or
// `sensitive = false`, expressed as a literal boolean.
func dropDefaultBoolLiteral(name string, attrs map[string]*hclsyntax.Attribute) bool {
	syn, ok := attrs[name]
	if !ok {
		return false
	}
	literal, ok := syn.Expr.(*hclsyntax.LiteralValueExpr)
	if !ok {
		return false
	}
	switch name {
	case "nullable":
		return literal.Val.True()
	case "sensitive":
		return literal.Val.False()
	}
	return false
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
