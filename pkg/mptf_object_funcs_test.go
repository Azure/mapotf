package pkg_test

import (
	"testing"

	"github.com/Azure/mapotf/pkg"
	"github.com/stretchr/testify/require"
	"github.com/zclconf/go-cty/cty"
)

func TestObjectEntries(t *testing.T) {
	for _, tc := range []struct {
		name  string
		input cty.Value
		want  map[string]string
	}{
		{"provider traversals", cty.StringVal(`{ azapi = azapi.owner, modtm = modtm }`), map[string]string{"azapi": "azapi.owner", "modtm": "modtm"}},
		{"quoted dotted key and parentheses", cty.StringVal(`({ ("azapi.owner") : (azapi.owner), "\u006dodtm" = modtm })`), map[string]string{"azapi.owner": "(azapi.owner)", "modtm": "modtm"}},
		{"unevaluated nested values", cty.StringVal(`{ source = "Azure/azapi", nested = { aliases = [azapi.owner] }, enabled = var.enabled ? true : false }`), map[string]string{"source": `"Azure/azapi"`, "nested": "{ aliases = [azapi.owner] }", "enabled": "var.enabled ? true : false"}},
		{"comments within value", cty.StringVal("{ aliases = [azapi.owner, # keep\nazapi.other] }"), map[string]string{"aliases": "[azapi.owner, # keep\nazapi.other]"}},
		{"literal dotted and nested keys differ", cty.StringVal(`{ "a.b" = azapi.owner, a = { b = modtm } }`), map[string]string{"a.b": "azapi.owner", "a": "{ b = modtm }"}},
		{"empty object expression", cty.StringVal(`{ # keep
}`), map[string]string{}},
		{"decoded empty object", cty.EmptyObjectVal, map[string]string{}},
		{"decoded literal object", cty.ObjectVal(map[string]cty.Value{"source": cty.StringVal("Azure/azapi"), "enabled": cty.True}), map[string]string{"source": `"Azure/azapi"`, "enabled": "true"}},
		{"decoded map", cty.MapVal(map[string]cty.Value{"azapi": cty.StringVal("azapi.owner")}), map[string]string{"azapi": `"azapi.owner"`}},
		{"empty decoded map", cty.MapValEmpty(cty.String), map[string]string{}},
		{"null member", cty.ObjectVal(map[string]cty.Value{"value": cty.NullVal(cty.String)}), map[string]string{"value": "null"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			actual, err := pkg.ObjectEntriesFunc.Call([]cty.Value{tc.input})
			require.NoError(t, err)
			require.True(t, actual.Type().Equals(cty.Map(cty.String)))
			values := map[string]string{}
			for key, value := range actual.AsValueMap() {
				values[key] = value.AsString()
			}
			require.Equal(t, tc.want, values)
		})
	}
}

func TestObjectEntriesRejectsUnsupportedInputs(t *testing.T) {
	for _, input := range []cty.Value{
		cty.StringVal(`local.providers`),
		cty.StringVal(`merge({}, local.providers)`),
		cty.StringVal(`var.enabled ? { modtm = modtm } : {}`),
		cty.StringVal(`{ for k, v in local.providers : k => v }`),
		cty.StringVal(`{ (var.key) = modtm }`),
		cty.StringVal(`{ "${var.key}" = modtm }`),
		cty.StringVal(`{ azapi.owner = azapi.owner }`),
		cty.StringVal(`{ azapi = azapi.owner, "\u0061zapi" = azapi.other }`),
		cty.StringVal(`{ 1 = modtm }`),
		cty.StringVal(`{ modtm = }`),
		cty.StringVal(`null`),
		cty.StringVal(`[]`),
		cty.StringVal(`"not an object"`),
		cty.NumberIntVal(1),
		cty.NullVal(cty.String),
		cty.ListValEmpty(cty.String),
		cty.ObjectVal(map[string]cty.Value{"source": cty.UnknownVal(cty.String)}),
	} {
		t.Run(input.GoString(), func(t *testing.T) {
			_, err := pkg.ObjectEntriesFunc.Call([]cty.Value{input})
			require.Error(t, err)
		})
	}
}
