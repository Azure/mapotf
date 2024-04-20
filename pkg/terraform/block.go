package terraform

import (
	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/zclconf/go-cty/cty"
)

type Block interface {
	EvalContext() cty.Value
	GetAttributes() map[string]*Attribute
	GetNestedBlocks() NestedBlocks
	WriteBody() *hclwrite.Body
}
