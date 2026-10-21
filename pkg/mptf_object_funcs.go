package pkg

import (
	"fmt"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/zclconf/go-cty/cty"
	"github.com/zclconf/go-cty/cty/function"
)

var ObjectEntriesFunc = function.New(&function.Spec{
	Description: "Inspect literal object keys and unevaluated value expressions.",
	Params: []function.Parameter{
		{Name: "input", Type: cty.DynamicPseudoType},
	},
	Type: function.StaticReturnType(cty.Map(cty.String)),
	Impl: func(args []cty.Value, _ cty.Type) (cty.Value, error) {
		input := args[0]
		var source []byte
		switch {
		case input.Type() == cty.String:
			source = []byte(input.AsString())
		case input.Type().IsObjectType() || input.Type().IsMapType():
			if !input.IsWhollyKnown() {
				return cty.NilVal, fmt.Errorf("object_entries input must be wholly known")
			}
			source = hclwrite.TokensForValue(input).Bytes()
		default:
			return cty.NilVal, fmt.Errorf("object_entries input must be an object expression string, object or map")
		}
		object, err := parseLiteralObject(source)
		if err != nil {
			return cty.NilVal, err
		}
		items, err := staticObjectItems(object)
		if err != nil {
			return cty.NilVal, err
		}
		values := make(map[string]cty.Value, len(items))
		for key, item := range items {
			values[key] = cty.StringVal(string(item.ValueExpr.Range().SliceBytes(source)))
		}
		if len(values) == 0 {
			return cty.MapValEmpty(cty.String), nil
		}
		return cty.MapVal(values), nil
	},
})

func parseLiteralObject(source []byte) (*hclsyntax.ObjectConsExpr, error) {
	expr, diag := hclsyntax.ParseExpression(source, "object-expression", hcl.InitialPos)
	if diag.HasErrors() {
		return nil, fmt.Errorf("cannot parse object expression: %s", diag.Error())
	}
	object, ok := unparenthesizedExpression(expr).(*hclsyntax.ObjectConsExpr)
	if !ok {
		return nil, fmt.Errorf("expression must be a literal object")
	}
	return object, nil
}
