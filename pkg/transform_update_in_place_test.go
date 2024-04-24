package pkg_test

import (
	"context"
	"strings"
	"testing"

	"github.com/Azure/golden"
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/lonegunmanb/hclfuncs"
	"github.com/lonegunmanb/mptf/pkg"
	"github.com/lonegunmanb/mptf/pkg/terraform"
	"github.com/prashantv/gostub"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zclconf/go-cty/cty"
)

func TestUpdateInPlaceTransform_Decode(t *testing.T) {
	cases := []struct {
		desc               string
		cfg                string
		expectedPatchBlock string
	}{
		{
			desc: "pure string",
			cfg: `
transform "update_in_place" this {
	target_block_address = "resource.fake_resource.this"
	asstring{
	  tags = "{ hello = world }"
	}
}
`,
			expectedPatchBlock: `patch {
	tags = { hello = world }
}`,
		},
		{
			desc: "function call",
			cfg: `
transform "update_in_place" this {
	target_block_address = "resource.fake_resource.this"
	asstring{
	  tags = "\"${join("-", ["foo", "bar", "baz"])}\""
	}
}
`,
			expectedPatchBlock: `patch {
	tags = "foo-bar-baz"
}`,
		},
		{
			desc: "string inside nested block",
			cfg: `
transform "update_in_place" this {
	target_block_address = "resource.fake_resource.this"
	asstring{
	  top_block {
	    id = "123"
	  }
	}
}
`,
			expectedPatchBlock: `patch {
	top_block {
		id = 123
	}
}`,
		},
		{
			desc: "nested block in nested block",
			cfg: `
transform "update_in_place" this {
	target_block_address = "resource.fake_resource.this"
	asstring{
	  top_block {
		second_block {
			id = "123"
		}
	  }
	}
}
`,
			expectedPatchBlock: `patch {
	top_block {
		second_block {
			id = 123
		}
	}
}`,
		},
		{
			desc: "raw token",
			cfg: `
transform "update_in_place" this {
	target_block_address = "resource.fake_resource.this"
	asraw{
	  tags = { hello = "world" }
	}
}
`,
			expectedPatchBlock: `patch {
	tags = { hello = "world" }
}`,
		},
		{
			desc: "raw token with function call",
			cfg: `
transform "update_in_place" this {
	target_block_address = "resource.fake_resource.this"
	asraw{
	  tags = merge({}, { hello = "world" })
	}
}
`,
			expectedPatchBlock: `patch {
	tags = merge({}, { hello = "world" })
}`,
		},
		{
			desc: "reserved keywords inside raw block",
			cfg: `
transform "update_in_place" this {
	target_block_address = "resource.fake_resource.this"
	asraw{
	  target_block_address = uuid()
      asraw {
	    id = timestamp()
      }
	}
}
`,
			expectedPatchBlock: `patch {
	target_block_address = uuid()
    asraw {
	  id = timestamp()
    }
}`,
		},
		{
			desc: "both string and raw updates, string should take precedence",
			cfg: `
transform "update_in_place" this {
	target_block_address = "resource.fake_resource.this"
	asraw{
	  id = 123
	}
	asstring{
	  id = "456"
	}
}
`,
			expectedPatchBlock: `patch {
	id = 456
}`,
		},
	}
	for _, c := range cases {
		t.Run(c.desc, func(t *testing.T) {
			stub := gostub.Stub(&terraform.Fs, fakeFs(map[string]string{
				"/main.tf": `
resource "fake_resource" this {
  tags = null
}`,
			}))
			defer stub.Reset()
			readFile, diag := hclsyntax.ParseConfig([]byte(c.cfg), "test.hcl", hcl.InitialPos)
			require.Falsef(t, diag.HasErrors(), diag.Error())
			writeFile, diag := hclwrite.ParseConfig([]byte(c.cfg), "test.hcl", hcl.InitialPos)
			require.Falsef(t, diag.HasErrors(), diag.Error())
			hclBlock := golden.NewHclBlock(readFile.Body.(*hclsyntax.Body).Blocks[0], writeFile.Body().Blocks()[0], nil)
			cfg, err := pkg.NewMetaProgrammingTFConfig("/", nil, context.TODO())
			require.NoError(t, err)
			sut := &pkg.UpdateInPlaceTransform{
				BaseBlock: golden.NewBaseBlock(cfg, hclBlock),
			}
			err = sut.Decode(hclBlock, &hcl.EvalContext{
				Variables: map[string]cty.Value{},
				Functions: hclfuncs.Functions("."),
			})
			require.NoError(t, err)
			assert.Equal(t, "resource.fake_resource.this", sut.TargetBlockAddress)
			updateBlock := sut.UpdateBlock()

			actual := string(updateBlock.BuildTokens(hclwrite.Tokens{}).Bytes())
			assert.Equal(t, formatHcl(c.expectedPatchBlock), formatHcl(actual))
		})
	}
}

