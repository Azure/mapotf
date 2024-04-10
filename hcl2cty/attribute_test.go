package hcl2cty_test

import (
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestAttributeAddress(t *testing.T) {
	config := `
locals {
	a = {
		name = "a"
	}
	b = local.a.name
}
`
	cfg, _ := hclsyntax.ParseConfig([]byte(config), "main.tf", hcl.InitialPos)
	attr := cfg.Body.(*hclsyntax.Body).Blocks[0].Body.Attributes["b"]
	expr := attr.Expr
	variables := expr.Variables()
	assert.NotNil(t, variables)
	assert.NotNil(t, expr)
}
