package pkg_test

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Azure/golden"
	"github.com/Azure/mapotf/pkg"
	"github.com/Azure/mapotf/pkg/terraform"
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
	"github.com/zclconf/go-cty/cty"
)

func TestUpdateInPlaceTransform_MatchNestedBlockLabelsMockDefaults(t *testing.T) {
	const customized = `  mock_data "azapi_client_config" {
    # Keep the authored client configuration.
    defaults = {
      subscription_id = "old"
      tenant_id = var.tenant_id # Tenant expression.
      client_id = local.client_id
    }
  }`
	cases := []struct {
		name   string
		merge  string
		target string
		want   string
	}{
		{
			name:   "customized defaults",
			merge:  "merge_object_attributes = true",
			target: customized,
			want: `  mock_data "azapi_client_config" {
    # Keep the authored client configuration.
    defaults = {
      subscription_id = "new"
      tenant_id = var.tenant_id # Tenant expression.
      client_id = local.client_id
    }
  }`,
		},
		{
			name:  "partial defaults",
			merge: "merge_object_attributes = true",
			target: `  mock_data "azapi_client_config" {
    defaults = { tenant_id = var.tenant_id }
  }`,
			want: `  mock_data "azapi_client_config" {
    defaults = { tenant_id = var.tenant_id, subscription_id = "new" }
  }`,
		},
		{
			name:  "empty defaults",
			merge: "merge_object_attributes = true",
			target: `  mock_data "azapi_client_config" {
    defaults = {}
  }`,
			want: `  mock_data "azapi_client_config" {
    defaults = { subscription_id = "new" }
  }`,
		},
		{
			name:  "missing defaults",
			merge: "merge_object_attributes = true",
			target: `  mock_data "azapi_client_config" {
    override_during = plan
  }`,
			want: `  mock_data "azapi_client_config" {
    override_during = plan
    defaults = { subscription_id = "new" }
  }`,
		},
		{
			name:  "missing label",
			merge: "merge_object_attributes = true",
			want: `  mock_data "azapi_client_config" {
    defaults = { subscription_id = "new" }
  }`,
		},
		{
			name:   "object replacement by default",
			target: customized,
			want: `  mock_data "azapi_client_config" {
    # Keep the authored client configuration.
    defaults = { subscription_id = "new" }
  }`,
		},
		{
			name:   "object replacement explicitly disabled merge",
			merge:  "merge_object_attributes = false",
			target: customized,
			want: `  mock_data "azapi_client_config" {
    # Keep the authored client configuration.
    defaults = { subscription_id = "new" }
  }`,
		},
	}
	provider := func(target string) string {
		if target != "" {
			target += "\n"
		}
		return fmt.Sprintf(`mock_provider "azapi" {
  source = "./authored-mocks"
  # Preserve unrelated mocks and their expressions.
  mock_data "azapi_resource" {
    defaults = { output = jsonencode({ id = var.resource_id }) } # Resource comment.
  }
  mock_data "azapi_resource_action" {
    defaults = local.authored_defaults
  }
  mock_resource "azapi_client_config" {
    defaults = { id = "different block type" }
  }
%s}

run "plan" {
  command = plan
}
`, target)
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rules := fmt.Sprintf(`transform "update_in_place" "client_config" {
  target_block_address = "mock_provider.azapi"
  match_nested_block_labels = true
  %s
  asraw {
    mock_data "azapi_client_config" {
      defaults = { subscription_id = "new" }
    }
  }
}`, tc.merge)
			runLabelMatchTestFilePlan(t, provider(tc.target), rules, provider(tc.want))
		})
	}
}

