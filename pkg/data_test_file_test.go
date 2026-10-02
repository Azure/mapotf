package pkg_test

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Azure/golden"
	"github.com/Azure/mapotf/pkg"
	filesystem "github.com/Azure/mapotf/pkg/fs"
	"github.com/Azure/mapotf/pkg/terraform"
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/prashantv/gostub"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zclconf/go-cty/cty"
)

const nativeTestSource = `# File comment with a quoted brace: "}"
variables {
  enable_telemetry = false
  hubs = {
    primary = { location = "uksouth", note = "} # not a comment" }
  }
  template = <<-TEXT
    Authored { value }: ${var.description}
    # This is heredoc content, not a comment.
  TEXT
  mptf = "an ordinary input named mptf"
}

mock_provider "azapi" {}
mock_provider "modtm" {} # obsolete empty mock
mock_provider "random" {}
mock_provider "azurerm" {
  alias = "west"
  mock_resource "azurerm_resource_group" {
    defaults = { location = "westus", number = 123 }
  }
  mock_resource "azurerm_other" {
    defaults = { enabled = true, number = "authored" }
  }
}
provider "azapi" {
  alias = "real"
  use_cli = false
}

run "root" {
  command = plan
  variables { enable_telemetry = false }
  providers = { azapi = azapi.real }
  expect_failures = [var.invalid_input, output.invalid_output]
  assert {
    condition = var.hubs.primary.location == "uksouth"
    error_message = "Keep } and # and \"quoted\" text"
  }
}
run "child" {
  command = plan
  module { source = "./modules/hub" }
  variables {
    enable_telemetry = false
    hubs = { child = { location = "northeurope" } }
    reference = run.root.result
  }
  assert {
    condition = output.hubs != null
    error_message = <<-MESSAGE
      Authored } assertion ${var.description}
    MESSAGE
  }
}
run "explicit" {
  variables { location = "authored" }
}
run "explicit_null" {
  variables { location = null }
}
run "explicit_expression" {
  variables { location = var.location }
}
run "compact" { command = plan }
run "empty" {}
`

func nativeTestConfig(t *testing.T, source, rules string) (*pkg.MetaProgrammingTFConfig, afero.Fs, string, string) {
	t.Helper()
	root := "native-module"
	filename := filepath.Join("tests", "unit", "case.tftest.hcl")
	fs := fakeFs(map[string]string{
		filepath.Join(root, filename):                            source,
		filepath.Join(root, "main.tf"):                           "variable \"root_input\" {}\n",
		filepath.Join(root, "modules", "hub", "variables.tf"):    "variable \"child_input\" {}\n",
		filepath.Join(root, "tests", "unit", "other.tftest.hcl"): "mock_provider \"azapi\" {}\nrun \"root\" {}\n",
		filepath.Join("native-rules", "main.mptf.hcl"):           rules,
	})
	stub := gostub.Stub(&filesystem.Fs, fs)
	t.Cleanup(stub.Reset)
	blocks, err := pkg.LoadMPTFHclBlocks(false, "native-rules")
	require.NoError(t, err)
	cfg, err := pkg.NewMetaProgrammingTFConfig(&pkg.TerraformModuleRef{
		Dir: root, AbsDir: root, TestFile: filename,
	}, nil, blocks, nil, context.Background())
	require.NoError(t, err)
	return cfg, fs, root, filename
}

