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

func TestReorderAttributes(t *testing.T) {
	cases := []struct {
		desc           string
		mptf           string
		tfConfig       string
		expected       string
		wantErr        bool
		errorSubstring string
	}{
		{
			desc: "head_only_reorders_named_attrs_first",
			mptf: `
transform "reorder_attributes" this {
  target_block_address = "variable.example"
  head_attributes      = ["type", "default", "description"]
}
`,
			tfConfig: `
variable "example" {
  description = "An example variable."
  default     = "value"
  type        = string
}
`,
			expected: `
variable "example" {
  type        = string
  default     = "value"
  description = "An example variable."
}
`,
		},
		{
			desc: "head_and_tail_reorder",
			mptf: `
transform "reorder_attributes" this {
  target_block_address = "module.example"
  head_attributes      = ["source", "version"]
  tail_attributes      = ["depends_on"]
}
`,
			tfConfig: `
module "example" {
  depends_on = [null_resource.this]
  custom     = "x"
  version    = "1.0.0"
  source     = "./module"
}
`,
			expected: `
module "example" {
  source     = "./module"
  version    = "1.0.0"
  custom     = "x"
  depends_on = [null_resource.this]
}
`,
		},
		{
			desc: "missing_attrs_in_lists_are_silently_skipped",
			mptf: `
transform "reorder_attributes" this {
  target_block_address = "variable.example"
  head_attributes      = ["type", "default", "description", "nullable", "sensitive"]
}
`,
			tfConfig: `
variable "example" {
  description = "x"
  type        = string
}
`,
			expected: `
variable "example" {
  type        = string
  description = "x"
}
`,
		},
		{
			desc: "head_tail_overlap_returns_error",
			mptf: `
transform "reorder_attributes" this {
  target_block_address = "variable.example"
  head_attributes      = ["type"]
  tail_attributes      = ["type"]
}
`,
			tfConfig: `
variable "example" {
  type = string
}
`,
			expected:       ``,
			wantErr:        true,
			errorSubstring: "cannot be in both head_attributes and tail_attributes",
		},
		{
			desc: "nested_blocks_preserved_after_attributes",
			mptf: `
transform "reorder_attributes" this {
  target_block_address = "resource.fake_resource.this"
  head_attributes      = ["count", "for_each"]
  tail_attributes      = ["depends_on"]
}
`,
			tfConfig: `
resource "fake_resource" this {
  depends_on = [other.thing]
  attr_b     = 2
  attr_a     = 1
  count      = 1
  nested {
    inside = "yes"
  }
}
`,
			expected: `
resource "fake_resource" this {
  count      = 1
  attr_b     = 2
  attr_a     = 1
  depends_on = [other.thing]

  nested {
    inside = "yes"
  }
}
`,
		},
		{
			desc: "no_op_when_already_in_order",
			mptf: `
transform "reorder_attributes" this {
  target_block_address = "variable.example"
  head_attributes      = ["type", "default"]
}
`,
			tfConfig: `
variable "example" {
  type    = string
  default = "v"
}
`,
			expected: `
variable "example" {
  type    = string
  default = "v"
}
`,
		},
		{
			desc: "missing_target_block_returns_error",
			mptf: `
transform "reorder_attributes" this {
  target_block_address = "variable.does_not_exist"
  head_attributes      = ["type"]
}
`,
			tfConfig: `
variable "other" {
  type = string
}
`,
			expected:       ``,
			wantErr:        true,
			errorSubstring: "cannot find block",
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
				if c.errorSubstring != "" {
					assert.Contains(t, err.Error(), c.errorSubstring)
				}
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
