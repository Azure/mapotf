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

func TestNewAttribute(t *testing.T) {
	// Define some Terraform code
	sut := newAttribute(t, `
	resource "azurerm_resource_group" "example" {
		name = "test"
	}
	`, "name")

	// Assert that the returned attribute has the expected properties
	assert.Equal(t, "name", sut.Name)
	assert.Equal(t, `"test"`, sut.String())
}

func newAttribute(t *testing.T, code string, attributeName string) *terraform.Attribute {
	// Parse the Terraform code
	readFile, diags := hclsyntax.ParseConfig([]byte(code), "test", hcl.InitialPos)
	require.False(t, diags.HasErrors())
	writeFile, diags := hclwrite.ParseConfig([]byte(code), "test", hcl.InitialPos)
	require.False(t, diags.HasErrors())

	// Get the first attribute from the parsed file
	rb := readFile.Body.(*hclsyntax.Body).Blocks[0].Body.Attributes[attributeName]
	wb := writeFile.Body().Blocks()[0].Body().GetAttribute(attributeName)

	// Call the function under test
	attribute := terraform.NewAttribute(attributeName, rb, wb)
	return attribute
}
