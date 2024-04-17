package terraform_test

import (
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/stretchr/testify/require"
	"github.com/zclconf/go-cty/cty"
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
