package pkg

import (
	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/zclconf/go-cty/cty"
	"github.com/zclconf/go-cty/cty/function"
)

var ToHclFunc = function.New(&function.Spec{
	Description: "Convert an cty.Value to HCL config in string format",
	Params: []function.Parameter{
		{
			Name: "input",
			Type: cty.DynamicPseudoType,
		},
	},
	Type: function.StaticReturnType(cty.String),
	Impl: func(args []cty.Value, retType cty.Type) (cty.Value, error) {
		input := args[0]
		return cty.StringVal(string(hclwrite.TokensForValue(input).BuildTokens(nil).Bytes())), nil
	},
})
