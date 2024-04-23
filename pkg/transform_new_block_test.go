package pkg_test

import (
	"strings"
	"testing"

	"github.com/Azure/golden"
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/lonegunmanb/mptf/pkg"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zclconf/go-cty/cty"
)

func TestNewBlockTransform_Decode(t *testing.T) {
	code := `transform "new_block" test {
	new_block_type = "variable"
	filename = "variables.tf"
	labels = ["test"]
	asraw {
	  type        = string
      description = "This is description"
	}
}`
	readFile, diag := hclsyntax.ParseConfig([]byte(code), "test.mptl.hcl", hcl.InitialPos)
	require.False(t, diag.HasErrors())
	writeFile, diag := hclwrite.ParseConfig([]byte(code), "test.mptl.hcl", hcl.InitialPos)
	require.False(t, diag.HasErrors())
	hclBlock := golden.NewHclBlock(readFile.Body.(*hclsyntax.Body).Blocks[0], writeFile.Body().Blocks()[0], nil)
	sut := new(pkg.NewBlockTransform)
	err := sut.Decode(hclBlock, &hcl.EvalContext{
		Variables: make(map[string]cty.Value),
	})
	require.NoError(t, err)
	assert.Equal(t, "variable", sut.NewBlockType)
	assert.Equal(t, "variables.tf", sut.FileName)
	assert.Equal(t, []string{"test"}, sut.Labels)
	newWriteBlock := sut.NewWriteBlock()
	assert.Equal(t, "variable", newWriteBlock.Type())
	assert.Equal(t, []string{"test"}, newWriteBlock.Labels())
	attributes := newWriteBlock.Body().Attributes()
	assert.Equal(t, `"This is description"`, strings.TrimSpace(string(attributes["description"].Expr().BuildTokens(nil).Bytes())))
	assert.Equal(t, "string", strings.TrimSpace(string(attributes["type"].Expr().BuildTokens(nil).Bytes())))
}

func TestNewBlockTransform_DecodeTwiceShouldGotCorrectLabels(t *testing.T) {
	code := `transform "new_block" test {
	new_block_type = "variable"
	filename = "variables.tf"
	labels = ["test"]
	asraw {
	  type        = string
      description = "This is description"
	}
}`
	readFile, diag := hclsyntax.ParseConfig([]byte(code), "test.mptl.hcl", hcl.InitialPos)
	require.False(t, diag.HasErrors())
	writeFile, diag := hclwrite.ParseConfig([]byte(code), "test.mptl.hcl", hcl.InitialPos)
	require.False(t, diag.HasErrors())
	hclBlock := golden.NewHclBlock(readFile.Body.(*hclsyntax.Body).Blocks[0], writeFile.Body().Blocks()[0], nil)
	sut := new(pkg.NewBlockTransform)
	ctx := &hcl.EvalContext{
		Variables: make(map[string]cty.Value),
	}
	err := sut.Decode(hclBlock, ctx)
	require.NoError(t, err)
	err = sut.Decode(hclBlock, ctx)
	require.NoError(t, err)
	assert.Equal(t, []string{"test"}, sut.Labels)
}