func TestUpdateInPlaceTransform_UseForEachInDecode(t *testing.T) {
	stub := gostub.Stub(&terraform.Fs, fakeFs(map[string]string{
		"/main.tf": `
resource "fake_resource" this {
  tags = {}
}

resource "fake_resource" that {
}
`,
	}))
	defer stub.Reset()
	hclBlocks := newHclBlocks(t, `
data resource "fake_resource" {
  resource_type = "fake_resource"
}

transform update_in_place "fake_resource" {
	for_each = data.resource.fake_resource.result.fake_resource
	target_block_address = each.value.mptf.block_address
	asstring{
	  tags = "merge(${try(coalesce(each.value.tags, "{}"), "{}")}, { \n block_address = \"${each.value.mptf.block_address}\" \n file_name = \"${each.value.mptf.range.file_name}\"\n  })"
	}
}
`)
	cfg, err := pkg.NewMetaProgrammingTFConfig("/", nil, context.TODO())
	require.NoError(t, err)
	err = cfg.Init(hclBlocks)
	require.NoError(t, err)
	err = cfg.RunPrePlan()
	require.NoError(t, err)
	err = cfg.RunPlan()
	require.NoError(t, err)
	vertices := cfg.BaseConfig.GetVertices()
	b, ok := vertices["transform.update_in_place.fake_resource[resource.fake_resource.this]"]
	require.True(t, ok)
	updateTransformBlock, ok := b.(*pkg.UpdateInPlaceTransform)
	require.True(t, ok)
	ub := updateTransformBlock.UpdateBlock()
	actual := string(ub.BuildTokens(hclwrite.Tokens{}).Bytes())
	expected := `patch {
	tags = merge({}, { 
  block_address = "resource.fake_resource.this"
  file_name = "main.tf"
})
}`
	assert.Equal(t, formatHcl(expected), formatHcl(actual))
	b, ok = vertices["transform.update_in_place.fake_resource[resource.fake_resource.that]"]
	require.True(t, ok)
	updateTransformBlock, ok = b.(*pkg.UpdateInPlaceTransform)
	require.True(t, ok)
	ub = updateTransformBlock.UpdateBlock()
	actual = string(ub.BuildTokens(hclwrite.Tokens{}).Bytes())
	expected = `patch {
	tags = merge({}, { 
  block_address = "resource.fake_resource.that"
  file_name = "main.tf"
})
}`
	assert.Equal(t, formatHcl(expected), formatHcl(actual))
}

func TestPatchWriteBlock(t *testing.T) {
	cases := []struct {
		desc         string
		dest         string
		patch        string
		expectedDest string
	}{
		{
			desc: "Same attribute in dest and patch",
			dest: `
block "example" {
	id = "untouched"
    attr = "old"
}`,
			patch: `
block "example" {
    attr = "new"
}`,
			expectedDest: `
block "example" {
	id = "untouched"
    attr = "new"
}`,
		},
		{
			desc: "Attribute in patch not in dest",
			dest: `
block "example" {
}`,
			patch: `
block "example" {
    attr = "new"
}`,
			expectedDest: `
block "example" {
    attr = "new"
}`,
		},
		{
			desc: "Same nested block in dest and patch",
			dest: `
block "example" {
    nested_block "nested" {
        attr = "old"
    }
}`,
			patch: `
block "example" {
    nested_block "nested" {
        attr = "new"
    }
}`,
			expectedDest: `
block "example" {
    nested_block "nested" {
        attr = "new"
    }
}`,
		},
		{
			desc: "Nested block in patch not in dest",
			dest: `
block "example" {
}`,
			patch: `
block "example" {
    nested_block "nested" {
        attr = "new"
    }
}`,
			expectedDest: `
block "example" {
    nested_block "nested" {
        attr = "new"
    }
}`,
		},
		{
			desc: "Nested block, multiple instances",
			dest: `
block "example" {
    nested_block {
        attr = "old"
    }
    nested_block {
		id = 123
        attr = "old"
    }
}`,
			patch: `
block "example" {
    nested_block {
        attr = "new"
    }
}`,
			expectedDest: `
block "example" {
    nested_block {
        attr = "new"
    }
    nested_block {
		id = 123
        attr = "new"
    }
}`,
		},
		{
			desc: "Nested block, multiple instances 2",
			dest: `
block "example" {
    nested_block {
        attr = "old"
    }
    nested_block {
		id = 123
    }
}`,
			patch: `
block "example" {
    nested_block {
        attr = "new"
    }
}`,
			expectedDest: `
block "example" {
    nested_block {
        attr = "new"
    }
    nested_block {
		id = 123
        attr = "new"
    }
}`,
		},
	}

	for _, c := range cases {
		t.Run(c.desc, func(t *testing.T) {
			readDst, diag := hclsyntax.ParseConfig([]byte(c.dest), "test.tf", hcl.InitialPos)
			require.Falsef(t, diag.HasErrors(), diag.Error())
			writeDst, diag := hclwrite.ParseConfig([]byte(c.dest), "test.tf", hcl.InitialPos)

			dstBlock := terraform.NewBlock(readDst.Body.(*hclsyntax.Body).Blocks[0], writeDst.Body().Blocks()[0])
			patchFile, diag := hclwrite.ParseConfig([]byte(c.patch), "patch.hcl", hcl.InitialPos)
			require.Falsef(t, diag.HasErrors(), diag.Error())
			sut := new(pkg.UpdateInPlaceTransform)
			sut.PatchWriteBlock(dstBlock, patchFile.Body().Blocks()[0])
			patched := string(dstBlock.WriteBlock.BuildTokens(hclwrite.Tokens{}).Bytes())
			assert.Equal(t, formatHcl(c.expectedDest), formatHcl(patched))
		})
	}
}

func newHclBlocks(t *testing.T, code string) []*golden.HclBlock {
	readFile, diag := hclsyntax.ParseConfig([]byte(code), "test.hcl", hcl.InitialPos)
	require.Falsef(t, diag.HasErrors(), diag.Error())
	writeFile, diag := hclwrite.ParseConfig([]byte(code), "test.hcl", hcl.InitialPos)
	require.Falsef(t, diag.HasErrors(), diag.Error())
	var r []*golden.HclBlock
	for i, rb := range readFile.Body.(*hclsyntax.Body).Blocks {
		r = append(r, golden.NewHclBlock(rb, writeFile.Body().Blocks()[i], nil))
	}
	return r
}

func formatHcl(inputHcl string) string {
	return strings.Trim(string(hclwrite.Format([]byte(inputHcl))), "\n")
}