func TestUpdateInPlaceTransform_MatchNestedBlockLabelsCompleteLabels(t *testing.T) {
	cases := []struct {
		name   string
		labels string
	}{
		{"multiple labels", ` "alpha" "beta"`},
		{"no labels", ""},
		{"empty label", ` ""`},
		{"label containing a separator", ` "alpha.beta"`},
	}
	for _, tc := range cases {
		for _, merge := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/merge=%t", tc.name, merge), func(t *testing.T) {
				var source, want strings.Builder
				source.WriteString("resource \"fake_resource\" this {\n")
				want.WriteString("resource \"fake_resource\" this {\n")
				for _, labels := range []string{
					"", ` ""`, ` "alpha"`, ` "alpha" "beta"`, ` "beta" "alpha"`,
					` "alpha" "beta" "gamma"`, ` "Alpha" "beta"`, ` "alpha" "Beta"`,
					` "alpha.beta"`, ` "alpha" "beta"`,
				} {
					fmt.Fprintf(&source, "  nested%s {\n    value = local.authored\n  }\n", labels)
					value := "local.authored"
					if labels == tc.labels {
						value = `"updated"`
					}
					fmt.Fprintf(&want, "  nested%s {\n    value = %s\n  }\n", labels, value)
				}
				for _, blockType := range []string{"other", "Nested"} {
					block := fmt.Sprintf("  %s%s {\n    value = local.other_type\n  }\n", blockType, tc.labels)
					source.WriteString(block)
					want.WriteString(block)
				}
				source.WriteString("}")
				want.WriteString("}")
				rules := fmt.Sprintf(`transform "update_in_place" this {
  target_block_address = "resource.fake_resource.this"
  match_nested_block_labels = true
  merge_object_attributes = %t
  asraw {
    nested%s {
      value = "updated"
    }
  }
}`, merge, tc.labels)
				runObjectMergePlan(t, source.String(), rules, want.String(), "")
			})
		}
	}
}

func TestUpdateInPlaceTransform_MatchNestedBlockLabelsRecursive(t *testing.T) {
	source := `resource "fake_resource" this {
  outer "keep" {
    inner "target" {
      value = local.keep
    }
  }
  outer "target" {
    inner "keep" {
      value = local.keep
    }
    inner "target" {
      leaf "first" "keep" {
        value = local.keep
      }
      leaf "first" "second" {
        value = "old"
        untouched = var.authored # Keep this expression.
      }
    }
  }
}`
	want := `resource "fake_resource" this {
  outer "keep" {
    inner "target" {
      value = local.keep
    }
  }
  outer "target" {
    inner "keep" {
      value = local.keep
    }
    inner "target" {
      leaf "first" "keep" {
        value = local.keep
      }
      leaf "first" "second" {
        value = "updated"
        untouched = var.authored # Keep this expression.
      }
      leaf "added" "second" {
        value = "added"
      }
    }
    inner "added" {
      leaf "new" "child" {
        value = "new"
      }
    }
  }
}`
	for _, merge := range []bool{false, true} {
		t.Run(fmt.Sprintf("merge=%t", merge), func(t *testing.T) {
			rules := fmt.Sprintf(`transform "update_in_place" this {
  target_block_address = "resource.fake_resource.this"
  match_nested_block_labels = true
  merge_object_attributes = %t
  asraw {
    outer "target" {
      inner "target" {
        leaf "first" "second" {
          value = "updated"
        }
        leaf "added" "second" {
          value = "added"
        }
      }
      inner "added" {
        leaf "new" "child" {
          value = "new"
        }
      }
    }
  }
}`, merge)
			runObjectMergePlan(t, source, rules, want, "")
		})
	}
}

