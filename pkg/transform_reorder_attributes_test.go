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
  source  = "./module"
  version = "1.0.0"

  custom = "x"

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
			desc: "nested_block_sorts_into_middle_alphabetically_with_blank_line",
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
  count = 1

  attr_a = 1
  attr_b = 2

  nested {
    inside = "yes"
  }

  depends_on = [other.thing]
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
		{
			desc: "head_tail_line_breaks_false_suppresses_section_blanks",
			mptf: `
transform "reorder_attributes" this {
  target_block_address     = "module.example"
  head_attributes          = ["source", "version"]
  tail_attributes          = ["depends_on"]
  head_tail_line_breaks    = false
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
			desc: "nested_block_listed_in_head_renders_first",
			mptf: `
transform "reorder_attributes" this {
  target_block_address = "resource.fake_resource.this"
  head_attributes      = ["lifecycle", "count"]
}
`,
			tfConfig: `
resource "fake_resource" this {
  attr_b = 2
  attr_a = 1
  count  = 1
  lifecycle {
    create_before_destroy = true
  }
}
`,
			expected: `
resource "fake_resource" this {
  lifecycle {
    create_before_destroy = true
  }
  count = 1

  attr_a = 1
  attr_b = 2
}
`,
		},
		{
			desc: "nested_block_listed_in_tail_renders_last",
			mptf: `
transform "reorder_attributes" this {
  target_block_address = "resource.fake_resource.this"
  tail_attributes      = ["lifecycle"]
}
`,
			tfConfig: `
resource "fake_resource" this {
  attr_b = 2
  attr_a = 1
  lifecycle {
    create_before_destroy = true
  }
}
`,
			expected: `
resource "fake_resource" this {
  attr_a = 1
  attr_b = 2

  lifecycle {
    create_before_destroy = true
  }
}
`,
		},
		{
			desc: "multiple_nested_blocks_each_get_leading_blank_line",
			mptf: `
transform "reorder_attributes" this {
  target_block_address = "resource.fake_resource.this"
}
`,
			tfConfig: `
resource "fake_resource" this {
  attr_b = 2
  attr_a = 1
  first {
    a = 1
  }
  second {
    b = 2
  }
}
`,
			expected: `
resource "fake_resource" this {
  attr_a = 1
  attr_b = 2

  first {
    a = 1
  }

  second {
    b = 2
  }
}
`,
		},
		{
			desc: "dynamic_block_addressable_by_label",
			mptf: `
transform "reorder_attributes" this {
  target_block_address = "resource.fake_resource.this"
  head_attributes      = ["count", "subnet"]
}
`,
			tfConfig: `
resource "fake_resource" this {
  attr_b = 2
  attr_a = 1
  count  = 1
  dynamic "subnet" {
    for_each = var.subnets
    content {
      name = subnet.value
    }
  }
}
`,
			expected: `
resource "fake_resource" this {
  count = 1

  dynamic "subnet" {
    for_each = var.subnets
    content {
      name = subnet.value
    }
  }

  attr_a = 1
  attr_b = 2
}
`,
		},
		{
			desc: "sort_middle_alphabetically_false_preserves_source_order",
			mptf: `
transform "reorder_attributes" this {
  target_block_address       = "resource.fake_resource.this"
  head_attributes            = ["count"]
  tail_attributes            = ["depends_on"]
  sort_middle_alphabetically = false
}
`,
			tfConfig: `
resource "fake_resource" this {
  depends_on = [other.thing]
  zeta       = 1
  alpha      = 2
  count      = 1
  middle {
    inside = "yes"
  }
}
`,
			expected: `
resource "fake_resource" this {
  count = 1

  zeta  = 1
  alpha = 2

  middle {
    inside = "yes"
  }

  depends_on = [other.thing]
}
`,
		},
		{
			desc: "sort_middle_alphabetically_false_still_inserts_nested_block_blank",
			mptf: `
transform "reorder_attributes" this {
  target_block_address       = "resource.fake_resource.this"
  sort_middle_alphabetically = false
}
`,
			tfConfig: `
resource "fake_resource" this {
  zeta = 1
  middle {
    inside = "yes"
  }
  alpha = 2
}
`,
			expected: `
resource "fake_resource" this {
  zeta = 1

  middle {
    inside = "yes"
  }
  alpha = 2
}
`,
		},
		{
			desc: "middle_only_no_head_no_tail_sorts_alphabetically_no_section_blanks",
			mptf: `
transform "reorder_attributes" this {
  target_block_address = "variable.example"
}
`,
			tfConfig: `
variable "example" {
  zeta  = 3
  alpha = 1
  mid   = 2
}
`,
			expected: `
variable "example" {
  alpha = 1
  mid   = 2
  zeta  = 3
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
