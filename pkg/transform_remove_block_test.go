package pkg_test

import (
	"context"
	"testing"
	
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
)

func TestRemoveBlock(t *testing.T) {
	cases := []struct {
		desc     string
		mptf     string
		tfConfig string
		expected string
		wantErr  bool
	}{
		{
			desc: "remove_resource",
			mptf: `
transform "remove_block" this {
  target_block_address = "resource.fake_resource.this"
}
`,
			tfConfig: `
resource "fake_resource" "this" {
  attr = "value"
}

resource "other_resource" "keep" {
  attr = "keep"
}
`,
			expected: `
resource "other_resource" "keep" {
  attr = "keep"
}
`,
		},
		{
			desc: "remove_data_block",
			mptf: `
transform "remove_block" this {
  target_block_address = "data.fake_data.this"
}
`,
			tfConfig: `
data "fake_data" "this" {
  attr = "value"
}

data "fake_data" "keep" {
  attr = "keep"
}
`,
			expected: `
data "fake_data" "keep" {
  attr = "keep"
}
`,
		},
		{
			desc: "remove_module_block",
			mptf: `
transform "remove_block" this {
  target_block_address = "module.test_module"
}
`,
			tfConfig: `
module "test_module" {
  source = "./modules/test"
  attr = "value"
}

module "keep_module" {
  source = "./modules/keep"
  attr = "keep"
}
`,
			expected: `
module "keep_module" {
  source = "./modules/keep"
  attr = "keep"
}
`,
		},
		{
			desc: "block_not_found",
			mptf: `
transform "remove_block" this {
  target_block_address = "resource.non_existent.block"
}
`,
			tfConfig: `
resource "fake_resource" "this" {
  attr = "value"
}
`,
			expected: `
resource "fake_resource" "this" {
  attr = "value"
}
`,
			wantErr: true,
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
			if c.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)

			after, err := afero.ReadFile(filesystem.Fs, "/main.tf")
			require.NoError(t, err)
			expected := formatHcl(c.expected)
			actual := formatHcl(string(after))
			assert.Equal(t, expected, actual)
		})
	}
}
