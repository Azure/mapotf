package terraform

import (
	"path/filepath"
	"testing"

	filesystem "github.com/Azure/mapotf/pkg/fs"
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
	sut, err := LoadModule(TerraformModuleRef{
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
	m, err := LoadModule(TerraformModuleRef{
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
