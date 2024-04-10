package expr

import (
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/zclconf/go-cty/cty"
)

type Tokens hclwrite.Tokens

func (t Tokens) Traverse(traverser hcl.Traverser) Tokens {
	switch tt := traverser.(type) {
	case hcl.TraverseRoot:
		t = t.TraverseRoot(tt)
		break
	case hcl.TraverseAttr:
		t = t.TraverseAttr(tt)
		break
	case hcl.TraverseIndex:
		t = t.TraverseIndex(tt)
	}
	return t
}

func (t Tokens) TraverseRoot(tr hcl.TraverseRoot) Tokens {
	return append(t, &hclwrite.Token{
		Type:  hclsyntax.TokenIdent,
		Bytes: []byte(tr.Name),
	})
}

func (t Tokens) TraverseAttr(ta hcl.TraverseAttr) Tokens {
	return append(t.Dot(), &hclwrite.Token{
		Type:  hclsyntax.TokenIdent,
		Bytes: []byte(ta.Name),
	})
}

func (t Tokens) TraverseIndex(tt hcl.TraverseIndex) Tokens {
	return t.OBrack().Value(tt.Key).CBrack()
}

func (t Tokens) Dot() Tokens {
	return append(t, &hclwrite.Token{
		Type:  hclsyntax.TokenDot,
		Bytes: []byte("."),
	})
}

func (t Tokens) OBrack() Tokens {
	return append(t, &hclwrite.Token{
		Type:  hclsyntax.TokenOBrack,
		Bytes: []byte("["),
	})
}

func (t Tokens) CBrack() Tokens {
	return append(t, &hclwrite.Token{
		Type:  hclsyntax.TokenCBrack,
		Bytes: []byte("]"),
	})
}

func (t Tokens) Value(v cty.Value) Tokens {
	return append(t, hclwrite.TokensForValue(v)...)
}

func (t Tokens) Bytes() []byte {
	return hclwrite.Tokens(t).Bytes()
}
