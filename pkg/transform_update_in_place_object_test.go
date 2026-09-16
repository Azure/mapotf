package pkg

import (
	"fmt"
	"testing"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/stretchr/testify/require"
)

func TestMergeObjectAttribute(t *testing.T) {
	cases := []struct {
		name   string
		target string
		patch  string
		want   string
	}{
		{
			name:   "scalar replacement",
			target: `"old"`,
			patch:  `"new"`,
			want:   `"new"`,
		},
		{
			name:   "scalar expression replacement",
			target: `local.old`,
			patch:  `join("-", [var.name, "new"])`,
			want:   `join("-", [var.name, "new"])`,
		},
		{
			name:   "boolean replacement",
			target: `false`,
			patch:  `true`,
			want:   `true`,
		},
		{
			name:  "missing attribute",
			patch: `{ source = "Azure/azapi", configuration_aliases = [azapi.primary] }`,
			want:  `{ source = "Azure/azapi", configuration_aliases = [azapi.primary] }`,
		},
		{
			name:   "empty patch preserves the object",
			target: `{ source = "Azure/azapi", /* keep */ version = "~> 2.0" }`,
			patch:  `{}`,
			want:   `{ source = "Azure/azapi", /* keep */ version = "~> 2.0" }`,
		},
		{
			name:   "parenthesized objects and quoted keys",
			target: `({ ("source") : "old", configuration_aliases = [azapi.primary] })`,
			patch:  `({ source = "Azure/azapi" })`,
			want:   `({ ("source") : "Azure/azapi", configuration_aliases = [azapi.primary] })`,
		},
		{
			name:   "escaped key matches its literal name",
			target: `{ "\u0073ource" = "old", "other.key" = local.keep }`,
			patch:  `{ source = "new" }`,
			want:   `{ "\u0073ource" = "new", "other.key" = local.keep }`,
		},
		{
			name:   "empty and keyword keys",
			target: `{ "" = 1, true = 2, null = 3 }`,
			patch:  `{ "" = 4, "true" = 5 }`,
			want:   `{ "" = 4, true = 5, null = 3 }`,
		},
		{
			name:   "preserve inline comments and add a separator",
			target: `{ source = "old" /* source note */ }`,
			patch:  `{ source = "new", version = "~> 2.12" }`,
			want:   `{ source = "new", /* source note */ version = "~> 2.12" }`,
		},
		{
			name:   "preserve comment before existing trailing comma",
			target: `{ source = "old" /* source note */, }`,
			patch:  `{ version = "~> 2.12" }`,
			want:   `{ source = "old" /* source note */, version = "~> 2.12" }`,
		},
		{
			name: "preserve standalone and end of line comments",
			target: `{
  source = "old" // source note
  # Keep the closing note.
}`,
			patch: `{ version = "~> 2.12" }`,
			want: `{
  source = "old" // source note
  # Keep the closing note.
  version = "~> 2.12"
}`,
		},
		{
			name: "empty multiline object with comments",
			target: `{
  # Provider metadata.
}`,
			patch: `{ source = "new" }`,
			want: `{
  # Provider metadata.
  source = "new"
}`,
		},
		{
			name:   "nested values are replaced shallowly",
			target: `{ nested = { old = 1, keep = 2 }, other = { keep = local.expression } }`,
			patch:  `{ nested = { new = 3 } }`,
			want:   `{ nested = { new = 3 }, other = { keep = local.expression } }`,
		},
		{
			name:   "append quoted key and unevaluated expression",
			target: `{ source = "old" }`,
			patch:  `{ "other.key" : /* important */ merge(local.options, { nested = var.value }) }`,
			want:   `{ source = "old", "other.key" : /* important */ merge(local.options, { nested = var.value }) }`,
		},
		{
			name:   "append multiline value to inline object",
			target: `{ source = "old" }`,
			patch: `{
  aliases = [
    azapi.primary, # Primary.
    azapi.secondary,
  ]
}`,
			want: `{ source = "old"
  aliases = [
    azapi.primary, # Primary.
    azapi.secondary,
  ]
}`,
		},
		{
			name: "preserve unrelated heredoc",
			target: `{
  source = "old"
  description = <<-EOT
    ${var.description}
  EOT
}`,
			patch: `{ source = "new", version = "~> 2.12" }`,
			want: `{
  source = "new"
  description = <<-EOT
    ${var.description}
  EOT
  version = "~> 2.12"
}`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var target hclwrite.Tokens
			if tc.target != "" {
				target = objectMergeTestTokens(t, tc.target)
			}
			patch := objectMergeTestTokens(t, tc.patch)
			var previous string
			for run := 0; run < 2; run++ {
				actual, err := mergeObjectAttribute(target, patch)
				require.NoError(t, err)
				format := func(source string) string {
					return string(hclwrite.Format([]byte(fmt.Sprintf("value = %s\n", source))))
				}
				require.Equal(t, format(tc.want), format(string(actual.Bytes())))
				if run > 0 {
					require.Equal(t, previous, string(actual.Bytes()))
				}
				previous = string(actual.Bytes())
				target = actual
			}
		})
	}
}