func TestNativeTestFileData(t *testing.T) {
	source := nativeTestSource + "\nrun \"remote\" {\n module { source = \"Azure/naming/azurerm\" }\n}\n"
	cfg, _, root, filename := nativeTestConfig(t, source, `data "test_file" "this" {}`)
	_, err := pkg.RunMetaProgrammingTFPlan(cfg)
	require.NoError(t, err)
	data := golden.Blocks[*pkg.TestFileData](cfg)[0].Result
	assert.Equal(t, filename, data.GetAttr("filename").AsString())
	assert.Equal(t, root, data.GetAttr("module_dir").AsString())
	variables := data.GetAttr("variables")
	assert.Equal(t, cty.False, variables.GetAttr("enable_telemetry"))
	assert.Equal(t, cty.StringVal("an ordinary input named mptf"), variables.GetAttr("mptf").GetAttr("attributes").GetAttr("mptf"))
	runs := data.GetAttr("runs")
	assert.True(t, runs.GetAttr("root").GetAttr("variables").Type().IsTupleType())
	assert.False(t, runs.GetAttr("empty").Type().HasAttribute("variables"))
	assert.Equal(t, "run.child", runs.GetAttr("child").GetAttr("mptf").GetAttr("block_address").AsString())
	assert.Equal(t, "[var.invalid_input, output.invalid_output]", runs.GetAttr("root").GetAttr("expect_failures").AsString())
	modules := data.GetAttr("run_modules")
	assert.Equal(t, "root", modules.GetAttr("root").GetAttr("kind").AsString())
	assert.True(t, modules.GetAttr("root").GetAttr("source").IsNull())
	assert.Equal(t, filepath.Join(root, "modules", "hub"), modules.GetAttr("child").GetAttr("dir").AsString())
	assert.Equal(t, "./modules/hub", modules.GetAttr("child").GetAttr("source").AsString())
	assert.Equal(t, "remote", modules.GetAttr("remote").GetAttr("kind").AsString())
	assert.True(t, modules.GetAttr("remote").GetAttr("dir").IsNull())
	mocks := data.GetAttr("mock_providers")
	assert.True(t, mocks.GetAttr("azapi").GetAttr("mptf").GetAttr("is_empty").True())
	custom := mocks.GetAttr("azurerm.west")
	assert.False(t, custom.GetAttr("mptf").GetAttr("is_empty").True())
	assert.Equal(t, "mock_provider.azurerm.west", custom.GetAttr("mptf").GetAttr("block_address").AsString())
	mockResources := custom.GetAttr("mock_resource")
	assert.Equal(t, "azurerm_resource_group", mockResources.Index(cty.NumberIntVal(0)).GetAttr("mptf").GetAttr("block_labels").Index(cty.NumberIntVal(0)).AsString())
	assert.True(t, cty.NumberIntVal(123).RawEquals(mockResources.Index(cty.NumberIntVal(0)).GetAttr("defaults").GetAttr("number")))
	assert.Equal(t, cty.StringVal("authored"), mockResources.Index(cty.NumberIntVal(1)).GetAttr("defaults").GetAttr("number"))
	real := data.GetAttr("providers").GetAttr("azapi.real")
	assert.Equal(t, cty.False, real.GetAttr("use_cli"))
	assert.Equal(t, "real", real.GetAttr("alias").AsString())
}

func TestNativeTestFileDataErrors(t *testing.T) {
	t.Run("normal module", func(t *testing.T) {
		stub := gostub.Stub(&filesystem.Fs, fakeFs(map[string]string{filepath.Join("module", "main.tf"): ""}))
		defer stub.Reset()
		cfg, err := pkg.NewMetaProgrammingTFConfig(&pkg.TerraformModuleRef{Dir: "module", AbsDir: "module"}, nil, nil, nil, context.Background())
		require.NoError(t, err)
		data := &pkg.TestFileData{BaseData: &pkg.BaseData{}, BaseBlock: golden.NewBaseBlock(cfg, nil)}
		require.ErrorContains(t, data.ExecuteDuringPlan(), "requires test-file mode")
	})
	for _, source := range []string{"./missing", "./main.tf"} {
		t.Run(source, func(t *testing.T) {
			cfg, _, _, _ := nativeTestConfig(t, fmt.Sprintf("run \"x\" {\n module { source = %q }\n}\n", source), `data "test_file" "this" {}`)
			_, err := pkg.RunMetaProgrammingTFPlan(cfg)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "local run module")
		})
	}
}

const nativeRunRules = `
data "test_file" "this" {}

transform "update_in_place" "missing_run_input" {
  for_each = {
    for name, run in data.test_file.this.result.runs : name => run
    if !contains(keys(try(run.variables[0].mptf.attributes, {})), "location") &&
       !contains(keys(try(data.test_file.this.result.variables.mptf.attributes, {})), "location")
  }
  target_block_address = each.value.mptf.block_address
  asraw {
    variables { location = "eastus" }
  }
}
transform "remove_block" "obsolete_empty_mock" {
  for_each = {
    for name, provider in data.test_file.this.result.mock_providers : name => provider
    if name == "modtm" && provider.mptf.is_empty
  }
  target_block_address = each.value.mptf.block_address
}
`

