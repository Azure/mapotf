package hcl2cty

import (
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/hashicorp/hcl/v2/hclwrite"
)

type HclAttribute struct {
	Block *HclBlock
	*hclsyntax.Attribute
	wa *hclwrite.Attribute
}

func (a *HclAttribute) Address() hcl.Traversal {
	b := a.Attribute.Expr
	println(b == nil)
	return nil
}

func (a *HclAttribute) EvalContext() *hcl.EvalContext {
	r := a.Block.EvalContext().NewChild()
	return r
}
