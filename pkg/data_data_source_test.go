package pkg_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/Azure/golden"
	"github.com/Azure/mapotf/pkg"
	filesystem "github.com/Azure/mapotf/pkg/fs"
	"github.com/Azure/mapotf/pkg/terraform"
	"github.com/prashantv/gostub"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zclconf/go-cty/cty"
)

func TestDataSourceData_QueryDataBlocks(t *testing.T) {
	cases := []struct {
		desc       string
		tfCode     string
		useForEach bool
		useCount   bool
		expected   cty.Value
	}{
		{
			desc: "only one data block without count or for_each",
			tfCode: `data "fake_data" this {
	id = 123
}`,
			expected: cty.ObjectVal(map[string]cty.Value{
				"fake_data": cty.ObjectVal(map[string]cty.Value{
					"this": cty.ObjectVal(map[string]cty.Value{
						"id": cty.StringVal("123"),
					}),
				}),
			}),
		},
		{
			desc: "count",
			tfCode: `
data "fake_data" this {}
data "fake_data" that {
  count = 2
}
`,
			useCount: true,
			expected: cty.ObjectVal(map[string]cty.Value{
				"fake_data": cty.ObjectVal(map[string]cty.Value{
					"that": cty.ObjectVal(map[string]cty.Value{
						"count": cty.StringVal("2"),
					}),
				}),
			}),
		},
		{
			desc: "for_each",
			tfCode: `
data "fake_data" this {}
data "fake_data" that {
  for_each = toset([1,2,3])
}
`,
			useForEach: true,
			expected: cty.ObjectVal(map[string]cty.Value{
				"fake_data": cty.ObjectVal(map[string]cty.Value{
					"that": cty.ObjectVal(map[string]cty.Value{
						"for_each": cty.StringVal("toset([1,2,3])"),
					}),
				}),
			}),
		},
	}
	for _, c := range cases {
		t.Run(c.desc, func(t *testing.T) {
			stub := gostub.Stub(&filesystem.Fs, fakeFs(map[string]string{
				"/main.tf": c.tfCode,
			})).Stub(&terraform.RootBlockReflectionInformation, func(map[string]cty.Value, *terraform.RootBlock) {})
			defer stub.Reset()
			cfg, err := pkg.NewMetaProgrammingTFConfig(&pkg.TerraformModuleRef{
				Dir:    "/",
				AbsDir: "/",
			}, nil, nil, nil, context.TODO())
			require.NoError(t, err)

			// Use the config to create a DataSourceData object
			data := &pkg.DataSourceData{
				BaseBlock:      golden.NewBaseBlock(cfg, nil),
				DataSourceType: "fake_data",
				UseCount:       c.useCount,
				UseForEach:     c.useForEach,
			}

			err = data.ExecuteDuringPlan()
			require.NoError(t, err)

			result := golden.Value(data)

			expected := map[string]cty.Value{
				"data_source_type": cty.StringVal("fake_data"),
				"use_count":        cty.BoolVal(c.useCount),
				"use_for_each":     cty.BoolVal(c.useForEach),
				"result":           c.expected,
			}
			assert.Equal(t, expected, result)
		})
	}
}

func TestDataSourceData_CustomizedToStringShouldContainsAllFields(t *testing.T) {
	stub := gostub.Stub(&filesystem.Fs, fakeFs(map[string]string{
		"/main.tf": `data "fake_data" this {
	id = 123
}`,
	}))
	defer stub.Reset()
	cfg, err := pkg.NewMetaProgrammingTFConfig(&pkg.TerraformModuleRef{
		Dir:    "/",
		AbsDir: "/",
	}, nil, nil, nil, context.TODO())
	require.NoError(t, err)

	data := &pkg.DataSourceData{
		BaseBlock:      golden.NewBaseBlock(cfg, nil),
		DataSourceType: "fake_data",
		UseCount:       false,
		UseForEach:     false,
	}

	err = data.ExecuteDuringPlan()
	require.NoError(t, err)

	var sut map[string]any
	err = json.Unmarshal([]byte(data.String()), &sut)
	require.NoError(t, err)
	assert.Contains(t, sut, "data_source_type")
	assert.Contains(t, sut, "use_count")
	assert.Contains(t, sut, "use_for_each")
	assert.Contains(t, sut, "result")
}

func TestDataSourceData_DifferentDataSourcesHaveAttributesWithSameNameButDifferentSchema(t *testing.T) {
	stub := gostub.Stub(&filesystem.Fs, fakeFs(map[string]string{
		"/main.tf": `
data "azurerm_application_gateway" this {
  sku {
    name     = "Standard_v2"
    tier     = "Standard_v2"
    capacity = 1
  }
}

data "azurerm_public_ip" "pip" {
  sku                 = "Standard"
}

`,
	}))
	defer stub.Reset()
	cfg, err := pkg.NewMetaProgrammingTFConfig(&pkg.TerraformModuleRef{
		Dir:    "/",
		AbsDir: "/",
	}, nil, nil, nil, context.TODO())
	require.NoError(t, err)

	data := &pkg.DataSourceData{
		BaseBlock: golden.NewBaseBlock(cfg, nil),
	}

	err = data.ExecuteDuringPlan()
	require.NoError(t, err)

	var sut map[string]any
	err = json.Unmarshal([]byte(data.String()), &sut)
	require.NoError(t, err)
	assert.Contains(t, sut, "data_source_type")
	assert.Contains(t, sut, "use_count")
	assert.Contains(t, sut, "use_for_each")
	assert.Contains(t, sut, "result")
}