func TestNativeTestFilePartialRunUpdates(t *testing.T) {
	cfg, fs, root, filename := nativeTestConfig(t, nativeTestSource, nativeRunRules)
	before := nativeAttributeSnapshot(t, root, filename)
	other := filepath.Join(root, "tests", "unit", "other.tftest.hcl")
	otherContent, err := afero.ReadFile(fs, other)
	require.NoError(t, err)
	plan, err := pkg.RunMetaProgrammingTFPlan(cfg)
	require.NoError(t, err)
	require.NoError(t, plan.Apply())
	after := nativeAttributeSnapshot(t, root, filename)
	for _, run := range []string{"root", "child", "compact", "empty"} {
		key := "run." + run + "/variables[0]/location"
		require.Equal(t, `"eastus"`, after[key])
		delete(after, key)
	}
	assert.Equal(t, before, after)
	result, err := afero.ReadFile(fs, filepath.Join(root, filename))
	require.NoError(t, err)
	assert.Contains(t, string(result), `# File comment with a quoted brace: "}"`)
	assert.Contains(t, string(result), `# This is heredoc content, not a comment.`)
	assert.Contains(t, string(result), `Authored } assertion ${var.description}`)
	assert.NotContains(t, string(result), `mock_provider "modtm"`)
	unchanged, err := afero.ReadFile(fs, other)
	require.NoError(t, err)
	assert.Equal(t, otherContent, unchanged)
	unchanged, err = afero.ReadFile(fs, filepath.Join(root, "main.tf"))
	require.NoError(t, err)
	assert.Equal(t, "variable \"root_input\" {}\n", string(unchanged))
	assertNativeSecondPass(t, root, filename, result)
}

func TestNativeTestFileGlobalVariables(t *testing.T) {
	for _, global := range []string{
		"", "variables {}\n", "variables { keep = false }\n", "variables { /* } authored comment */ keep = \"{ quoted }\" }\n",
		"variables {\n # Keep the opt-out.\n enable_telemetry = false\n}\n",
		"variables { location = null }\n", "variables { location = var.authored }\n",
	} {
		t.Run(global, func(t *testing.T) {
			rules := `
data "test_file" "this" {}
transform "new_block" "global" {
  for_each = data.test_file.this.result.variables == null ? { create = true } : {}
  filename = data.test_file.this.result.filename
  new_block_type = "variables"
  asraw { location = "eastus" }
}
transform "update_in_place" "global" {
  for_each = data.test_file.this.result.variables != null && !contains(keys(try(data.test_file.this.result.variables.mptf.attributes, {})), "location") ? { update = true } : {}
  target_block_address = "variables"
  asraw { location = "eastus" }
}
`
			cfg, fs, root, filename := nativeTestConfig(t, global+"run \"root\" {}\n", rules)
			plan, err := pkg.RunMetaProgrammingTFPlan(cfg)
			require.NoError(t, err)
			data := golden.Blocks[*pkg.TestFileData](cfg)[0].Result
			assert.Equal(t, global == "", data.GetAttr("variables").IsNull())
			require.NoError(t, plan.Apply())
			module, err := terraform.LoadModule(terraform.ModuleRef{Dir: root, AbsDir: root, TestFile: filename})
			require.NoError(t, err)
			count := 0
			for _, block := range module.TestBlocks {
				if block.Type == "variables" {
					count++
					if strings.Contains(global, "location") {
						assert.Contains(t, global, block.Attributes["location"].String())
					} else {
						assert.Equal(t, `"eastus"`, block.Attributes["location"].String())
					}
					if strings.Contains(global, "keep") {
						assert.Contains(t, global, block.Attributes["keep"].String())
					}
				}
			}
			assert.Equal(t, 1, count)
			content, err := afero.ReadFile(fs, filepath.Join(root, filename))
			require.NoError(t, err)
			assertNativeSecondPass(t, root, filename, content)
		})
	}
}

func TestNativeTestFileAuthoredGlobalInput(t *testing.T) {
	for _, value := range []string{`"authored"`, "null", "var.location"} {
		t.Run(value, func(t *testing.T) {
			source := fmt.Sprintf("variables { location = %s }\nrun \"root\" {}\n", value)
			cfg, _, root, filename := nativeTestConfig(t, source, nativeRunRules)
			before := nativeAttributeSnapshot(t, root, filename)
			plan, err := pkg.RunMetaProgrammingTFPlan(cfg)
			require.NoError(t, err)
			require.Empty(t, plan.Transforms)
			require.NoError(t, plan.Apply())
			assert.Equal(t, before, nativeAttributeSnapshot(t, root, filename))
		})
	}
}

func TestNativeTestFilePreservesConfiguredMocks(t *testing.T) {
	for _, mock := range []string{
		`mock_provider "modtm" { alias = "authored" }`,
		`mock_provider "modtm" { source = "./custom-mocks" }`,
		"mock_provider \"modtm\" {\n mock_resource \"modtm_telemetry\" {\n defaults = { value = \"authored\" }\n }\n}",
	} {
		t.Run(mock, func(t *testing.T) {
			source := "variables { location = \"authored\" }\nrun \"root\" {}\n" + mock + "\n"
			cfg, _, root, filename := nativeTestConfig(t, source, nativeRunRules)
			before := nativeAttributeSnapshot(t, root, filename)
			plan, err := pkg.RunMetaProgrammingTFPlan(cfg)
			require.NoError(t, err)
			require.Empty(t, plan.Transforms)
			require.NoError(t, plan.Apply())
			assert.Equal(t, before, nativeAttributeSnapshot(t, root, filename))
		})
	}
}

