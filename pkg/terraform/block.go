package terraform

import (
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/zclconf/go-cty/cty"
)

type Block interface {
	EvalContext() cty.Value
	GetAttributes() map[string]*Attribute
	GetNestedBlocks() NestedBlocks
	WriteBody() *hclwrite.Body
	SetAttributeRaw(name string, tokens hclwrite.Tokens)
	AppendBlock(block *hclwrite.Block)
	Range() hcl.Range
}

func lockBlockFile(b Block) func() {
	fn := b.Range().Filename
	lock.Lock(fn)
	return func() {
		lock.Unlock(fn)
	}
}
