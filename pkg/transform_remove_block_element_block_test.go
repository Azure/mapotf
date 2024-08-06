package pkg_test

import (
	"context"
	"github.com/Azure/golden"
	"github.com/Azure/mapotf/pkg"
	filesystem "github.com/Azure/mapotf/pkg/fs"
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/prashantv/gostub"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestRemoveBlockContent(t *testing.T) {
	cases := []struct {
		desc     string
		mptf     string
		tfConfig string
		expected string
	}{
		{
			desc: "singlePath",
			mptf: `
transform "remove_block_element" this {
  target_block_address = "resource.fake_resource.this"
  paths = ["nested_block"]
}
`,
			tfConfig: `
resource "fake_resource" this {
  nested_block {}
  non_target_block {}
}
`,
			expected: `
resource "fake_resource" this {
  non_target_block {}
}
`,
		},
		{
			desc: "multiplePath",
			mptf: `
transform "remove_block_element" this {
  target_block_address = "resource.fake_resource.this"
  paths = ["nested_block", "nested_block2"]
}
`,
			tfConfig: `
resource "fake_resource" this {
  nested_block {}
  nested_block2 {}
  non_target_block {}
}
`,
			expected: `
resource "fake_resource" this {
  non_target_block {}
}
`,
		},
		{
			desc: "deepNestedPath",
			mptf: `
transform "remove_block_element" this {
  target_block_address = "resource.fake_resource.this"
  paths = ["nested_block.second_nested_block"]
}
`,
			tfConfig: `
resource "fake_resource" this {
  nested_block {
    non_target_block {}
  }
  nested_block {
    second_nested_block {
    }
  }
  non_target_block {}
}
`,
			expected: `
resource "fake_resource" this {
  nested_block {
    non_target_block {}
  }
  nested_block {
  }
  non_target_block {}
}
`,
		},
		{
			desc: "removeAttribute",
			mptf: `
transform "remove_block_element" this {
  target_block_address = "resource.fake_resource.this"
  paths = ["attr"]
}
`,
			tfConfig: `
resource "fake_resource" this {
  attr = 1
  nested_block {
    attr = "hello"
  }
}
`,
			expected: `
resource "fake_resource" this {
  nested_block {
    attr = "hello"
  }
}
`,
		},
		{
			desc: "removeAttributeInStaticNestedBlock",
			mptf: `
transform "remove_block_element" this {
  target_block_address = "resource.fake_resource.this"
  paths = ["nested_block.attr"]
}
`,
			tfConfig: `
resource "fake_resource" this {
  attr = 1
  nested_block {
    attr = "hello"
  }
}
`,
			expected: `
resource "fake_resource" this {
  attr = 1
  nested_block {
  }
}
`,
		},
		{
			desc: "removeAttributeInDynamicNestedBlock",
			mptf: `
transform "remove_block_element" this {
  target_block_address = "resource.fake_resource.this"
  paths = ["nested_block.attr"]
}
`,
			tfConfig: `
resource "fake_resource" this {
  attr = 1
  dynamic "nested_block" {
    for_each = [1]
    content {
      attr = "hello"
    }
  }
}
`,
			expected: `
resource "fake_resource" this {
  attr = 1
  dynamic "nested_block" {
    for_each = [1]
    content {
    }
  }
}
`,
		},
	}
	for _, c := range cases {
		t.Run(c.desc, func(t *testing.T) {
			stub := gostub.Stub(&filesystem.Fs, fakeFs(map[string]string{
				"/main.tf": c.tfConfig,
			}))
			defer stub.Reset()

			readFile, diag := hclsyntax.ParseConfig([]byte(c.mptf), "test.hcl", hcl.InitialPos)
			require.Falsef(t, diag.HasErrors(), diag.Error())
			writeFile, diag := hclwrite.ParseConfig([]byte(c.mptf), "test.hcl", hcl.InitialPos)
			require.Falsef(t, diag.HasErrors(), diag.Error())
			hclBlock := golden.NewHclBlock(readFile.Body.(*hclsyntax.Body).Blocks[0], writeFile.Body().Blocks()[0], nil)
			cfg, err := pkg.NewMetaProgrammingTFConfig(&pkg.TerraformModuleRef{
				Dir:    "/",
				AbsDir: "/",
			}, nil, []*golden.HclBlock{hclBlock}, nil, context.TODO())
			require.NoError(t, err)
			plan, err := pkg.RunMetaProgrammingTFPlan(cfg)
			require.NoError(t, err)
			err = plan.Apply()
			require.NoError(t, err)
			after, err := afero.ReadFile(filesystem.Fs, "/main.tf")
			require.NoError(t, err)
			expected := formatHcl(c.expected)
			actual := formatHcl(string(after))
			assert.Equal(t, expected, actual)
		})
	}
}

func TestRemoveNestedBlock_mergeAfterRemove(t *testing.T) {
	mptfCfg := `
transform "remove_block_element" this {
  target_block_address = "resource.fake_resource.this"
  paths = ["identity"]
}

transform "update_in_place" this {
  target_block_address = "resource.fake_resource.this"
  asraw {
    dynamic "identity" {
	  for_each = var.enabled ? [1] : []
      content {
		type = "SystemAssigned"
      }
    }
  }
  depends_on = [transform.remove_block_element.this]
}
`
	stub := gostub.Stub(&filesystem.Fs, fakeFs(map[string]string{
		"/main.tf": `
resource "fake_resource" this {
  identity {}
}
`,
		"/cfg/main.mptf.hcl": mptfCfg,
	}))
	defer stub.Reset()

	hclBlocks, err := pkg.LoadMPTFHclBlocks(false, "/cfg")
	require.NoError(t, err)
	cfg, err := pkg.NewMetaProgrammingTFConfig(&pkg.TerraformModuleRef{
		Dir:    "/",
		AbsDir: "/",
	}, nil, hclBlocks, nil, context.TODO())
	require.NoError(t, err)
	plan, err := pkg.RunMetaProgrammingTFPlan(cfg)
	require.NoError(t, err)
	err = plan.Apply()
	require.NoError(t, err)
	after, err := afero.ReadFile(filesystem.Fs, "/main.tf")
	require.NoError(t, err)
	expected := formatHcl(`
resource "fake_resource" this {
  dynamic "identity" {
	for_each = var.enabled ? [1] : []
    content {
      type = "SystemAssigned"
    }
  }
}
`)
	actual := formatHcl(string(after))
	assert.Equal(t, expected, actual)
}