func TestUpdateInPlaceTransform_MatchNestedBlockLabelsSequential(t *testing.T) {
	for _, merge := range []bool{false, true} {
		for name, source := range map[string]string{
			"empty provider": `mock_provider "azapi" {}`,
			"unrelated sibling": `mock_provider "azapi" {
  mock_data "azapi_resource" {
    defaults = local.authored
  }
}`,
		} {
			t.Run(fmt.Sprintf("%s/merge=%t", name, merge), func(t *testing.T) {
				rules := fmt.Sprintf(`transform "update_in_place" first {
  target_block_address = "mock_provider.azapi"
  match_nested_block_labels = true
  merge_object_attributes = %[1]t
  asraw {
    mock_data "azapi_client_config" {
      defaults = { tenant_id = var.tenant_id }
    }
  }
}
transform "update_in_place" second {
  depends_on = [transform.update_in_place.first]
  target_block_address = "mock_provider.azapi"
  match_nested_block_labels = true
  merge_object_attributes = %[1]t
  asraw {
    mock_data "azapi_client_config" {
      defaults = { subscription_id = "new" }
    }
  }
}`, merge)
				defaults := `{ subscription_id = "new" }`
				if merge {
					defaults = `{ tenant_id = var.tenant_id, subscription_id = "new" }`
				}
				want := strings.TrimSuffix(strings.TrimSuffix(source, "}"), "\n") + fmt.Sprintf(`
  mock_data "azapi_client_config" {
    defaults = %s
  }
}`, defaults)
				runLabelMatchTestFilePlan(t, source, rules, want)
			})
		}
	}
}

func TestUpdateInPlaceTransform_MatchNestedBlockLabelsPatchSources(t *testing.T) {
	for name, patch := range map[string]string{
		"asraw": `asraw {
    nested "target" {
      value = local.updated
    }
  }`,
		"asstring": `asstring {
    nested "target" {
      value = "local.updated"
    }
  }`,
		"dynamic body": `dynamic_block_body = <<-PATCH
nested "target" {
  value = local.updated
}
PATCH`,
		"multiple patch blocks": `asraw {
    nested "target" {
      value = local.first
    }
    nested "other" {
      value = local.other
    }
  }
  asstring {
    nested "target" {
      value = "local.updated"
    }
  }`,
	} {
		t.Run(name, func(t *testing.T) {
			want := `resource "fake_resource" this {
  nested "keep" {
    value = local.keep
  }
  nested "target" {
    value = local.updated
  }`
			if name == "multiple patch blocks" {
				want += `
  nested "other" {
    value = local.other
  }`
			}
			want += "\n}"
			runObjectMergePlan(t, `resource "fake_resource" this {
  nested "keep" {
    value = local.keep
  }
}`, fmt.Sprintf(`transform "update_in_place" this {
  target_block_address = "resource.fake_resource.this"
  match_nested_block_labels = 1 == 1
  %s
}`, patch), want, "")
		})
	}
}

func TestUpdateInPlaceTransform_MatchNestedBlockLabelsRunVariables(t *testing.T) {
	for _, flag := range []string{"", "match_nested_block_labels = false", "match_nested_block_labels = true"} {
		for name, variables := range map[string]string{
			"missing": "",
			"empty":   "  variables {}\n",
			"authored": `  variables {
    enable_telemetry = false
    options = { authored = var.authored } # Keep the input.
  }
`,
		} {
			t.Run(name+"/"+flag, func(t *testing.T) {
				code := func(body string) string {
					return "run \"case\" {\n  command = plan\n" + body + "}\n"
				}
				want := `  variables {
    options = { added = "default" }
  }
`
				if name == "authored" {
					want = `  variables {
    enable_telemetry = false
    options = { authored = var.authored, added = "default" } # Keep the input.
  }
`
				}
				rules := fmt.Sprintf(`transform "update_in_place" this {
  target_block_address = "run.case"
  merge_object_attributes = true
  %s
  asraw {
    variables {
      options = { added = "default" }
    }
  }
}`, flag)
				runLabelMatchTestFilePlan(t, code(variables), rules, code(want))
			})
		}
	}
}

