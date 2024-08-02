package pkg_test

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"testing"

	"github.com/Azure/mapotf/pkg"
	filesystem "github.com/Azure/mapotf/pkg/fs"
	"github.com/go-playground/assert/v2"
	"github.com/prashantv/gostub"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
)

const attributePattern = `%s\.(\s*\r?\n\s*)?(\w+)(\[\s*[^]]+\s*\])?(\.)(\s*\r?\n\s*)?%s`
const replPattern = "%s.${1}${2}${3}${4}${5}%s"
const sampleTfConfig = `
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

locals {
  example_location = azurerm_kubernetes_cluster.example.location
}
`

func TestRegexReplaceExpressionTransform_Apply(t *testing.T) {
	cases := []struct {
		desc        string
		cfg         string
		tfCfg       string
		expectedHCL string
	}{
		{
			desc: "Replace location with region",
			cfg: fmt.Sprintf(`
transform "regex_replace_expression" this {
  regex = "%s"
  replacement = "%s"
}
`, strings.ReplaceAll(fmt.Sprintf(attributePattern, "azurerm_kubernetes_cluster", "location"), `\`, `\\`),
				strings.ReplaceAll(fmt.Sprintf(replPattern, "azurerm_kubernetes_cluster", "region"), `${`, `$${`)),
			tfCfg: sampleTfConfig,
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

locals {
  example_location = azurerm_kubernetes_cluster.example.region
}
`,
		},
		{
			desc: "Replace location with region in another block's nested block",
			cfg: fmt.Sprintf(`
transform "regex_replace_expression" this {
  regex = "%s"
  replacement = "%s"
}
`, strings.ReplaceAll(fmt.Sprintf(attributePattern, "azurerm_kubernetes_cluster", "location"), `\`, `\\`),
				strings.ReplaceAll(fmt.Sprintf(replPattern, "azurerm_kubernetes_cluster", "region"), `${`, `$${`)),
			tfCfg: `
resource "azurerm_kubernetes_cluster" "example" {
  count = 1
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

resource "fake_resource" {
  static_nested_block {
    location = azurerm_kubernetes_cluster.example[0].location
  }
  dynamic "dynamic_block" {
    for_each = [azurerm_kubernetes_cluster.example[0].location]
    content {
	  location = dynamic_block.value
    }
  }
}
`,
			expectedHCL: `
resource "azurerm_kubernetes_cluster" "example" {
  count               = 1
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

resource "fake_resource" {
  static_nested_block {
    location = azurerm_kubernetes_cluster.example[0].region
  }
  dynamic "dynamic_block" {
    for_each = [azurerm_kubernetes_cluster.example[0].region]
    content {
      location = dynamic_block.value
    }
  }
}
`,
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
			require.NoError(t, err)
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

func TestAttributeRegex(t *testing.T) {
	resourceType := "azurerm_resource_group"
	attribute := "location"
	regex := fmt.Sprintf(attributePattern, resourceType, attribute)
	re := regexp.MustCompile(regex)
	inputs := []string{
		"azurerm_resource_group.this.location",
		"azurerm_resource_group.that.location",
		"azurerm_resource_group.this[0].location",
		`azurerm_resource_group.this[coalesce(var.index, "hello")].location`,
		"azurerm_resource_group.this.\nlocation",
		"azurerm_resource_group.this.\r\nlocation",
		"azurerm_resource_group.\nthis.location",
		"azurerm_resource_group.\r\nthis.location",
		"azurerm_resource_group.\nthis.\r\nlocation",
		"azurerm_resource_group.\r\nthis.\r\nlocation",
	}
	wanted := []string{
		"azurerm_resource_group.this.region",
		"azurerm_resource_group.that.region",
		"azurerm_resource_group.this[0].region",
		`azurerm_resource_group.this[coalesce(var.index, "hello")].region`,
		"azurerm_resource_group.this.\nregion",
		"azurerm_resource_group.this.\r\nregion",
		"azurerm_resource_group.\nthis.region",
		"azurerm_resource_group.\r\nthis.region",
		"azurerm_resource_group.\nthis.\r\nregion",
		"azurerm_resource_group.\r\nthis.\r\nregion",
	}
	for i, input := range inputs {
		assert.MatchRegex(t, input, re)
		replaced := re.ReplaceAllString(input, fmt.Sprintf(replPattern, "azurerm_resource_group", "region"))
		assert.Equal(t, wanted[i], replaced)
	}
}
