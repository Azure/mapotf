package terraform

import (
	"path/filepath"
	"testing"

	filesystem "github.com/Azure/mapotf/pkg/fs"
	"github.com/hashicorp/hcl/v2"
	"github.com/prashantv/gostub"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zclconf/go-cty/cty"
)

func TestLoadModuleShouldLoadAllTerraformFiles(t *testing.T) {
	mockFs := afero.NewMemMapFs()
	stub := gostub.Stub(&filesystem.Fs, mockFs)
	defer stub.Reset()
	_ = afero.WriteFile(mockFs, "/main.tf", []byte(`resource "fake_resource" this {}
data "fake_data" this {}
`), 0644)
	_ = afero.WriteFile(mockFs, "/main2.tf", []byte(`resource "fake_resource" that {}
data "fake_data" that {}
`), 0644)
	sut, err := LoadModule(ModuleRef{
		Dir:    ".",
		AbsDir: "/",
	})
	require.NoError(t, err)
	assert.Len(t, sut.ResourceBlocks, 2)
	assert.Len(t, sut.DataBlocks, 2)
}

func TestModule_SaveToDisk(t *testing.T) {
	// Create a mock file system
	mockFs := afero.NewMemMapFs()
	stub := gostub.Stub(&filesystem.Fs, mockFs)
	defer stub.Reset()

	// Compose a valid Terraform config file
	originalContent := `resource "fake_resource" "this" {
}`
	filename := filepath.Join("tmp", "main.tf")
	_ = afero.WriteFile(mockFs, filename, []byte(originalContent), 0644)

	// Load the config file into a Module
	m, err := LoadModule(ModuleRef{
		Dir:    "tmp",
		AbsDir: "tmp",
	})
	require.NoError(t, err)

	// Do some modification on the resource block's hclwrite.Block
	for _, rb := range m.ResourceBlocks {
		rb.WriteBlock.Body().SetAttributeValue("new_attribute", cty.StringVal("new_value"))
	}
	// Save the changes back to disk
	err = m.SaveToDisk()
	require.NoError(t, err)
	// Verify that the file's content in the file system has been changed correctly
	modifiedContent, err := afero.ReadFile(mockFs, filename)
	require.NoError(t, err)
	expectedContent := `resource "fake_resource" "this" {
  new_attribute = "new_value"
}`
	assert.Equal(t, expectedContent, string(modifiedContent))
}

func TestLoadModuleShouldLoadTerraformBlock(t *testing.T) {
	mockFs := afero.NewMemMapFs()
	stub := gostub.Stub(&filesystem.Fs, mockFs)
	defer stub.Reset()
	_ = afero.WriteFile(mockFs, "/main.tf", []byte(`terraform {
  required_version = "~> 1.1.9"
  required_providers {
    mycloud = {
      source  = "mycorp/mycloud"
      version = "~> 1.0"
    }
  }
}`), 0644)
	sut, err := LoadModule(ModuleRef{
		Dir:    ".",
		AbsDir: "/",
	})
	require.NoError(t, err)
	assert.Len(t, sut.TerraformBlocks, 1)
	tb := sut.TerraformBlocks[0]
	terraformVersion, diag := tb.Attributes["required_version"].Expr.Value(&hcl.EvalContext{})
	require.False(t, diag.HasErrors())
	assert.Equal(t, "~> 1.1.9", terraformVersion.AsString())
	rpbs, ok := tb.NestedBlocks["required_providers"]
	require.True(t, ok)
	assert.Len(t, rpbs, 1)
	rpb := rpbs[0]
	assert.Len(t, rpb.Attributes, 1)
	providerConfig, ok := rpb.Attributes["mycloud"]
	require.True(t, ok)
	pc, diag := providerConfig.Expr.Value(&hcl.EvalContext{})
	require.False(t, diag.HasErrors())
	assert.Equal(t, cty.StringVal("mycorp/mycloud"), pc.GetAttr("source"))
	assert.Equal(t, cty.StringVal("~> 1.0"), pc.GetAttr("version"))
}

func TestLoadModuleShouldLoadLocalBlocks(t *testing.T) {
	mockFs := afero.NewMemMapFs()
	stub := gostub.Stub(&filesystem.Fs, mockFs)
	defer stub.Reset()

	// Write a Terraform configuration file with local blocks
	_ = afero.WriteFile(mockFs, "/main.tf", []byte(`
locals {
  local_var1 = "value1"
  local_var2 = "value2"
}
`), 0644)

	// Load the configuration file into a Module
	sut, err := LoadModule(ModuleRef{
		Dir:    ".",
		AbsDir: "/",
	})
	require.NoError(t, err)

	// Verify that the local blocks are loaded correctly
	assert.Len(t, sut.Locals, 2)
	localVar1 := sut.Locals[0].Attributes["local_var1"]
	localVar2 := sut.Locals[1].Attributes["local_var2"]
	if localVar1 == nil {
		localVar1 = sut.Locals[1].Attributes["local_var1"]
		localVar2 = sut.Locals[0].Attributes["local_var2"]
	}

	localVar1Value, diag := localVar1.Expr.Value(&hcl.EvalContext{})
	require.False(t, diag.HasErrors())
	assert.Equal(t, "value1", localVar1Value.AsString())

	localVar2Value, diag := localVar2.Expr.Value(&hcl.EvalContext{})
	require.False(t, diag.HasErrors())
	assert.Equal(t, "value2", localVar2Value.AsString())
}

func TestLoadModuleShouldBypassOverrideFiles(t *testing.T) {
	mockFs := afero.NewMemMapFs()
	stub := gostub.Stub(&filesystem.Fs, mockFs)
	defer stub.Reset()

	// Write Terraform configuration files including override files
	_ = afero.WriteFile(mockFs, "/main.tf", []byte(`resource "fake_resource" "this" {}`), 0644)
	_ = afero.WriteFile(mockFs, "/main_override.tf", []byte(`resource "fake_resource" "override" {}`), 0644)
	_ = afero.WriteFile(mockFs, "/override.tf", []byte(`resource "fake_resource" "another_override" {}`), 0644)

	// Load the configuration files into a Module
	sut, err := LoadModule(ModuleRef{
		Dir:    ".",
		AbsDir: "/",
	})
	require.NoError(t, err)

	// Verify that the override files are bypassed
	assert.Len(t, sut.ResourceBlocks, 1)
	assert.Equal(t, "this", sut.ResourceBlocks[0].Labels[1])
}
