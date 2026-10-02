package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Azure/mapotf/pkg"
	"github.com/Azure/mapotf/pkg/backup"
	filesystem "github.com/Azure/mapotf/pkg/fs"
	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/prashantv/gostub"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTestFileCommandSelection(t *testing.T) {
	for _, tc := range []struct {
		command string
		flags   []string
		message string
	}{
		{"transform", []string{"--test-file", "tests/unit.tftest.hcl"}, ""},
		{"debug", []string{"--test-file", "tests/unit.tftest.hcl", "--eval", "{}"}, ""},
		{"reset", []string{"--test-file", "tests/unit.tftest.hcl"}, ""},
		{"clean-backup", []string{"--test-file", "tests/unit.tftest.hcl"}, ""},
		{"apply", []string{"--test-file", "tests/unit.tftest.hcl"}, "supported only"},
		{"test", []string{"--test-file", "tests/unit.tftest.hcl"}, "supported only"},
		{"transform", []string{"--test-file", ""}, "must not be empty"},
		{"transform", []string{"--test-file", "tests/unit.tftest.hcl", "--recursive"}, "cannot be combined"},
		{"transform", []string{"--test-file", "tests/unit.tftest.hcl", "-r"}, "cannot be combined"},
		{"apply", []string{"--eval", "{}"}, "supported only by debug"},
		{"debug", []string{"--eval", ""}, "must not be empty"},
	} {
		t.Run(tc.command+":"+tc.message, func(t *testing.T) {
			flags := &commonFlags{}
			stub := gostub.Stub(&cf, flags)
			defer stub.Reset()
			root := &cobra.Command{Use: "mapotf", PersistentPreRunE: validateSelectionFlags, SilenceErrors: true, SilenceUsage: true}
			root.PersistentFlags().StringVar(&flags.testFile, "test-file", "", "")
			root.PersistentFlags().StringVar(&flags.debugEval, "eval", "", "")
			ran := false
			command := &cobra.Command{Use: tc.command, RunE: func(_ *cobra.Command, _ []string) error {
				ran = true
				return nil
			}}
			command.Flags().BoolP("recursive", "r", false, "")
			root.AddCommand(command)
			root.SetArgs(append([]string{tc.command}, tc.flags...))
			err := root.Execute()
			if tc.message == "" {
				require.NoError(t, err)
				assert.True(t, ran)
			} else {
				require.ErrorContains(t, err, tc.message)
				assert.False(t, ran)
			}
		})
	}
}

func TestDebugEvalTestFileAndLocalDeclarations(t *testing.T) {
	t.Setenv("PATH", "")
	root := t.TempDir()
	child := filepath.Join(root, "modules", "child")
	tests := filepath.Join(root, "tests", "unit")
	rules := t.TempDir()
	require.NoError(t, os.MkdirAll(child, 0755))
	require.NoError(t, os.MkdirAll(tests, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "variables.tf"), []byte("variable \"root_input\" {}\n"), 0600))
	require.NoError(t, os.WriteFile(filepath.Join(child, "variables.tf"), []byte("variable \"location\" { type = string }\n"), 0600))
	filename := filepath.Join("tests", "unit", "case.tftest.hcl")
	source := "run \"root\" {}\nrun \"child\" {\n module { source = \"./modules/child\" }\n}\n"
	require.NoError(t, os.WriteFile(filepath.Join(root, filename), []byte(source), 0600))
	require.NoError(t, os.WriteFile(filepath.Join(rules, "main.mptf.hcl"), []byte(`
data "test_file" "this" {}
data "module_source" "targets" {
  for_each = { for name, target in data.test_file.this.result.run_modules : name => target if target.kind != "remote" }
  source = each.value.dir
}
transform "update_in_place" "not_applied" {
  target_block_address = "run.root"
  asraw {
    variables { location = "must not be written" }
  }
}
`), 0600))
	stub := gostub.Stub(&cf, &commonFlags{
		tfDir: root, testFile: filename,
		debugEval: `{ test = data.test_file.this.result, modules = { for name, target in data.module_source.targets : name => target.variables } }`,
	}).Stub(&os.Args, []string{"mapotf", "debug"})
	defer stub.Reset()
	var output bytes.Buffer
	command := &cobra.Command{}
	command.SetContext(context.Background())
	command.SetOut(&output)
	require.NoError(t, replFunc(&root, &rules)(command, nil))
	var result struct {
		Test struct {
			Variables any            `json:"variables"`
			Runs      map[string]any `json:"runs"`
		} `json:"test"`
		Modules map[string]map[string]any `json:"modules"`
	}
	require.NoError(t, json.Unmarshal(output.Bytes(), &result))
	assert.Nil(t, result.Test.Variables)
	assert.Len(t, result.Test.Runs, 2)
	assert.Contains(t, result.Modules["root"], "root_input")
	assert.NotContains(t, result.Modules["root"], "location")
	assert.Contains(t, result.Modules["child"], "location")
	assert.True(t, bytes.HasSuffix(output.Bytes(), []byte("\n")))
	unchanged, err := os.ReadFile(filepath.Join(root, filename))
	require.NoError(t, err)
	assert.Equal(t, source, string(unchanged))
	_, err = os.Stat(filepath.Join(root, filename) + backup.BackupExtension)
	assert.ErrorIs(t, err, os.ErrNotExist)

	for _, expression := range []string{"{", "data.test_file.missing", "1 / 0"} {
		cf.debugEval = expression
		output.Reset()
		require.Error(t, replFunc(&root, &rules)(command, nil))
		assert.Empty(t, output.String())
	}
}