func TestUpdateInPlaceTransform_MatchNestedBlockLabelsDefaultCompatibility(t *testing.T) {
	source := `resource "fake_resource" this {
  dynamic "nested" {
    for_each = var.values
    iterator = entry
    content {
      value = entry.value
      keep = local.authored
    }
  }
  nested "first" {
    value = local.first
  }
  nested "second" {
    value = local.second
  }
}`
	want := `resource "fake_resource" this {
  dynamic "nested" {
    for_each = var.values
    iterator = entry
    content {
      value = "updated"
      keep = local.authored
    }
  }
  nested "first" {
    value = "updated"
  }
  nested "second" {
    value = "updated"
  }
}`
	for _, flag := range []string{"", "match_nested_block_labels = false"} {
		for _, merge := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/merge=%t", flag, merge), func(t *testing.T) {
				rules := fmt.Sprintf(`transform "update_in_place" this {
  target_block_address = "resource.fake_resource.this"
  merge_object_attributes = %t
  %s
  asraw {
    nested "first" {
      value = "updated"
    }
  }
}`, merge, flag)
				runObjectMergePlan(t, source, rules, want, "")
			})
		}
	}
}

func TestUpdateInPlaceTransform_MatchNestedBlockLabelsDynamicBlocks(t *testing.T) {
	source := `resource "fake_resource" this {
  dynamic "nested" {
    for_each = var.values
    iterator = entry
    content {
      value = entry.value
    }
  }
}`
	for _, merge := range []bool{false, true} {
		for name, patch := range map[string]string{
			"static type does not match dynamic": `nested {
      value = "updated"
    }`,
			"dynamic matched literally": `dynamic "nested" {
      for_each = var.replacement
      content {
        value = "updated"
      }
    }`,
			"another dynamic label": `dynamic "other" {
      for_each = var.other
      content {
        value = "other"
      }
    }`,
		} {
			t.Run(fmt.Sprintf("%s/merge=%t", name, merge), func(t *testing.T) {
				want := strings.TrimSuffix(source, "}") + patch + "\n}"
				if name == "dynamic matched literally" {
					want = `resource "fake_resource" this {
  dynamic "nested" {
    for_each = var.replacement
    iterator = entry
    content {
      value = "updated"
    }
  }
}`
				}
				runObjectMergePlan(t, source, fmt.Sprintf(`transform "update_in_place" this {
  target_block_address = "resource.fake_resource.this"
  match_nested_block_labels = true
  merge_object_attributes = %t
  asraw {
    %s
  }
}`, merge, patch), want, "")
			})
		}
	}
}

func TestUpdateInPlaceTransform_MatchNestedBlockLabelsReflection(t *testing.T) {
	for _, flag := range []string{"", "match_nested_block_labels = false", "match_nested_block_labels = true", "match_nested_block_labels = 1 == 1"} {
		t.Run(flag, func(t *testing.T) {
			cfg, _, _, _ := nativeTestConfig(t, `mock_provider "azapi" {}`, fmt.Sprintf(`
transform "update_in_place" this {
  target_block_address = "mock_provider.azapi"
  %s
}`, flag))
			plan, err := pkg.RunMetaProgrammingTFPlan(cfg)
			require.NoError(t, err)
			want := flag == "match_nested_block_labels = true" || flag == "match_nested_block_labels = 1 == 1"
			transform := golden.Blocks[*pkg.UpdateInPlaceTransform](cfg)[0]
			require.Equal(t, want, transform.MatchNestedBlockLabels)
			value := cfg.EvalContext().Variables["transform"].GetAttr("update_in_place").GetAttr("this")
			require.Equal(t, cty.BoolVal(want), value.GetAttr("match_nested_block_labels"))
			if want {
				require.Contains(t, plan.String(), `"match_nested_block_labels":true`)
			} else {
				require.NotContains(t, plan.String(), "match_nested_block_labels")
			}
		})
	}
}

