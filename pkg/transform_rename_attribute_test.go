package pkg_test

import (
	"context"
	"github.com/prashantv/gostub"
	"github.com/spf13/afero"
	"testing"

	"github.com/Azure/mapotf/pkg"
	filesystem "github.com/Azure/mapotf/pkg/fs"
	"github.com/go-playground/assert/v2"
	"github.com/stretchr/testify/require"
)

const aksResourceTf = `
resource "azurerm_kubernetes_cluster" "example" {
  name                = "example-aks1"
  location            = azurerm_resource_group.example.location
  resource_group_name = azurerm_resource_group.example.name
  dns_prefix          = "exampleaks1"
  default_node_pool {
    name       = "default"
    node_count = 1
    vm_size    = "Standard_D2_v2"
    linux_os_config {
      swap_file_size_mb = 100
    }
  }
  identity {
    type = "SystemAssigned"
  }
  tags = {
    Environment = "Production"
  }
}
`
const resourceGroupDataSourceTf = `
data "azurerm_resource_group" "this" {
  location = "westus"
}
`
const vnetResourceTf = `
resource "azurerm_virtual_network" "example" {
  name                = "example-network"
  location            = azurerm_resource_group.example.location
  resource_group_name = azurerm_resource_group.example.name
  address_space       = ["10.0.0.0/16"]
  dns_servers         = ["10.0.0.4", "10.0.0.5"]

  subnet {
    name           = "subnet1"
    address_prefix = "10.0.1.0/24"
  }

  subnet {
    name           = "subnet2"
    address_prefix = "10.0.2.0/24"
    security_group = azurerm_network_security_group.example.id
  }

  tags = {
    environment = "Production"
  }
}
`