func TestTestFileTransformBackupLifecycle(t *testing.T) {
	fs := afero.NewMemMapFs()
	root := "native-cli"
	filename := filepath.Join("tests", "unit", "case.tftest.hcl")
	path := filepath.Join(root, filename)
	other := filepath.Join(root, "tests", "unit", "other.tftest.hcl")
	source := "variables { authored = false }\nrun \"root\" {}\n"
	rules := "native-rules"
	require.NoError(t, afero.WriteFile(fs, path, []byte(source), 0644))
	require.NoError(t, afero.WriteFile(fs, other, []byte(source), 0644))
	require.NoError(t, afero.WriteFile(fs, other+backup.BackupExtension, []byte("other backup"), 0644))
	require.NoError(t, afero.WriteFile(fs, filepath.Join(root, "main.tf"), []byte("variable \"x\" {}\n"), 0644))
	require.NoError(t, afero.WriteFile(fs, filepath.Join(rules, "main.mptf.hcl"), []byte(`
data "test_file" "this" {}
transform "update_in_place" "global" {
  target_block_address = data.test_file.this.result.variables.mptf.block_address
  asraw { location = "eastus" }
}
`), 0644))
	stub := gostub.Stub(&cf, &commonFlags{tfDir: root, testFile: filename, mptfDirs: []string{rules}}).
		Stub(&filesystem.Fs, fs).
		Stub(&pkg.AbsDir, func(path string) (string, error) { return path, nil }).
		Stub(&os.Args, []string{"mapotf", "transform"})
	defer stub.Reset()
	_, err := transform(false, context.Background())
	require.NoError(t, err)
	content, err := afero.ReadFile(fs, path+backup.BackupExtension)
	require.NoError(t, err)
	assert.Equal(t, source, string(content))
	_, err = transform(false, context.Background())
	require.NoError(t, err)
	content, err = afero.ReadFile(fs, path+backup.BackupExtension)
	require.NoError(t, err)
	assert.Equal(t, source, string(content))
	require.NoError(t, reset())
	content, err = afero.ReadFile(fs, path)
	require.NoError(t, err)
	assert.Equal(t, source, string(content))
	_, err = transform(false, context.Background())
	require.NoError(t, err)
	require.NoError(t, cleanBackup())
	content, err = afero.ReadFile(fs, path)
	require.NoError(t, err)
	assert.Contains(t, string(content), `"eastus"`)
	exists, err := afero.Exists(fs, path+backup.BackupExtension)
	require.NoError(t, err)
	assert.False(t, exists)
	content, err = afero.ReadFile(fs, other+backup.BackupExtension)
	require.NoError(t, err)
	assert.Equal(t, "other backup", string(content))
	exists, err = afero.Exists(fs, filepath.Join(root, "main.tf")+backup.BackupExtension)
	require.NoError(t, err)
	assert.False(t, exists)
	_, err = transform(true, context.Background())
	require.ErrorContains(t, err, "cannot be combined")
}

