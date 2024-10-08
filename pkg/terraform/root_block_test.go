package terraform_test

import (
	"fmt"
	"testing"

	"github.com/Azure/mapotf/pkg/terraform"
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/lonegunmanb/hclfuncs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zclconf/go-cty/cty"
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

func TestBlockAddressGetValue(t *testing.T) {
	code := `resource "azurerm_kubernetes_cluster" "example" {
  name                = "example-aks1"
  location            = azurerm_resource_group.example.location
  resource_group_name = azurerm_resource_group.example.name
  dns_prefix          = "exampleaks1"

  dynamic "web_app_routing" {
	for_each = var.web_app_routing
	content {
      dns_zone_id = web_app_routing.value.dns_zone_id
      dynamic "web_app_routing_identity" {
		for_each = web_app_routing.value.web_app_routing_identity == null ? [] : [web_app_routing.value.web_app_routing_identity]
		content {
	    	client_id = web_app_routing_identity.value.client_id
		}
	  }
	  web_app_routing_identity {
	    user_assigned_identity_id = var.user_assigned_identity_id
	  }
	}
  }

  default_node_pool {
    name       = "default"
    node_count = 1
    vm_size    = "Standard_D2_v2"
  }

  identity {
    type = "SystemAssigned"
  }

  tags = {
    Environment = "Production"
  }
}`
	sut := newBlock(t, code)
	ctx := &hcl.EvalContext{
		Variables: map[string]cty.Value{
			"result": sut.EvalContext(),
		},
	}
	cases := []struct {
		desc     string
		path     string
		expected string
	}{
		{
			desc:     "root attribute",
			path:     "name",
			expected: `"example-aks1"`,
		},
		{
			desc:     "nested block's attribute",
			path:     "default_node_pool.0.name",
			expected: `"default"`,
		},
		{
			desc:     "nested block's attribute, bracket syntax",
			path:     "default_node_pool[0].name",
			expected: `"default"`,
		},
		{
			desc:     "dynamic block's for_each",
			path:     "web_app_routing.0.for_each",
			expected: `var.web_app_routing`,
		},
		{
			desc:     "dynamic block's attribute",
			path:     "web_app_routing.0.dns_zone_id",
			expected: "web_app_routing.value.dns_zone_id",
		},
		{
			desc:     "first nested block instance",
			path:     "web_app_routing.0.web_app_routing_identity.0.client_id",
			expected: "web_app_routing_identity.value.client_id",
		},
		{
			desc:     "second nested block instance",
			path:     "web_app_routing.0.web_app_routing_identity.1.user_assigned_identity_id",
			expected: "var.user_assigned_identity_id",
		},
		{
			desc: "tags",
			path: "tags",
			expected: `{
    Environment = "Production"
  }`,
		},
	}
	for _, cc := range cases {
		t.Run(cc.desc, func(t *testing.T) {
			exp := fmt.Sprintf(`result.%s`, cc.path)
			expression, diag := hclsyntax.ParseExpression([]byte(exp), "main.hcl", hcl.InitialPos)
			require.Falsef(t, diag.HasErrors(), diag.Error())
			value, diag := expression.Value(ctx)
			require.False(t, diag.HasErrors())
			assert.Equal(t, cc.expected, value.AsString())
		})
	}
}

func TestBlockAddress_GetNonExistAttributeShouldUseTryFunction(t *testing.T) {
	sut := newBlock(t, `
	resource "azurerm_resource_group" "example" {
		name           = "test"
		location 	   = "eastus"
	}
	`)
	ctx := &hcl.EvalContext{
		Variables: map[string]cty.Value{
			"result": sut.EvalContext(),
		},
		Functions: hclfuncs.Functions("."),
	}
	expression, diag := hclsyntax.ParseExpression([]byte("try(result.for_each, null)"), "main.hcl", hcl.InitialPos)
	require.Falsef(t, diag.HasErrors(), diag.Error())
	value, diag := expression.Value(ctx)
	require.Falsef(t, diag.HasErrors(), diag.Error())
	assert.True(t, value.IsNull())
}

func TestReflectionObjectInEvalContext(t *testing.T) {
	// Define some Terraform code
	sut := newBlock(t, `
	resource "azurerm_resource_group" "example" {
		name           = "test"
		location 	   = "eastus"
	}
	`)

	ctx := make(map[string]cty.Value)
	terraform.RootBlockReflectionInformation(ctx, sut)
	assert.Contains(t, ctx, "mptf")
	obj := ctx["mptf"]
	assert.Equal(t, cty.StringVal("resource.azurerm_resource_group.example"), obj.GetAttr("block_address"))
	assert.Equal(t, cty.StringVal("azurerm_resource_group.example"), obj.GetAttr("terraform_address"))
	assert.Equal(t, cty.StringVal("resource"), obj.GetAttr("block_type"))
	assert.Equal(t, cty.ListVal([]cty.Value{
		cty.StringVal("azurerm_resource_group"),
		cty.StringVal("example"),
	}), obj.GetAttr("block_labels"))
	assert.Equal(t, cty.ListVal([]cty.Value{
		cty.StringVal("azurerm_resource_group"),
		cty.StringVal("example"),
	}), obj.GetAttr("block_labels"))
	assert.Equal(t, cty.ObjectVal(map[string]cty.Value{
		"file_name":    cty.StringVal("test"),
		"start_line":   cty.NumberIntVal(2),
		"start_column": cty.NumberIntVal(2),
		"end_line":     cty.NumberIntVal(5),
		"end_column":   cty.NumberIntVal(3),
	}), obj.GetAttr("range"))
	assert.Equal(t, cty.ObjectVal(map[string]cty.Value{
		"key":      cty.StringVal(""),
		"version":  cty.StringVal(""),
		"source":   cty.StringVal(""),
		"dir":      cty.StringVal(""),
		"abs_dir":  cty.StringVal(""),
		"git_hash": cty.StringVal(""),
	}), obj.GetAttr("module"))
}

func TestNestedBlock_SameResourceBlockContainsSameNestedBlocksWithDifferentSchema(t *testing.T) {
	code := `
resource "fake_resource" this {
  top_block {
	second_block {
	  id = 123
	}
  }
}

resource "fake_resource" that {
  top_block {
	third_block {
	  name = "John"
	}
  }
}
`
	sut := newBlocks(t, code)
	assert.Len(t, sut, 2)
	ctx := &hcl.EvalContext{
		Variables: map[string]cty.Value{
			"result": terraform.ListOfObject(sut),
		},
	}
	v := expressionValue(t, "result.0.top_block.0.second_block.0.id", ctx)
	assert.Equal(t, "123", v.AsString())
	v = expressionValue(t, "result.1.top_block.0.third_block.0.name", ctx)
	assert.Equal(t, `"John"`, v.AsString())
}

func TestRootBlock_RemoveDeepNestedBlock(t *testing.T) {
	cfg := `
root_block "root" {
  nested_block{
    target_block {}
  }
  nested_block{
    dynamic "target_block" {
	  for_each = var.enabled ? [1] : []
      content {
		
	  }
    } 
  }
  dynamic "nested_block" {
    for_each = var.enabled ? [1] : []
    content {
      target_block {}
	}
  }
  dynamic "nested_block" {
	for_each = var.enabled ? [1] : []
    content {
      dynamic "target_block" {
		for_each = var.enabled ? [1] : []
		content {
		
		}
	  } 
	}
  }
}
`
	expected := `
root_block "root" {
  nested_block{
  }
  nested_block{
  }
  dynamic "nested_block" {
    for_each = var.enabled ? [1] : []
    content {
	}
  }
  dynamic "nested_block" {
	for_each = var.enabled ? [1] : []
    content {
	}
  }
}
`
	sFile, diag := hclsyntax.ParseConfig([]byte(cfg), "test.hcl", hcl.InitialPos)
	require.False(t, diag.HasErrors())
	wFile, diag := hclwrite.ParseConfig([]byte(cfg), "test.hcl", hcl.InitialPos)
	require.False(t, diag.HasErrors())
	rb := terraform.NewBlock(nil, sFile.Body.(*hclsyntax.Body).Blocks[0], wFile.Body().Blocks()[0])

	// Call RemoveContent to remove the nested block
	rb.RemoveContent("nested_block.target_block")

	assert.Equal(t, formatHcl(expected), formatHcl(string(rb.WriteBlock.BuildTokens(nil).Bytes())))
}

func TestRootBlock_RemoveDynamicNestedBlock(t *testing.T) {
	cfg := `
root_block "root" {
  dynamic "nested_block" {
    for_each = var.enabled ? [1] : []
    content {
      target_block {}
	}
  }
}
`
	expected := `
root_block "root" {
}
`
	sFile, diag := hclsyntax.ParseConfig([]byte(cfg), "test.hcl", hcl.InitialPos)
	require.False(t, diag.HasErrors())
	wFile, diag := hclwrite.ParseConfig([]byte(cfg), "test.hcl", hcl.InitialPos)
	require.False(t, diag.HasErrors())
	rb := terraform.NewBlock(nil, sFile.Body.(*hclsyntax.Body).Blocks[0], wFile.Body().Blocks()[0])

	// Call RemoveContent to remove the nested block
	rb.RemoveContent("nested_block")

	assert.Equal(t, formatHcl(expected), formatHcl(string(rb.WriteBlock.BuildTokens(nil).Bytes())))
}

func newBlock(t *testing.T, code string) *terraform.RootBlock {

	// Parse the Terraform code
	readFile, diags := hclsyntax.ParseConfig([]byte(code), "test", hcl.InitialPos)
	require.False(t, diags.HasErrors())
	writeFile, diags := hclwrite.ParseConfig([]byte(code), "test", hcl.InitialPos)
	require.False(t, diags.HasErrors())

	// Get the first block from the parsed file
	rb := readFile.Body.(*hclsyntax.Body).Blocks[0]
	wb := writeFile.Body().Blocks()[0]

	// Call the function under test
	block := terraform.NewBlock(nil, rb, wb)
	return block
}

func newBlocks(t *testing.T, code string) []*terraform.RootBlock {
	// Parse the Terraform code
	readFile, diags := hclsyntax.ParseConfig([]byte(code), "test", hcl.InitialPos)
	require.False(t, diags.HasErrors())
	writeFile, diags := hclwrite.ParseConfig([]byte(code), "test", hcl.InitialPos)
	require.False(t, diags.HasErrors())

	var blocks []*terraform.RootBlock

	for i, rb := range readFile.Body.(*hclsyntax.Body).Blocks {
		wb := writeFile.Body().Blocks()[i]
		blocks = append(blocks, terraform.NewBlock(nil, rb, wb))
	}
	return blocks
}

func expressionValue(t *testing.T, expression string, ctx *hcl.EvalContext) cty.Value {
	exp, diag := hclsyntax.ParseExpression([]byte(expression), "main.hcl", hcl.InitialPos)
	require.Falsef(t, diag.HasErrors(), diag.Error())
	value, diag := exp.Value(ctx)
	require.Falsef(t, diag.HasErrors(), diag.Error())
	return value
}
