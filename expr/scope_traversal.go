package expr

import (
	"github.com/hashicorp/hcl/v2/hclsyntax"
)

type ScopeTraversalExpr hclsyntax.ScopeTraversalExpr

func (e *ScopeTraversalExpr) Tokens() Tokens {
	tokens := Tokens{}
	for _, t := range e.Traversal {
		tokens = tokens.Traverse(t)
	}
	return tokens
}
