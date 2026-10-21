package pkg

import (
	"testing"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/stretchr/testify/require"
)

func TestRemoveObjectAttributesPreservesRetainedBytes(t *testing.T) {
	for _, tc := range []struct {
		name   string
		source string
		path   []string
		keys   []string
		want   string
		empty  bool
	}{
		{"first inline", `{ modtm = modtm, azapi = azapi.owner }`, nil, []string{"modtm"}, `{  azapi = azapi.owner }`, false},
		{"middle inline", `{ a = azapi.owner, modtm = modtm, b = azapi.other }`, nil, []string{"modtm"}, `{ a = azapi.owner,  b = azapi.other }`, false},
		{"last inline", `{ azapi = azapi.owner, modtm = modtm }`, nil, []string{"modtm"}, `{ azapi = azapi.owner,  }`, false},
		{"trailing comma", `{ azapi = azapi.owner, modtm = modtm, }`, nil, []string{"modtm"}, `{ azapi = azapi.owner,  }`, false},
		{"all entries", `{ modtm = modtm, random = random }`, nil, []string{"modtm", "random"}, `{   }`, true},
		{"consecutive entries", `{ a = a, b = b, c = c, d = d }`, nil, []string{"a", "b", "d"}, `{   c = c,  }`, false},
		{"colon syntax", `{ modtm : modtm, azapi : azapi.owner }`, nil, []string{"modtm"}, `{  azapi : azapi.owner }`, false},
		{"quoted dotted key", `({ "azapi.owner" = azapi.owner, azapi = { owner = azapi.other } })`, nil, []string{"azapi.owner"}, `({  azapi = { owner = azapi.other } })`, false},
		{"nested literal path", `{ "a.b" = { modtm = modtm, azapi = azapi.owner }, a = { b = { modtm = modtm } } }`, []string{"a.b"}, []string{"modtm"}, `{ "a.b" = {  azapi = azapi.owner }, a = { b = { modtm = modtm } } }`, false},
		{"nested path", `{ a = ({ b = { modtm = modtm, azapi = azapi.owner } }) }`, []string{"a", "b"}, []string{"modtm"}, `{ a = ({ b = {  azapi = azapi.owner } }) }`, false},
		{"escaped key", `{ "\u006dodtm" = modtm, azapi = azapi.owner }`, nil, []string{"modtm"}, `{  azapi = azapi.owner }`, false},
		{"empty key", `{ "" = modtm, azapi = azapi.owner }`, nil, []string{""}, `{  azapi = azapi.owner }`, false},
		{"missing key", `{ azapi = azapi.owner }`, nil, []string{"modtm"}, `{ azapi = azapi.owner }`, false},
		{"no keys", `{ azapi = azapi.owner }`, nil, nil, `{ azapi = azapi.owner }`, false},
		{"missing path", `{ azapi = azapi.owner }`, []string{"missing"}, []string{"modtm"}, `{ azapi = azapi.owner }`, false},
		{"empty object", `{ /* closing note */ }`, nil, []string{"modtm"}, `{ /* closing note */ }`, true},
		{"duplicate requested keys", `{ modtm = modtm }`, nil, []string{"modtm", "modtm"}, `{  }`, true},
		{"comments around separator", `{ azapi = azapi.owner /* keep */, modtm = modtm /* orphan, retained */, /* keep too */ other = other.owner }`, nil, []string{"modtm"}, `{ azapi = azapi.owner /* keep */,  /* orphan, retained */ /* keep too */ other = other.owner }`, false},
		{"multiline comments", "{\r\n  # keep alias\r\n  azapi = azapi.owner // keep trailing\r\n  modtm = modtm # orphan retained\r\n  # closing\r\n}", nil, []string{"modtm"}, "{\r\n  # keep alias\r\n  azapi = azapi.owner // keep trailing\r\n   # orphan retained\r\n  # closing\r\n}", false},
		{"retained multiline values", "{\n  modtm = modtm\n  description = <<-EOT\n  ${var.text}\n  EOT\n  aliases = [azapi.owner, # keep\n  azapi.other]\n}", nil, []string{"modtm"}, "{\n  \n  description = <<-EOT\n  ${var.text}\n  EOT\n  aliases = [azapi.owner, # keep\n  azapi.other]\n}", false},
		{"removed multiline value", "{\n  modtm = {\n    nested = [var.value]\n  }\n  azapi = azapi.owner\n}", nil, []string{"modtm"}, "{\n  \n  azapi = azapi.owner\n}", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tokens := objectMergeTestTokens(t, tc.source)
			for range 2 {
				result, empty, err := removeObjectAttributes(tokens, tc.path, tc.keys)
				require.NoError(t, err)
				require.Equal(t, tc.want, string(result.Bytes()))
				require.Equal(t, tc.empty, empty)
				_, diag := hclsyntax.ParseExpression(result.Bytes(), "test", hcl.InitialPos)
				require.False(t, diag.HasErrors(), diag.Error())
				tokens = result
			}
		})
	}
}

func TestRemoveObjectAttributesRejectsAmbiguity(t *testing.T) {
	for _, tc := range []struct {
		source string
		path   []string
	}{
		{`local.providers`, nil},
		{`merge({}, local.providers)`, nil},
		{`var.enabled ? { modtm = modtm } : {}`, nil},
		{`{ for k, v in local.providers : k => v }`, nil},
		{`{ modtm = modtm, (var.key) = azapi.owner }`, nil},
		{`{ modtm = modtm, "${var.key}" = azapi.owner }`, nil},
		{`{ modtm = modtm, "\u006dodtm" = modtm.other }`, nil},
		{`{ nested = local.providers }`, []string{"nested"}},
		{`{ nested = { modtm = modtm, (var.key) = azapi.owner } }`, []string{"nested"}},
		{`{ nested = {}, nested = {} }`, []string{"nested"}},
	} {
		t.Run(tc.source, func(t *testing.T) {
			tokens := objectMergeTestTokens(t, tc.source)
			before := string(tokens.Bytes())
			result, _, err := removeObjectAttributes(tokens, tc.path, []string{"modtm"})
			require.Error(t, err)
			require.Nil(t, result)
			require.Equal(t, before, string(tokens.Bytes()))
		})
	}
}
