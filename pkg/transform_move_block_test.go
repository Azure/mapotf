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

func TestMoveBlock(t *testing.T) {
	cases := []struct {
		desc           string
		mptf           string
		initialFiles   map[string]string
		expectedFiles  map[string]string
		wantErr        bool
		errorSubstring string
	}{
		{
			desc: "move_resource_to_new_file",
			mptf: `
transform "move_block" test {
  target_block_address = "resource.fake_resource.this"
  file_name = "resources.tf"
}
`,
			initialFiles: map[string]string{
				"/main.tf": `
resource "fake_resource" "this" {
  attr = "value"
}

resource "other_resource" "keep" {
  attr = "keep"
}
`,
			},
			expectedFiles: map[string]string{
				"/main.tf": `
resource "other_resource" "keep" {
  attr = "keep"
}
`,
				"/resources.tf": `resource "fake_resource" "this" {
  attr = "value"
}

`,
			},
		},
		{
			desc: "move_to_existing_file",
			mptf: `
transform "move_block" test {
  target_block_address = "resource.fake_resource.this"
  file_name = "existing.tf"
}
`,
			initialFiles: map[string]string{
				"/main.tf": `
resource "fake_resource" "this" {
  attr = "value"
}

resource "other_resource" "keep" {
  attr = "keep"
}
`,
				"/existing.tf": `
resource "existing_resource" "foo" {
  name = "foo"
}
`,
			},
			expectedFiles: map[string]string{
				"/main.tf": `
resource "other_resource" "keep" {
  attr = "keep"
}
`,
				"/existing.tf": `
resource "existing_resource" "foo" {
  name = "foo"
}
resource "fake_resource" "this" {
  attr = "value"
}

`,
			},
		},
		{
			desc: "move_data_source",
			mptf: `
transform "move_block" test {
  target_block_address = "data.fake_data.this"
  file_name = "data.tf"
}
`,
			initialFiles: map[string]string{
				"/main.tf": `
data "fake_data" "this" {
  id = "123"
}

resource "fake_resource" "keep" {
  attr = "keep"
}
`,
			},
			expectedFiles: map[string]string{
				"/main.tf": `
resource "fake_resource" "keep" {
  attr = "keep"
}
`,
				"/data.tf": `data "fake_data" "this" {
  id = "123"
}

`,
			},
		},
		{
			desc: "move_module",
			mptf: `
transform "move_block" test {
  target_block_address = "module.test_module"
  file_name = "modules.tf"
}
`,
			initialFiles: map[string]string{
				"/main.tf": `
module "test_module" {
  source = "./modules/test"
  attr = "value"
}

module "keep_module" {
  source = "./modules/keep"
  attr = "keep"
}
`,
			},
			expectedFiles: map[string]string{
				"/main.tf": `
module "keep_module" {
  source = "./modules/keep"
  attr = "keep"
}
`,
				"/modules.tf": `module "test_module" {
  source = "./modules/test"
  attr = "value"
}

`,
			},
		},
		{
			desc: "block_not_found",
			mptf: `
transform "move_block" test {
  target_block_address = "resource.non_existent.block"
  file_name = "new.tf"
}
`,
			initialFiles: map[string]string{
				"/main.tf": `
resource "fake_resource" "this" {
  attr = "value"
}
`,
			},
			expectedFiles: map[string]string{
				"/main.tf": `
resource "fake_resource" "this" {
  attr = "value"
}
`,
			},
			wantErr:        true,
			errorSubstring: "cannot find block",
		},
		{
			desc: "move_to_same_file",
			mptf: `
transform "move_block" test {
  target_block_address = "resource.fake_resource.this"
  file_name = "main.tf"
}
`,
			initialFiles: map[string]string{
				"/main.tf": `
resource "fake_resource" "this" {
  attr = "value"
}
`,
			},
			expectedFiles: map[string]string{
				"/main.tf": `
resource "fake_resource" "this" {
  attr = "value"
}
`,
			},
			// No change expected since it's the same file
		},
	}

	for _, c := range cases {
		t.Run(c.desc, func(t *testing.T) {
			stub := gostub.Stub(&filesystem.Fs, fakeFs(c.initialFiles))
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
				if c.errorSubstring != "" {
					assert.Contains(t, err.Error(), c.errorSubstring)
				}
				return
			}
			require.NoError(t, err)

			// Check all expected files
			for path, expectedContent := range c.expectedFiles {
				actual, err := afero.ReadFile(filesystem.Fs, path)
				require.NoError(t, err)
				expected := formatHcl(expectedContent)
				actualFormatted := formatHcl(string(actual))
				assert.Equal(t, expected, actualFormatted)
			}
		})
	}
}
