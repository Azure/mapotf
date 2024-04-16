package terraform_test

import (
	"testing"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/lonegunmanb/mptf/pkg/terraform"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewTerraformBlock(t *testing.T) {
	// Define some Terraform code
	sut := newBlock(t, `
	resource "azurerm_resource_group" "example" {
		name           = "test"
		location 	   = "eastus"
	}
	`)

	// Assert that the returned block has the expected properties
	assert.Equal(t, "resource", sut.Type)
	assert.Equal(t, []string{"azurerm_resource_group", "example"}, sut.Labels)
	assert.Equal(t, "resource.azurerm_resource_group.example", sut.Address)
	assert.Nil(t, sut.Count)
	assert.Nil(t, sut.ForEach)
	assert.Contains(t, sut.Attributes, "name")
	assert.Contains(t, sut.Attributes, "location")
}

func TestNewTerraformBlock_Count(t *testing.T) {
	sut := newBlock(t, `
	resource "azurerm_resource_group" "example" {
		count 		   = var.create_rg ? 1 : 0
		name           = "test"
		location 	   = "eastus"
	}
	`)
	assert.Nil(t, sut.ForEach)
	assert.NotNil(t, sut.Count)
	assert.Equal(t, "var.create_rg ? 1 : 0", sut.Count.String())
}

func TestNewTerraformBlock_ForEach(t *testing.T) {
	// Define some Terraform code
	sut := newBlock(t, `
	resource "azurerm_resource_group" "example" {
		for_each 	   = var.create_rg ? toset(["rg"]) : []
		name           = "test"
		location 	   = "eastus"
	}
	`)
	assert.Nil(t, sut.Count)
	assert.NotNil(t, sut.ForEach)
	assert.Equal(t, `var.create_rg ? toset(["rg"]) : []`, sut.ForEach.String())
}

func newBlock(t *testing.T, code string) *terraform.Block {

	// Parse the Terraform code
	readFile, diags := hclsyntax.ParseConfig([]byte(code), "test", hcl.InitialPos)
	require.False(t, diags.HasErrors())
	writeFile, diags := hclwrite.ParseConfig([]byte(code), "test", hcl.InitialPos)
	require.False(t, diags.HasErrors())

	// Get the first block from the parsed file
	rb := readFile.Body.(*hclsyntax.Body).Blocks[0]
	wb := writeFile.Body().Blocks()[0]

	// Call the function under test
	block := terraform.NewBlock(rb, wb)
	return block
}