func TestUpdateInPlaceTransform_MatchNestedBlockLabelsCurrentWriteTree(t *testing.T) {
	for _, source := range []string{"block {}", `block {
  nested "target" {
    value = local.original
  }
}`} {
		for _, merge := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/merge=%t", source, merge), func(t *testing.T) {
				newTarget := func() *terraform.RootBlock {
					read, diag := hclsyntax.ParseConfig([]byte(source), "target.tf", hcl.InitialPos)
					require.False(t, diag.HasErrors(), diag.Error())
					write, diag := hclwrite.ParseConfig([]byte(source), "target.tf", hcl.InitialPos)
					require.False(t, diag.HasErrors(), diag.Error())
					return terraform.NewBlock(nil, read.Body.(*hclsyntax.Body).Blocks[0], write.Body().Blocks()[0])
				}
				first, diag := hclwrite.ParseConfig([]byte(`patch {
  nested "target" {
    value = local.first
    child "first" {
      value = local.first
    }
  }
}`), "first.hcl", hcl.InitialPos)
				require.False(t, diag.HasErrors(), diag.Error())
				second, diag := hclwrite.ParseConfig([]byte(`patch {
  nested "target" {
    value = local.second
    child "first" {
      value = local.second
    }
    child "second" {
      value = local.second
    }
  }
}`), "second.hcl", hcl.InitialPos)
				require.False(t, diag.HasErrors(), diag.Error())
				firstPatch := first.Body().Blocks()[0]
				patchBefore := string(firstPatch.BuildTokens(nil).Bytes())
				dest := newTarget()
				transform := &pkg.UpdateInPlaceTransform{MatchNestedBlockLabels: true, MergeObjectAttributes: merge}
				require.NoError(t, transform.PatchWriteBlock(dest, firstPatch))
				firstResult := string(dest.WriteBlock.BuildTokens(nil).Bytes())
				require.NoError(t, transform.PatchWriteBlock(dest, firstPatch))
				require.Equal(t, firstResult, string(dest.WriteBlock.BuildTokens(nil).Bytes()))
				require.NoError(t, transform.PatchWriteBlock(dest, second.Body().Blocks()[0]))
				secondResult := string(dest.WriteBlock.BuildTokens(nil).Bytes())
				require.Equal(t, formatHcl(`block {
  nested "target" {
    value = local.second
    child "first" {
      value = local.second
    }
    child "second" {
      value = local.second
    }
  }
}`), formatHcl(secondResult))
				require.Equal(t, patchBefore, string(firstPatch.BuildTokens(nil).Bytes()), "appended blocks must not share patch tokens")
				another := newTarget()
				require.NoError(t, transform.PatchWriteBlock(another, firstPatch))
				require.Equal(t, firstResult, string(another.WriteBlock.BuildTokens(nil).Bytes()))
				dest.WriteBody().RemoveBlock(dest.WriteBody().Blocks()[0])
				require.NoError(t, transform.PatchWriteBlock(dest, firstPatch))
				require.Equal(t, formatHcl(firstResult), formatHcl(string(dest.WriteBlock.BuildTokens(nil).Bytes())))
			})
		}
	}
}

func runLabelMatchTestFilePlan(t *testing.T, source, rules, want string) {
	t.Helper()
	cfg, fs, root, filename := nativeTestConfig(t, source, rules)
	var previous []byte
	for run := 0; run < 2; run++ {
		plan, err := pkg.RunMetaProgrammingTFPlan(cfg)
		require.NoError(t, err)
		require.NoError(t, plan.Apply())
		content, err := afero.ReadFile(fs, filepath.Join(root, filename))
		require.NoError(t, err)
		_, diag := hclsyntax.ParseConfig(content, filename, hcl.InitialPos)
		require.False(t, diag.HasErrors(), diag.Error())
		require.Equal(t, formatHcl(want), formatHcl(string(content)))
		if run > 0 {
			require.Equal(t, previous, content, "a second run must not change the output")
			return
		}
		previous = content
		blocks, err := pkg.LoadMPTFHclBlocks(false, "native-rules")
		require.NoError(t, err)
		cfg, err = pkg.NewMetaProgrammingTFConfig(&pkg.TerraformModuleRef{
			Dir: root, AbsDir: root, TestFile: filename,
		}, nil, blocks, nil, context.Background())
		require.NoError(t, err)
	}
}
