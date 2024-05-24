package terraform

import (
	"strings"

	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/hashicorp/hcl/v2/hclwrite"
)

type Attribute struct {
	Name string
	*hclsyntax.Attribute
	WriteAttribute *hclwrite.Attribute
}

func NewAttribute(name string, attribute *hclsyntax.Attribute, writeAttribute *hclwrite.Attribute) *Attribute {
	r := &Attribute{
		Name:           name,
		Attribute:      attribute,
		WriteAttribute: writeAttribute,
	}
	return r
}

func (a *Attribute) String() string {
	return strings.TrimSpace(string(a.WriteAttribute.Expr().BuildTokens(hclwrite.Tokens{}).Bytes()))
}