func TestMergeObjectAttributeRejectsUnsafeExpressions(t *testing.T) {
	cases := []struct {
		name   string
		target string
		patch  string
		error  string
	}{
		{"target traversal", `local.providers`, `{ source = "new" }`, "target must be a literal object"},
		{"target function", `merge({}, local.providers)`, `{ source = "new" }`, "target must be a literal object"},
		{"target conditional", `var.enabled ? { source = "old" } : {}`, `{ source = "new" }`, "target must be a literal object"},
		{"target comprehension", `{ for k, v in local.providers : k => v }`, `{ source = "new" }`, "target must be a literal object"},
		{"scalar target", `"old"`, `{ source = "new" }`, "target must be a literal object"},
		{"null target", `null`, `{ source = "new" }`, "target must be a literal object"},
		{"list target", `[]`, `{ source = "new" }`, "target must be a literal object"},
		{"conflicting scalar patch", `{ source = "old" }`, `"new"`, "non-object patch"},
		{"nonliteral patch", `{ source = "old" }`, `local.patch`, "non-object patch"},
		{"comprehension patch", `{}`, `{ for k, v in local.patch : k => v }`, "not a for expression"},
		{"comprehension creating attribute", "", `{ for k, v in local.patch : k => v }`, "not a for expression"},
		{"computed target key", `{ (var.key) = "old" }`, `{ source = "new" }`, "invalid target object"},
		{"interpolated target key", `{ "${var.key}" = "old" }`, `{ source = "new" }`, "invalid target object"},
		{"computed patch key", `{}`, `{ (var.key) = "new" }`, "invalid patch object"},
		{"computed key creating attribute", "", `{ (var.key) = "new" }`, "invalid patch object"},
		{"interpolated patch key", `{}`, `{ "${var.key}" = "new" }`, "invalid patch object"},
		{"function patch key", `{}`, `{ (upper("source")) = "new" }`, "literal name or string"},
		{"numeric key", `{}`, `{ 1 = "new" }`, "literal name or string"},
		{"duplicate target key", `{ source = "old", "source" = "older" }`, `{ source = "new" }`, `duplicate object key "source"`},
		{"duplicate patch key", `{}`, `{ source = "new", "\u0073ource" = "other" }`, `duplicate object key "source"`},
		{"malformed target", `{ source = }`, `{ source = "new" }`, "cannot parse target expression"},
		{"malformed patch", `{}`, `{ source = }`, "cannot parse patch expression"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var target hclwrite.Tokens
			if tc.target != "" {
				target = objectMergeTestTokens(t, tc.target)
			}
			actual, err := mergeObjectAttribute(target, objectMergeTestTokens(t, tc.patch))
			require.ErrorContains(t, err, tc.error)
			require.Nil(t, actual)
		})
	}
}

func objectMergeTestTokens(t *testing.T, source string) hclwrite.Tokens {
	t.Helper()
	tokens, diag := hclsyntax.LexExpression([]byte(source), "test", hcl.InitialPos)
	require.False(t, diag.HasErrors(), diag.Error())
	return writerTokens(tokens[:len(tokens)-1])
}
