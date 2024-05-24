package pkg_test

import (
	"strings"
	"testing"

	"github.com/Azure/golden"
	"github.com/Azure/mapotf/pkg"
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zclconf/go-cty/cty"
)

func TestNewBlockTransform_DecodeBody(t *testing.T) {
	cases := []struct {
		desc     string
		code     string
		expected map[string]string
	}{
		{
			desc: "asraw",
			code: `transform "new_block" test {
	new_block_type = "variable"
	filename = "variables.tf"
	labels = ["test"]
	asraw {
	  type        = string
      description = "This is description"
	}
}`,
			expected: map[string]string{
				"description": `"This is description"`,
				"type":        "string",
			},
		},
		{
			desc: "asstring",
			code: `transform "new_block" test {
	new_block_type = "variable"
	filename = "variables.tf"
	labels = ["test"]
	asstring {
	  type        = "string"
      description = "\"This is description\""
	}
}`,
			expected: map[string]string{
				"description": `"This is description"`,
				"type":        "string",
			},
		},
		{
			desc: "hybrid",
			code: `transform "new_block" test {
	new_block_type = "variable"
	filename = "variables.tf"
	labels = ["test"]
	asraw {
	  description = "This is description"
	}
	asstring {
	  type        = "string"
      description = "\"This is description\""
	}
}`,
			expected: map[string]string{
				"description": `"This is description"`,
				"type":        "string",
			},
		},
	}
	for _, c := range cases {
		t.Run(c.desc, func(t *testing.T) {
			readFile, diag := hclsyntax.ParseConfig([]byte(c.code), "test.mptl.hcl", hcl.InitialPos)
			require.False(t, diag.HasErrors())
			writeFile, diag := hclwrite.ParseConfig([]byte(c.code), "test.mptl.hcl", hcl.InitialPos)
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
			for attributeName, expected := range c.expected {
				assert.Equal(t, expected, strings.TrimSpace(string(attributes[attributeName].Expr().BuildTokens(nil).Bytes())))
			}
		})
	}
}

func TestNewBlockTransform_DecodeHybridNestedBlock(t *testing.T) {
	code := `transform "new_block" test {
	new_block_type = "fake_block"
	filename = "variables.tf"
	labels = ["test"]
	asraw {
	  top_block {
		id = var.id
      }
	}
	asstring {
	  top_block {
		name = "\"John\""
      }
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
	wb := sut.NewWriteBlock()
	blocks := wb.Body().Blocks()
	assert.Len(t, blocks, 2)
	assert.Equal(t, "var.id", strings.TrimSpace(string(blocks[0].Body().Attributes()["id"].Expr().BuildTokens(nil).Bytes())))
	assert.Equal(t, `"John"`, strings.TrimSpace(string(blocks[1].Body().Attributes()["name"].Expr().BuildTokens(nil).Bytes())))
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
