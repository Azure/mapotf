package expr_test

import (
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/lonegunmanb/mptf/expr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestScopeTraversal(t *testing.T) {
	cases := []struct {
		desc     string
		exp      string
		expected expr.Tokens
	}{
		{
			desc: "all_attrs",
			exp:  "locals.a.b",
			expected: []*hclwrite.Token{
				{
					Type:  hclsyntax.TokenIdent,
					Bytes: []byte("locals"),
				},
				{
					Type:  hclsyntax.TokenDot,
					Bytes: []byte("."),
				},
				{
					Type:  hclsyntax.TokenIdent,
					Bytes: []byte("a"),
				},
				{
					Type:  hclsyntax.TokenDot,
					Bytes: []byte("."),
				},
				{
					Type:  hclsyntax.TokenIdent,
					Bytes: []byte("b"),
				},
			},
		},
		{
			desc: "traverse_number_index_inside",
			exp:  "locals.a[0].b",
			expected: []*hclwrite.Token{
				{
					Type:  hclsyntax.TokenIdent,
					Bytes: []byte("locals"),
				},
				{
					Type:  hclsyntax.TokenDot,
					Bytes: []byte("."),
				},
				{
					Type:  hclsyntax.TokenIdent,
					Bytes: []byte("a"),
				},
				{
					Type:  hclsyntax.TokenOBrack,
					Bytes: []byte("["),
				},
				{
					Type:  hclsyntax.TokenNumberLit,
					Bytes: []byte("0"),
				},
				{
					Type:  hclsyntax.TokenCBrack,
					Bytes: []byte("]"),
				},
				{
					Type:  hclsyntax.TokenDot,
					Bytes: []byte("."),
				},
				{
					Type:  hclsyntax.TokenIdent,
					Bytes: []byte("b"),
				},
			},
		},
		{
			desc: "traverse_string_index_inside",
			exp:  `locals.a["name"].b`,
			expected: []*hclwrite.Token{
				{
					Type:  hclsyntax.TokenIdent,
					Bytes: []byte("locals"),
				},
				{
					Type:  hclsyntax.TokenDot,
					Bytes: []byte("."),
				},
				{
					Type:  hclsyntax.TokenIdent,
					Bytes: []byte("a"),
				},
				{
					Type:  hclsyntax.TokenOBrack,
					Bytes: []byte("["),
				},
				{
					Type:  hclsyntax.TokenOQuote,
					Bytes: []byte("\""),
				},
				{
					Type:  hclsyntax.TokenQuotedLit,
					Bytes: []byte(`name`),
				},
				{
					Type:  hclsyntax.TokenCQuote,
					Bytes: []byte("\""),
				},
				{
					Type:  hclsyntax.TokenCBrack,
					Bytes: []byte("]"),
				},
				{
					Type:  hclsyntax.TokenDot,
					Bytes: []byte("."),
				},
				{
					Type:  hclsyntax.TokenIdent,
					Bytes: []byte("b"),
				},
			},
		},
	}
	for _, c := range cases {
		cc := c
		t.Run(cc.desc, func(t *testing.T) {
			e, diag := hclsyntax.ParseExpression([]byte(cc.exp), "main.tf", hcl.InitialPos)
			require.False(t, diag.HasErrors())
			traversalExpr, ok := e.(*hclsyntax.ScopeTraversalExpr)
			require.True(t, ok)
			sut := expr.Pointer(expr.ScopeTraversalExpr(*traversalExpr))
			actual := sut.Tokens()
			assert.Equal(t, cc.expected, actual, string(cc.expected.Bytes()), string(actual.Bytes()))
		})
	}
}