func TestNativeTestFileTargetedAliasRemoval(t *testing.T) {
	source := `mock_provider "azapi" {}
mock_provider "azapi" { alias = "keep" }
mock_provider "azapi" { alias = "remove" }
provider "azapi" { alias = "real" }
`
	cfg, _, root, filename := nativeTestConfig(t, source, `
transform "remove_block" "alias" {
  target_block_address = "mock_provider.azapi.remove"
}
transform "remove_block" "same_alias_again" {
  target_block_address = "mock_provider.azapi.remove"
  depends_on = [transform.remove_block.alias]
}`)
	plan, err := pkg.RunMetaProgrammingTFPlan(cfg)
	require.NoError(t, err)
	require.NoError(t, plan.Apply())
	module, err := terraform.LoadModule(terraform.ModuleRef{Dir: root, AbsDir: root, TestFile: filename})
	require.NoError(t, err)
	var addresses []string
	for _, block := range module.TestBlocks {
		addresses = append(addresses, block.Address)
	}
	assert.Equal(t, []string{"mock_provider.azapi", "mock_provider.azapi.keep", "provider.azapi.real"}, addresses)
}

func TestNativeTestFileTransformBoundaries(t *testing.T) {
	for _, rule := range []string{
		`transform "new_block" "x" {
  filename = "other.tftest.hcl"
  new_block_type = "variables"
}`,
		`transform "move_block" "x" {
  file_name = "main.tf"
  target_block_address = "run.root"
}`,
		`transform "sort_blocks_in_file" "x" {
  file_name = "other.tftest.hcl"
  desired_order = ["run.root"]
}`,
		`transform "ensure_local" "x" {
  name = "x"
  fallback_file_name = "case.tftest.hcl"
  value_as_raw = true
}`,
		`transform "new_block" "x" {
  filename = data.test_file.this.result.filename
  new_block_type = "variables"
}`,
	} {
		t.Run(rule, func(t *testing.T) {
			source := "variables {}\nrun \"root\" {}\n"
			cfg, fs, root, filename := nativeTestConfig(t, source, "data \"test_file\" \"this\" {}\n"+rule)
			plan, err := pkg.RunMetaProgrammingTFPlan(cfg)
			if err == nil {
				err = plan.Apply()
			}
			require.Error(t, err)
			content, err := afero.ReadFile(fs, filepath.Join(root, filename))
			require.NoError(t, err)
			assert.Equal(t, source, string(content))
		})
	}
}

func assertNativeSecondPass(t *testing.T, root, filename string, expected []byte) {
	t.Helper()
	blocks, err := pkg.LoadMPTFHclBlocks(false, "native-rules")
	require.NoError(t, err)
	cfg, err := pkg.NewMetaProgrammingTFConfig(&pkg.TerraformModuleRef{Dir: root, AbsDir: root, TestFile: filename}, nil, blocks, nil, context.Background())
	require.NoError(t, err)
	plan, err := pkg.RunMetaProgrammingTFPlan(cfg)
	require.NoError(t, err)
	require.Empty(t, plan.Transforms)
	require.NoError(t, plan.Apply())
	content, err := afero.ReadFile(filesystem.Fs, filepath.Join(root, filename))
	require.NoError(t, err)
	assert.Equal(t, expected, content)
}

func nativeAttributeSnapshot(t *testing.T, root, filename string) map[string]string {
	t.Helper()
	module, err := terraform.LoadModule(terraform.ModuleRef{Dir: root, AbsDir: root, TestFile: filename})
	require.NoError(t, err)
	snapshot := make(map[string]string)
	var capture func(string, terraform.Block)
	capture = func(prefix string, block terraform.Block) {
		for name, attribute := range block.GetAttributes() {
			tokens, diag := hclsyntax.LexExpression([]byte(attribute.String()), "", hcl.InitialPos)
			require.False(t, diag.HasErrors(), diag.Error())
			var expression strings.Builder
			for _, token := range tokens {
				if token.Type != hclsyntax.TokenNewline && token.Type != hclsyntax.TokenEOF {
					expression.Write(token.Bytes)
				}
			}
			snapshot[prefix+"/"+name] = expression.String()
		}
		for kind, nested := range block.GetNestedBlocks() {
			for i, child := range nested {
				capture(fmt.Sprintf("%s/%s[%d]", prefix, kind, i), child)
			}
		}
	}
	for _, block := range module.TestBlocks {
		capture(block.Address, block)
	}
	return snapshot
}
