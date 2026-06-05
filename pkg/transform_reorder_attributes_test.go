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
			desc: "head_and_foot_reorder",
			mptf: `
transform "reorder_attributes" this {
  target_block_address = "module.example"
  head_attributes      = ["source", "version"]
  foot_attributes      = ["depends_on"]
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
			desc: "head_foot_overlap_returns_error",
			mptf: `
transform "reorder_attributes" this {
  target_block_address = "variable.example"
  head_attributes      = ["type"]
  foot_attributes      = ["type"]
}
`,
			tfConfig: `
variable "example" {
  type = string
}
`,
			expected:       ``,
			wantErr:        true,
			errorSubstring: "cannot be in both head_attributes and foot_attributes",
		},
		{
			desc: "nested_block_sorts_into_body_alphabetically_with_blank_line",
			mptf: `
transform "reorder_attributes" this {
  target_block_address = "resource.fake_resource.this"
  head_attributes      = ["count", "for_each"]
  foot_attributes      = ["depends_on"]
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
			desc: "head_foot_line_breaks_false_suppresses_section_blanks",
			mptf: `
transform "reorder_attributes" this {
  target_block_address     = "module.example"
  head_attributes          = ["source", "version"]
  foot_attributes          = ["depends_on"]
  head_foot_line_breaks    = false
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
			desc: "nested_block_listed_in_foot_renders_last",
			mptf: `
transform "reorder_attributes" this {
  target_block_address = "resource.fake_resource.this"
  foot_attributes      = ["lifecycle"]
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
			desc: "sort_body_alphabetically_false_preserves_source_order",
			mptf: `
transform "reorder_attributes" this {
  target_block_address       = "resource.fake_resource.this"
  head_attributes            = ["count"]
  foot_attributes            = ["depends_on"]
  sort_body_alphabetically = false
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
			desc: "sort_body_alphabetically_false_still_inserts_nested_block_blank",
			mptf: `
transform "reorder_attributes" this {
  target_block_address       = "resource.fake_resource.this"
  sort_body_alphabetically = false
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
			desc: "body_only_no_head_no_foot_sorts_alphabetically_no_section_blanks",
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
		{
			desc: "adjacent_same_name_nested_blocks_stay_adjacent",
			mptf: `
transform "reorder_attributes" this {
  target_block_address = "variable.example"
  head_attributes      = ["type", "description"]
}
`,
			tfConfig: `
variable "example" {
  description = "An example."
  type        = string
  validation {
    condition     = length(var.example) > 0
    error_message = "Must not be empty."
  }
  validation {
    condition     = length(var.example) < 10
    error_message = "Must be short."
  }
}
`,
			expected: `
variable "example" {
  type        = string
  description = "An example."

  validation {
    condition     = length(var.example) > 0
    error_message = "Must not be empty."
  }
  validation {
    condition     = length(var.example) < 10
    error_message = "Must be short."
  }
}
`,
		},
		{
			desc: "three_adjacent_same_name_nested_blocks_all_stay_adjacent",
			mptf: `
transform "reorder_attributes" this {
  target_block_address = "variable.example"
  head_attributes      = ["type"]
}
`,
			tfConfig: `
variable "example" {
  type = string
  validation {
    condition     = true
    error_message = "first"
  }
  validation {
    condition     = true
    error_message = "second"
  }
  validation {
    condition     = true
    error_message = "third"
  }
}
`,
			expected: `
variable "example" {
  type = string

  validation {
    condition     = true
    error_message = "first"
  }
  validation {
    condition     = true
    error_message = "second"
  }
  validation {
    condition     = true
    error_message = "third"
  }
}
`,
		},
		{
			desc: "adjacent_same_label_dynamic_blocks_stay_adjacent",
			mptf: `
transform "reorder_attributes" this {
  target_block_address = "resource.fake_resource.this"
  head_attributes      = ["count"]
}
`,
			tfConfig: `
resource "fake_resource" this {
  count = 1
  dynamic "subnet" {
    for_each = var.subnets_a
    content {
      name = subnet.value
    }
  }
  dynamic "subnet" {
    for_each = var.subnets_b
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
    for_each = var.subnets_a
    content {
      name = subnet.value
    }
  }
  dynamic "subnet" {
    for_each = var.subnets_b
    content {
      name = subnet.value
    }
  }
}
`,
		},
		{
			desc: "different_label_dynamic_blocks_still_get_blank_between",
			mptf: `
transform "reorder_attributes" this {
  target_block_address = "resource.fake_resource.this"
  head_attributes      = ["count"]
}
`,
			tfConfig: `
resource "fake_resource" this {
  count = 1
  dynamic "subnet" {
    for_each = var.subnets
    content {
      name = subnet.value
    }
  }
  dynamic "route" {
    for_each = var.routes
    content {
      cidr = route.value
    }
  }
}
`,
			expected: `
resource "fake_resource" this {
  count = 1

  dynamic "route" {
    for_each = var.routes
    content {
      cidr = route.value
    }
  }

  dynamic "subnet" {
    for_each = var.subnets
    content {
      name = subnet.value
    }
  }
}
`,
		},
		{
			desc: "same_name_nested_with_attr_between_does_not_group",
			mptf: `
transform "reorder_attributes" this {
  target_block_address       = "variable.example"
  head_attributes            = ["type"]
  sort_body_alphabetically = false
}
`,
			tfConfig: `
variable "example" {
  type = string
  validation {
    condition     = true
    error_message = "first"
  }
  some_attr = "x"
  validation {
    condition     = true
    error_message = "second"
  }
}
`,
			expected: `
variable "example" {
  type = string

  validation {
    condition     = true
    error_message = "first"
  }
  some_attr = "x"

  validation {
    condition     = true
    error_message = "second"
  }
}
`,
		},
		{
			desc: "section_boundary_at_same_name_nested_still_inserts_blank",
			mptf: `
transform "reorder_attributes" this {
  target_block_address = "variable.example"
  head_attributes      = ["type", "validation"]
  foot_attributes      = ["depends_on"]
}
`,
			tfConfig: `
variable "example" {
  description = "An example."
  type        = string
  depends_on  = ["other"]
  validation {
    condition     = true
    error_message = "first"
  }
  validation {
    condition     = true
    error_message = "second"
  }
}
`,
			expected: `
variable "example" {
  type = string

  validation {
    condition     = true
    error_message = "first"
  }
  validation {
    condition     = true
    error_message = "second"
  }

  description = "An example."

  depends_on = ["other"]
}
`,
		},
		{
			desc: "body_attributes_listed_first_unlisted_sorted_alphabetically",
			mptf: `
transform "reorder_attributes" this {
  target_block_address = "resource.fake_resource.this"
  body_attributes      = ["name", "location"]
}
`,
			tfConfig: `
resource "fake_resource" this {
  tags     = {}
  location = "westus"
  name     = "rg"
  sku      = "Standard"
}
`,
			expected: `
resource "fake_resource" this {
  name     = "rg"
  location = "westus"
  sku      = "Standard"
  tags     = {}
}
`,
		},
		{
			desc: "body_attributes_with_sort_body_false_keeps_unlisted_in_source_order",
			mptf: `
transform "reorder_attributes" this {
  target_block_address     = "resource.fake_resource.this"
  body_attributes          = ["name", "location"]
  sort_body_alphabetically = false
}
`,
			tfConfig: `
resource "fake_resource" this {
  tags     = {}
  location = "westus"
  name     = "rg"
  sku      = "Standard"
}
`,
			expected: `
resource "fake_resource" this {
  name     = "rg"
  location = "westus"
  tags     = {}
  sku      = "Standard"
}
`,
		},
		{
			desc: "body_attributes_with_head_and_foot_three_section_flow",
			mptf: `
transform "reorder_attributes" this {
  target_block_address = "resource.fake_resource.this"
  head_attributes      = ["for_each", "count", "provider"]
  body_attributes      = ["name", "location"]
  foot_attributes      = ["lifecycle", "depends_on"]
}
`,
			tfConfig: `
resource "fake_resource" this {
  depends_on = [other.thing]
  tags       = {}
  location   = "westus"
  name       = "rg"
  sku        = "Standard"
  count      = 1
  lifecycle {
    create_before_destroy = true
  }
}
`,
			expected: `
resource "fake_resource" this {
  count = 1

  name     = "rg"
  location = "westus"
  sku      = "Standard"
  tags     = {}

  lifecycle {
    create_before_destroy = true
  }
  depends_on = [other.thing]
}
`,
		},
		{
			desc: "body_attributes_listed_nested_block_placed_before_unlisted",
			mptf: `
transform "reorder_attributes" this {
  target_block_address = "variable.example"
  head_attributes      = ["type"]
  body_attributes      = ["validation"]
}
`,
			tfConfig: `
variable "example" {
  description = "x"
  type        = string
  validation {
    condition     = true
    error_message = "first"
  }
  default = "v"
}
`,
			expected: `
variable "example" {
  type = string

  validation {
    condition     = true
    error_message = "first"
  }
  default     = "v"
  description = "x"
}
`,
		},
		{
			desc: "body_attributes_missing_names_silently_skipped",
			mptf: `
transform "reorder_attributes" this {
  target_block_address = "variable.example"
  body_attributes      = ["type", "default", "description", "nullable", "sensitive"]
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
			desc: "body_head_overlap_returns_error",
			mptf: `
transform "reorder_attributes" this {
  target_block_address = "variable.example"
  head_attributes      = ["type"]
  body_attributes      = ["type"]
}
`,
			tfConfig: `
variable "example" {
  type = string
}
`,
			expected:       ``,
			wantErr:        true,
			errorSubstring: "cannot be in both head_attributes and body_attributes",
		},
		{
			desc: "body_foot_overlap_returns_error",
			mptf: `
transform "reorder_attributes" this {
  target_block_address = "variable.example"
  body_attributes      = ["type"]
  foot_attributes      = ["type"]
}
`,
			tfConfig: `
variable "example" {
  type = string
}
`,
			expected:       ``,
			wantErr:        true,
			errorSubstring: "cannot be in both body_attributes and foot_attributes",
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
