package terraform

import (
	"github.com/hashicorp/hcl/v2"
	"github.com/heimdalr/dag"
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

func TestLoadModuleShouldBuildDagForResourceBlocks(t *testing.T) {
	mockFs := afero.NewMemMapFs()
	stub := gostub.Stub(&filesystem.Fs, mockFs)
	defer stub.Reset()

	_ = afero.WriteFile(mockFs, "/main.tf", []byte(`resource "fake_resource" "this" {
		depends_on = [data.fake_data.this]
	}
	data "fake_data" "this" {}
	`), 0644)

	sut, err := LoadModule(ModuleRef{
		Dir:    ".",
		AbsDir: "/",
	})
	require.NoError(t, err)

	assert.Len(t, sut.ResourceBlocks, 1)
	assert.Len(t, sut.DataBlocks, 1)

	// Verify DAG vertices
	assertHasVertex(t, sut.BlockDag, "resource.fake_resource.this")
	assertHasVertex(t, sut.BlockDag, "data.fake_data.this")
	assertIsDownstream(t, sut.BlockDag, "data.fake_data.this", "resource.fake_resource.this")
}

func TestLoadModuleShouldBuildDagForModuleBlocks(t *testing.T) {
	mockFs := afero.NewMemMapFs()
	stub := gostub.Stub(&filesystem.Fs, mockFs)
	defer stub.Reset()

	_ = afero.WriteFile(mockFs, "/main.tf", []byte(`module "fake_module" {
		source = "./fake_module"
	}
	output "module_output" {
		value = module.fake_module.output
	}
	`), 0644)

	sut, err := LoadModule(ModuleRef{
		Dir:    ".",
		AbsDir: "/",
	})
	require.NoError(t, err)

	assert.Len(t, sut.ModuleBlocks, 1)
	assert.Len(t, sut.Outputs, 1)

	// Verify DAG vertices
	assertHasVertex(t, sut.BlockDag, "module.fake_module")
	assertHasVertex(t, sut.BlockDag, "output.module_output")
	assertIsDownstream(t, sut.BlockDag, "module.fake_module", "output.module_output")
}

func TestLoadModuleShouldBuildDagForLocalBlocks(t *testing.T) {
	mockFs := afero.NewMemMapFs()
	stub := gostub.Stub(&filesystem.Fs, mockFs)
	defer stub.Reset()

	_ = afero.WriteFile(mockFs, "/main.tf", []byte(`locals {
		local_var1 = "value1"
		local_var2 = local.local_var1
	}
	`), 0644)

	sut, err := LoadModule(ModuleRef{
		Dir:    ".",
		AbsDir: "/",
	})
	require.NoError(t, err)

	assert.Len(t, sut.Locals, 2)

	// Verify DAG vertices
	assertHasVertex(t, sut.BlockDag, "local.local_var1")
	assertHasVertex(t, sut.BlockDag, "local.local_var2")
	assertIsDownstream(t, sut.BlockDag, "local.local_var1", "local.local_var2")
}

func TestLoadModuleShouldBuildDagForVariableAndOutputBlocks(t *testing.T) {
	mockFs := afero.NewMemMapFs()
	stub := gostub.Stub(&filesystem.Fs, mockFs)
	defer stub.Reset()

	_ = afero.WriteFile(mockFs, "/main.tf", []byte(`variable "var1" {}
	output "output1" {
		value = var.var1
	}
	`), 0644)

	sut, err := LoadModule(ModuleRef{
		Dir:    ".",
		AbsDir: "/",
	})
	require.NoError(t, err)

	assert.Len(t, sut.Variables, 1)
	assert.Len(t, sut.Outputs, 1)

	// Verify DAG vertices
	assertHasVertex(t, sut.BlockDag, "var.var1")
	assertHasVertex(t, sut.BlockDag, "output.output1")

	// Verify DAG edges
	assertIsDownstream(t, sut.BlockDag, "var.var1", "output.output1")
}

func TestLoadModuleShouldBuildDagForVariableReferenceInResourceBlock(t *testing.T) {
	mockFs := afero.NewMemMapFs()
	stub := gostub.Stub(&filesystem.Fs, mockFs)
	defer stub.Reset()

	_ = afero.WriteFile(mockFs, "/main.tf", []byte(`
variable "var1" {}
resource "fake_resource" "this" {
  dynamic "nested_block" {
    for_each = [1]
    content {
      attr = var.var1
    }
  }
}
`), 0644)

	sut, err := LoadModule(ModuleRef{
		Dir:    ".",
		AbsDir: "/",
	})
	require.NoError(t, err)

	assert.Len(t, sut.Variables, 1)
	assert.Len(t, sut.ResourceBlocks, 1)

	// Verify DAG vertices
	assertHasVertex(t, sut.BlockDag, "var.var1")
	assertHasVertex(t, sut.BlockDag, "resource.fake_resource.this")

	// Verify DAG edges
	assertIsDownstream(t, sut.BlockDag, "var.var1", "resource.fake_resource.this")
}

func TestLoadModuleShouldBuildDagForVariableReferenceInDataBlock(t *testing.T) {
	mockFs := afero.NewMemMapFs()
	stub := gostub.Stub(&filesystem.Fs, mockFs)
	defer stub.Reset()

	_ = afero.WriteFile(mockFs, "/main.tf", []byte(`
variable "var1" {}
data "fake_data" "this" {
  dynamic "nested_block" {
    for_each = [1]
    content {
      attr = var.var1
    }
  }
}
`), 0644)

	sut, err := LoadModule(ModuleRef{
		Dir:    ".",
		AbsDir: "/",
	})
	require.NoError(t, err)

	assert.Len(t, sut.Variables, 1)
	assert.Len(t, sut.DataBlocks, 1)

	// Verify DAG vertices
	assertHasVertex(t, sut.BlockDag, "var.var1")
	assertHasVertex(t, sut.BlockDag, "data.fake_data.this")

	// Verify DAG edges
	assertIsDownstream(t, sut.BlockDag, "var.var1", "data.fake_data.this")
}

func TestLoadModuleShouldBuildDagForVariableReferenceInLocalsBlock(t *testing.T) {
	mockFs := afero.NewMemMapFs()
	stub := gostub.Stub(&filesystem.Fs, mockFs)
	defer stub.Reset()

	_ = afero.WriteFile(mockFs, "/main.tf", []byte(`
variable "var1" {}
locals {
  local_var1 = var.var1
}
`), 0644)

	sut, err := LoadModule(ModuleRef{
		Dir:    ".",
		AbsDir: "/",
	})
	require.NoError(t, err)

	assert.Len(t, sut.Variables, 1)
	assert.Len(t, sut.Locals, 1)

	// Verify DAG vertices
	assertHasVertex(t, sut.BlockDag, "var.var1")
	assertHasVertex(t, sut.BlockDag, "local.local_var1")

	// Verify DAG edges
	assertIsDownstream(t, sut.BlockDag, "var.var1", "local.local_var1")
}

func TestLoadModuleShouldBuildDagForLocalsReferenceInResourceBlock(t *testing.T) {
	mockFs := afero.NewMemMapFs()
	stub := gostub.Stub(&filesystem.Fs, mockFs)
	defer stub.Reset()

	_ = afero.WriteFile(mockFs, "/main.tf", []byte(`
locals {
  local_var1 = "value1"
}
resource "fake_resource" "this" {
  dynamic "nested_block" {
    for_each = [1]
    content {
      attr = local.local_var1
    }
  }
}
`), 0644)

	sut, err := LoadModule(ModuleRef{
		Dir:    ".",
		AbsDir: "/",
	})
	require.NoError(t, err)

	assert.Len(t, sut.Locals, 1)
	assert.Len(t, sut.ResourceBlocks, 1)

	// Verify DAG vertices
	assertHasVertex(t, sut.BlockDag, "local.local_var1")
	assertHasVertex(t, sut.BlockDag, "resource.fake_resource.this")

	// Verify DAG edges
	assertIsDownstream(t, sut.BlockDag, "local.local_var1", "resource.fake_resource.this")
}

func TestLoadModuleShouldBuildDagForLocalsReferenceInDataBlock(t *testing.T) {
	mockFs := afero.NewMemMapFs()
	stub := gostub.Stub(&filesystem.Fs, mockFs)
	defer stub.Reset()

	_ = afero.WriteFile(mockFs, "/main.tf", []byte(`
locals {
  local_var1 = ["value1"]
}
data "fake_data" "this" {
  dynamic "nested_block" {
    for_each = local.local_var1
    content {
      attr = nested_block.value
    }
  }
}
`), 0644)

	sut, err := LoadModule(ModuleRef{
		Dir:    ".",
		AbsDir: "/",
	})
	require.NoError(t, err)

	assert.Len(t, sut.Locals, 1)
	assert.Len(t, sut.DataBlocks, 1)

	// Verify DAG vertices
	assertHasVertex(t, sut.BlockDag, "local.local_var1")
	assertHasVertex(t, sut.BlockDag, "data.fake_data.this")

	// Verify DAG edges
	assertIsDownstream(t, sut.BlockDag, "local.local_var1", "data.fake_data.this")
}

func TestLoadModuleShouldBuildDagForDynamicBlockWithIterator(t *testing.T) {
	mockFs := afero.NewMemMapFs()
	stub := gostub.Stub(&filesystem.Fs, mockFs)
	defer stub.Reset()

	_ = afero.WriteFile(mockFs, "/main.tf", []byte(`
variable "var1" {
  default = [1,2,3]
}
resource "fake_resource" "this" {
  dynamic "nested_block" {
    for_each = var.var1
    iterator = "iter"
    content {
      attr = iter.value
    }
  }
}
`), 0644)

	sut, err := LoadModule(ModuleRef{
		Dir:    ".",
		AbsDir: "/",
	})
	require.NoError(t, err)

	assert.Len(t, sut.Variables, 1)
	assert.Len(t, sut.ResourceBlocks, 1)

	// Verify DAG vertices
	assertHasVertex(t, sut.BlockDag, "var.var1")
	assertHasVertex(t, sut.BlockDag, "resource.fake_resource.this")

	// Verify DAG edges
	assertIsDownstream(t, sut.BlockDag, "var.var1", "resource.fake_resource.this")
}

func TestLoadModuleShouldBuildDagForNestedDynamicBlocks(t *testing.T) {
	mockFs := afero.NewMemMapFs()
	stub := gostub.Stub(&filesystem.Fs, mockFs)
	defer stub.Reset()

	_ = afero.WriteFile(mockFs, "/main.tf", []byte(`
variable "var1" {
  default = [1, 2, 3]
}
resource "fake_resource" "this" {
  dynamic "outer_block" {
    for_each = var.var1
	iterator = "outer"
    content {
      dynamic "inner_block" {
        for_each = [1]
        content {
          attr = outer.value + inner_block.value
        }
      }
    }
  }
}
`), 0644)

	sut, err := LoadModule(ModuleRef{
		Dir:    ".",
		AbsDir: "/",
	})
	require.NoError(t, err)

	assert.Len(t, sut.Variables, 1)
	assert.Len(t, sut.ResourceBlocks, 1)

	// Verify DAG vertices
	assertHasVertex(t, sut.BlockDag, "var.var1")
	assertHasVertex(t, sut.BlockDag, "resource.fake_resource.this")

	// Verify DAG edges
	assertIsDownstream(t, sut.BlockDag, "var.var1", "resource.fake_resource.this")
}

func TestLoadModuleShouldBuildDagForResourceWithStaticAndDynamicNestedBlocks(t *testing.T) {
	mockFs := afero.NewMemMapFs()
	stub := gostub.Stub(&filesystem.Fs, mockFs)
	defer stub.Reset()

	_ = afero.WriteFile(mockFs, "/main.tf", []byte(`
variable "var1" {
  default = "value1"
}

locals {
  local_var1 = ["value2"]
}

resource "fake_resource" "this" {
  static_block {
    attr = var.var1
  }
  dynamic "dynamic_block" {
    for_each = local.local_var1
    content {
      attr = dynamic_block.value
    }
  }
}
`), 0644)

	sut, err := LoadModule(ModuleRef{
		Dir:    ".",
		AbsDir: "/",
	})
	require.NoError(t, err)

	assert.Len(t, sut.Variables, 1)
	assert.Len(t, sut.Locals, 1)
	assert.Len(t, sut.ResourceBlocks, 1)

	// Verify DAG vertices
	assertHasVertex(t, sut.BlockDag, "var.var1")
	assertHasVertex(t, sut.BlockDag, "local.local_var1")
	assertHasVertex(t, sut.BlockDag, "resource.fake_resource.this")

	// Verify DAG edges
	assertIsDownstream(t, sut.BlockDag, "var.var1", "resource.fake_resource.this")
	assertIsDownstream(t, sut.BlockDag, "local.local_var1", "resource.fake_resource.this")
}

func assertHasVertex(t *testing.T, d *dag.DAG, id string) {
	v, err := d.GetVertex(id)
	require.NoError(t, err)
	assert.NotNil(t, v)
}

func assertIsDownstream(t *testing.T, d *dag.DAG, upstream, downstream string) {
	edgeExist, err := d.IsEdge(upstream, downstream)
	require.NoError(t, err)
	assert.True(t, edgeExist)
}
