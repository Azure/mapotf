package terraform_test

import (
	"github.com/Azure/mapotf/pkg/terraform"
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/stretchr/testify/require"
	"github.com/zclconf/go-cty/cty"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewNestedBlock(t *testing.T) {
	// Define some Terraform code with a nested block
	sut := newBlock(t, `
	resource "azurerm_kubernetes_cluster" "example" {
  name                = "example-aks1"
  location            = azurerm_resource_group.example.location
  resource_group_name = azurerm_resource_group.example.name
  dns_prefix          = "exampleaks1"

  default_node_pool {
    name       = "default"
    node_count = 1
    vm_size    = "Standard_D2_v2"
  }

  identity {
    type = "SystemAssigned"
  }

  dynamic "microsoft_defender" {
	for_each = var.msdefender_log_analytics_workspace == null ? ["microsoft_defender"] : []
    content {
	  log_analytics_workspace_id = var.msdefender_log_analytics_workspace.id
	}
  }

  tags = {
    Environment = "Production"
  }
}
	`)

	assert.Len(t, sut.NestedBlocks, 3)
	defaultNodePoolBlock := sut.NestedBlocks["default_node_pool"][0]
	assert.Equal(t, "default_node_pool", defaultNodePoolBlock.Type)
	assert.Len(t, defaultNodePoolBlock.Attributes, 3)
	assert.Empty(t, defaultNodePoolBlock.NestedBlocks)
	assert.Contains(t, defaultNodePoolBlock.Attributes, "name")
	assert.Contains(t, defaultNodePoolBlock.Attributes, "node_count")
	assert.Contains(t, defaultNodePoolBlock.Attributes, "vm_size")
	identityBlock := sut.NestedBlocks["identity"][0]
	assert.Equal(t, "identity", identityBlock.Type)
	assert.Empty(t, identityBlock.NestedBlocks)
	assert.Len(t, identityBlock.Attributes, 1)
	assert.Equal(t, `"SystemAssigned"`, identityBlock.Attributes["type"].String())
	mdBlock := sut.NestedBlocks["microsoft_defender"][0]
	assert.Equal(t, "microsoft_defender", mdBlock.Type)
	require.NotNil(t, mdBlock.ForEach)
	assert.Equal(t, `var.msdefender_log_analytics_workspace == null ? ["microsoft_defender"] : []`, mdBlock.ForEach.String())
	assert.Equal(t, `var.msdefender_log_analytics_workspace.id`, mdBlock.Attributes["log_analytics_workspace_id"].String())
}

func TestNestedBlock_Iterator(t *testing.T) {
	// Define some Terraform code with a dynamic block that includes an iterator attribute
	code := `
resource "azurerm_kubernetes_cluster" "example" {
	dynamic "microsoft_defender" {
		for_each = var.msdefender_log_analytics_workspace == null ? ["microsoft_defender"] : []
		iterator = defender
		content {
			log_analytics_workspace_id = var.msdefender_log_analytics_workspace.id
		}
	}
}
`
	// Parse the Terraform code and create a NestedBlock
	sut := newBlock(t, code)
	mdBlock := sut.NestedBlocks["microsoft_defender"][0]

	// Verify that the Iterator attribute has been decoded correctly
	require.NotNil(t, mdBlock.Iterator)
	assert.Equal(t, "defender", mdBlock.Iterator.String())
}

func TestNewNestedInNestedBlock(t *testing.T) {
	cases := []struct {
		desc string
		code string
	}{
		{
			desc: "normal nested block",
			code: `
	resource "azurerm_kubernetes_cluster" "example" {
  name                = "example-aks1"
  location            = azurerm_resource_group.example.location
  resource_group_name = azurerm_resource_group.example.name
  dns_prefix          = "exampleaks1"

  web_app_routing {
    dns_zone_id = var.dns_zone_id
    web_app_routing_identity {
		client_id = var.client_id
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
}
	`,
		},
		{
			desc: "dynamic block",
			code: `
	resource "azurerm_kubernetes_cluster" "example" {
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
}
	`,
		},
	}

	for _, cc := range cases {
		t.Run(cc.desc, func(t *testing.T) {
			// Define some Terraform code with a nested block
			sut := newBlock(t, cc.code)
			webAppRoutingBlock := sut.NestedBlocks["web_app_routing"][0]
			assert.Equal(t, "web_app_routing", webAppRoutingBlock.Type)
			assert.Contains(t, webAppRoutingBlock.Attributes, "dns_zone_id")
			assert.Len(t, webAppRoutingBlock.NestedBlocks["web_app_routing_identity"], 1)
			identityBlock := webAppRoutingBlock.NestedBlocks["web_app_routing_identity"][0]
			assert.NotNil(t, identityBlock.Attributes, "client_id")
		})
	}
}

func TestNestedBlock_NestInNestedBlockHasDifferentSchema(t *testing.T) {
	code := `
resource "fake_resource" this {
  top_block {
	second_block {
	  id = 123
	}
  }
  top_block {
	second_block {
      name = "John"
	}
  }
  top_block {
	second_block{
	  third_block{
		enabled = true
      }
	}
  }
}
`
	sut := newBlock(t, code)
	assert.Len(t, sut.NestedBlocks["top_block"], 3)
	ctx := &hcl.EvalContext{
		Variables: map[string]cty.Value{
			"result": sut.EvalContext(),
		},
	}
	exp, diag := hclsyntax.ParseExpression([]byte("result.top_block.1.second_block.0.name"), "main.hcl", hcl.InitialPos)
	require.Falsef(t, diag.HasErrors(), diag.Error())
	value, diag := exp.Value(ctx)
	require.Falsef(t, diag.HasErrors(), diag.Error())
	assert.Equal(t, `"John"`, value.AsString())
}

func TestNestedBlock_RemoveNestedBlock(t *testing.T) {
	cfg := `
root_block {
  nested_block{}
}
`
	sFile, diag := hclsyntax.ParseConfig([]byte(cfg), "test.hcl", hcl.InitialPos)
	require.False(t, diag.HasErrors())
	wFile, diag := hclwrite.ParseConfig([]byte(cfg), "test.hcl", hcl.InitialPos)
	require.False(t, diag.HasErrors())
	nb := terraform.NewNestedBlock(sFile.Body.(*hclsyntax.Body).Blocks[0], wFile.Body().Blocks()[0])

	// Call RemoveContent to remove the nested block
	nb.RemoveContent("nested_block")

	// Assert that the nested block has been removed correctly
	assert.Empty(t, nb.WriteBlock.Body().Blocks())
}

func TestNestedBlock_RemoveDynamicNestedBlock(t *testing.T) {
	cfg := `
root_block {
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
	nb := terraform.NewNestedBlock(sFile.Body.(*hclsyntax.Body).Blocks[0], wFile.Body().Blocks()[0])

	// Call RemoveContent to remove the nested block
	nb.RemoveContent("nested_block")

	// Assert that the nested block has been removed correctly
	assert.Empty(t, nb.WriteBlock.Body().Blocks())
}

func TestNestedBlock_RemoveDeepNestedBlock(t *testing.T) {
	cfg := `
root_block {
  nested_block{
   target_block {}
  }
  nested_block {
   target_block {
	 another_block {}
   }
	non_target_block {}
  }
  nested_block {
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
root_block {
  nested_block{
  }
  nested_block {
	non_target_block {}
  }
  nested_block {
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
	nb := terraform.NewNestedBlock(sFile.Body.(*hclsyntax.Body).Blocks[0], wFile.Body().Blocks()[0])

	// Call RemoveContent to remove the nested block
	nb.RemoveContent("nested_block/target_block")
	expected = formatHcl(expected)
	actual := formatHcl(string(nb.WriteBlock.BuildTokens(nil).Bytes()))
	assert.Equal(t, expected, actual)
}

func TestNestedBlock_EvalContextWithIterator(t *testing.T) {
	// Define some Terraform code with a dynamic block that includes an iterator attribute
	code := `
resource "azurerm_kubernetes_cluster" "example" {
	dynamic "microsoft_defender" {
		for_each = var.msdefender_log_analytics_workspace == null ? ["microsoft_defender"] : []
		iterator = defender
		content {
			log_analytics_workspace_id = var.msdefender_log_analytics_workspace.id
		}
	}
}
`
	// Parse the Terraform code and create a NestedBlock
	sut := newBlock(t, code)
	mdBlock := sut.NestedBlocks["microsoft_defender"][0]

	// Call the EvalContext method
	obj := mdBlock.EvalContext()

	// Verify that the iterator attribute is correctly included in the evaluation context
	require.NotNil(t, obj)
	assert.Equal(t, cty.StringVal("defender"), obj.GetAttr("iterator"))
}

func TestNestedBlock_NestedBlockToString(t *testing.T) {
	cfg := `
root_block {
  nested_block{
   target_block {}
  }
  nested_block {
   target_block {
	 another_block {}
   }
	non_target_block {}
  }
  nested_block {
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
	expected := cfg
	sFile, diag := hclsyntax.ParseConfig([]byte(cfg), "test.hcl", hcl.InitialPos)
	require.False(t, diag.HasErrors())
	wFile, diag := hclwrite.ParseConfig([]byte(cfg), "test.hcl", hcl.InitialPos)
	require.False(t, diag.HasErrors())
	nb := terraform.NewNestedBlock(sFile.Body.(*hclsyntax.Body).Blocks[0], wFile.Body().Blocks()[0])

	// Call RemoveContent to remove the nested block
	str := nb.EvalContext().GetAttr("mptf").GetAttr("tostring").AsString()
	expected = formatHcl(expected)
	actual := formatHcl(str)
	assert.Equal(t, expected, actual)
	lastBlockStr := nb.EvalContext().GetAttr("nested_block").Index(cty.NumberIntVal(4)).GetAttr("mptf").GetAttr("tostring").AsString()
	expected = formatHcl(`dynamic "nested_block" {
    for_each = var.enabled ? [1] : []
	content {
	  dynamic "target_block" {
	    for_each = var.enabled ? [1] : []
        content {
	    }
     }
	}
  }`)
	actual = formatHcl(lastBlockStr)
	assert.Equal(t, expected, actual)
}

func formatHcl(inputHcl string) string {
	return strings.Trim(string(hclwrite.Format([]byte(inputHcl))), "\n")
}