func TestTestFileTerraformPlanOnly(t *testing.T) {
	terraformPath, err := exec.LookPath("terraform")
	if err != nil {
		t.Skip("Terraform is not available on PATH")
	}
	root := t.TempDir()
	rules := t.TempDir()
	filename := filepath.Join("tests", "unit", "case.tftest.hcl")
	moduleSource := `
variable "location" { type = string }
variable "enable_telemetry" { type = bool }
variable "hubs" { type = map(object({ location = string })) }
variable "valid" {
  type = bool
  default = true
  validation {
    condition = var.valid
    error_message = "Expected invalid input."
  }
}
output "location" { value = var.location }
output "hubs" { value = var.hubs }
output "opt_out" { value = !var.enable_telemetry }
`
	testSource := `
variables {
  enable_telemetry = false
  hubs = { primary = { location = "uksouth" } }
}
run "root" {
  command = plan
  assert {
    condition = output.location == "eastus" && output.hubs.primary.location == "uksouth" && output.opt_out
    error_message = "Root inputs changed."
  }
}
run "child" {
  command = plan
  module { source = "./modules/child" }
  variables {
    hubs = { child = { location = "northeurope" } }
  }
  assert {
    condition = output.location == "eastus" && output.hubs.child.location == "northeurope" && output.opt_out
    error_message = "Child inputs changed."
  }
}
run "explicit" {
  command = plan
  variables { location = "authored" }
  assert {
    condition = output.location == "authored"
    error_message = "Explicit input changed."
  }
}
run "negative" {
  command = plan
  variables { valid = false }
  expect_failures = [var.valid]
}
`
	for path, source := range map[string]string{
		filepath.Join(root, "main.tf"):                     moduleSource,
		filepath.Join(root, "modules", "child", "main.tf"): moduleSource,
		filepath.Join(root, filename):                      testSource,
		filepath.Join(rules, "main.mptf.hcl"): `
data "test_file" "this" {}
transform "update_in_place" "missing_input" {
  for_each = {
    for name, run in data.test_file.this.result.runs : name => run
    if !contains(keys(try(run.variables[0].mptf.attributes, {})), "location")
  }
  target_block_address = each.value.mptf.block_address
  asraw {
    variables { location = "eastus" }
  }
}
`,
	} {
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0755))
		require.NoError(t, os.WriteFile(path, hclwrite.Format([]byte(source)), 0600))
	}
	t.Setenv("TF_DATA_DIR", filepath.Join(root, ".terraform"))
	t.Setenv("TF_CLI_ARGS", "")
	t.Setenv("TF_CLI_ARGS_init", "")
	t.Setenv("TF_CLI_ARGS_test", "")
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	runTerraform := func(args ...string) ([]byte, error) {
		command := exec.CommandContext(ctx, terraformPath, args...)
		command.Dir = root
		for _, variable := range os.Environ() {
			if !strings.HasPrefix(variable, "TF_VAR_") {
				command.Env = append(command.Env, variable)
			}
		}
		return command.CombinedOutput()
	}
	testDirectory := "-test-directory=" + filepath.Join("tests", "unit")
	output, err := runTerraform("init", "-backend=false", "-input=false", "-no-color", testDirectory)
	require.NoError(t, err, string(output))
	output, err = runTerraform("test", "-no-color", testDirectory)
	require.Error(t, err, "the required input must be missing before transformation")
	assert.Contains(t, string(output), "No value for required variable")
	stub := gostub.Stub(&cf, &commonFlags{tfDir: root, testFile: filename, mptfDirs: []string{rules}}).
		Stub(&os.Args, []string{"mapotf", "transform"})
	defer stub.Reset()
	_, err = transform(false, ctx)
	require.NoError(t, err)
	output, err = runTerraform("test", "-no-color", testDirectory)
	require.NoError(t, err, string(output))
	assert.Contains(t, string(output), "4 passed, 0 failed")
}
