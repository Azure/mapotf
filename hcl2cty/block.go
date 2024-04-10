package hcl2cty

import (
	"github.com/hashicorp/hcl/v2"
	"github.com/zclconf/go-cty/cty"
	"github.com/zclconf/go-cty/cty/function"
)

type HclBlock struct {
}

func (b *HclBlock) EvalContext() *hcl.EvalContext {
	return &hcl.EvalContext{
		Variables: make(map[string]cty.Value),
		Functions: make(map[string]function.Function),
	}
}

func (b *HclBlock) Address() hcl.Traversal {
	return nil
}