func TestRenameAttributeTransform_Apply(t *testing.T) {
	cases := []struct {
		desc        string
		cfg         string
		tfCfg       string
		expectedHCL string
	}{
		{
			desc: "Rename single attribute in root block",
			cfg: `
transform "rename_attribute" this {
	rename {
		resource_type  = "azurerm_kubernetes_cluster"
		attribute_path = ["location"]
		new_name       = "region"
	}
}
`,
			tfCfg: aksResourceTf,
			expectedHCL: `
resource "azurerm_kubernetes_cluster" "example" {
  name                = "example-aks1"
  resource_group_name = azurerm_resource_group.example.name
  dns_prefix          = "exampleaks1"
  default_node_pool {
    name       = "default"
    node_count = 1
    vm_size    = "Standard_D2_v2"
    linux_os_config {
      swap_file_size_mb = 100
    }
  }
  identity {
    type = "SystemAssigned"
  }
  tags = {
    Environment = "Production"
  }
  region = azurerm_resource_group.example.location
}
`,
		},
		{
			desc: "Rename nested attribute",
			cfg: `
transform "rename_attribute" this {
	rename {
		resource_type  = "azurerm_kubernetes_cluster"
		attribute_path = ["default_node_pool", "vm_size"]
		new_name       = "sku"
	}
}
`,
			tfCfg: aksResourceTf,
			expectedHCL: `
resource "azurerm_kubernetes_cluster" "example" {
  name                = "example-aks1"
  location            = azurerm_resource_group.example.location
  resource_group_name = azurerm_resource_group.example.name
  dns_prefix          = "exampleaks1"
  default_node_pool {
    name       = "default"
    node_count = 1
    linux_os_config {
      swap_file_size_mb = 100
    }
    sku = "Standard_D2_v2"
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
			desc: "Rename dynamic nested attribute",
			cfg: `
transform "rename_attribute" this {
	rename {
		resource_type  = "azurerm_kubernetes_cluster"
		attribute_path = ["default_node_pool", "vm_size"]
		new_name       = "sku"
	}
}
`,
			tfCfg: `
resource "azurerm_kubernetes_cluster" "example" {
  name                = "example-aks1"
  location            = azurerm_resource_group.example.location
  resource_group_name = azurerm_resource_group.example.name
  dns_prefix          = "exampleaks1"
  dynamic "default_node_pool" {
    for_each = ["Standard_D2_v2"]
    content {
      name       = "default"
      node_count = 1
      vm_size    = default_node_pool.value
      linux_os_config {
        swap_file_size_mb = 100
      }
    }
  }
  identity {
    type = "SystemAssigned"
  }
  tags = {
    Environment = "Production"
  }
}
`,
			expectedHCL: `
resource "azurerm_kubernetes_cluster" "example" {
  name                = "example-aks1"
  location            = azurerm_resource_group.example.location
  resource_group_name = azurerm_resource_group.example.name
  dns_prefix          = "exampleaks1"
  dynamic "default_node_pool" {
    for_each = ["Standard_D2_v2"]
    content {
      name       = "default"
      node_count = 1
      linux_os_config {
        swap_file_size_mb = 100
      }
      sku = default_node_pool.value
    }
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
			desc: "Rename multiple nested block attributes",
			cfg: `
transform "rename_attribute" this {
	rename {
		resource_type  = "azurerm_virtual_network"
		attribute_path = ["subnet", "address_prefix"]
		new_name       = "cidr"
	}
}
`,
			tfCfg: vnetResourceTf,
			expectedHCL: `
resource "azurerm_virtual_network" "example" {
  name                = "example-network"
  location            = azurerm_resource_group.example.location
  resource_group_name = azurerm_resource_group.example.name
  address_space       = ["10.0.0.0/16"]
  dns_servers         = ["10.0.0.4", "10.0.0.5"]

  subnet {
    name = "subnet1"
    cidr = "10.0.1.0/24"
  }

  subnet {
    name           = "subnet2"
    security_group = azurerm_network_security_group.example.id
    cidr           = "10.0.2.0/24"
  }

  tags = {
    environment = "Production"
  }
}
`,
		},
		{
			desc: "Rename attribute in data block",
			cfg: `
transform "rename_attribute" this {
	rename {
		resource_type  = "data.azurerm_resource_group"
		attribute_path = ["location"]
		new_name       = "region"
	}
}
`,
			tfCfg: resourceGroupDataSourceTf,
			expectedHCL: `
data "azurerm_resource_group" "this" {
  region = "westus"
}
`,
		},
		{
			desc: "Rename attribute in multiple dest blocks",
			cfg: `
transform "rename_attribute" this {
	rename {
		resource_type  = "data.azurerm_resource_group"
		attribute_path = ["location"]
		new_name       = "region"
	}
}
`,
			tfCfg: `
data "azurerm_resource_group" "this" {
  location = "westus"
}

data "azurerm_resource_group" "that" {
  location = "eastus"
}
`,
			expectedHCL: `
data "azurerm_resource_group" "this" {
  region = "westus"
}

data "azurerm_resource_group" "that" {
  region = "eastus"
}
`,
		},
		{
			desc: "Rename attribute with multiple nested levels",
			cfg: `
transform "rename_attribute" this {
	rename {
		resource_type  = "azurerm_kubernetes_cluster"
		attribute_path = ["default_node_pool", "linux_os_config", "swap_file_size_mb"]
		new_name       = "swap_file_size_in_mb"
	}
}
`,
			tfCfg: aksResourceTf,
			expectedHCL: `
resource "azurerm_kubernetes_cluster" "example" {
  name                = "example-aks1"
  location            = azurerm_resource_group.example.location
  resource_group_name = azurerm_resource_group.example.name
  dns_prefix          = "exampleaks1"
  default_node_pool {
    name       = "default"
    node_count = 1
    vm_size    = "Standard_D2_v2"
    linux_os_config {
      swap_file_size_in_mb = 100
    }
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
			desc: "Rename non-existent attribute",
			cfg: `
transform "rename_attribute" this {
	rename {
		resource_type  = "azurerm_kubernetes_cluster"
		attribute_path = ["non_existent"]
		new_name       = "new_name"
	}
}
`,
			tfCfg:       aksResourceTf,
			expectedHCL: aksResourceTf,
		},
		{
			desc: "type mismatch",
			cfg: `
transform "rename_attribute" this {
	rename {
		resource_type  = "azurerm_kubernetes_cluster"
		attribute_path = ["non_existent"]
		new_name       = "new_name"
	}
}
`,
			tfCfg:       resourceGroupDataSourceTf,
			expectedHCL: resourceGroupDataSourceTf,
		},
	}

	for _, c := range cases {
		t.Run(c.desc, func(t *testing.T) {
			fs := fakeFs(map[string]string{
				"/main.tf":           c.tfCfg,
				"/cfg/main.mptf.hcl": c.cfg,
			})
			stub := gostub.Stub(&filesystem.Fs, fs)
			defer stub.Reset()
			hclBlocks, err := pkg.LoadMPTFHclBlocks(false, "/cfg")
			cfg, err := pkg.NewMetaProgrammingTFConfig(&pkg.TerraformModuleRef{
				Dir:    "/",
				AbsDir: "/",
			}, nil, hclBlocks, nil, context.TODO())
			require.NoError(t, err)
			plan, err := pkg.RunMetaProgrammingTFPlan(cfg)
			require.NoError(t, err)
			require.NoError(t, plan.Apply())
			tfFile, err := afero.ReadFile(fs, "/main.tf")
			require.NoError(t, err)
			actual := string(tfFile)
			assert.Equal(t, c.expectedHCL, actual)
		})
	}
}

//func TestAttributeRegex(t *testing.T) {
//	resourceType := "azurerm_resource_group"
//	attribute := "location"
//	regex := fmt.Sprintf(pkg.RenamePattern, resourceType, attribute)
//	re := regexp.MustCompile(regex)
//	inputs := []string{
//		"azurerm_resource_group.this.location",
//		"azurerm_resource_group.that.location",
//		"azurerm_resource_group.this[0].location",
//		`azurerm_resource_group.this[coalesce(var.index, "hello")].location`,
//		"azurerm_resource_group.this.\nlocation",
//		"azurerm_resource_group.this.\r\nlocation",
//		"azurerm_resource_group.\nthis.location",
//		"azurerm_resource_group.\r\nthis.location",
//		"azurerm_resource_group.\nthis.\r\nlocation",
//		"azurerm_resource_group.\r\nthis.\r\nlocation",
//	}
//	for _, input := range inputs {
//		assert.MatchRegex(t, input, re)
//	}
//}
