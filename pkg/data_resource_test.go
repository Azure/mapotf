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

func TestResourceData_QueryResourceBlocks(t *testing.T) {
	cases := []struct {
		desc       string
		tfCode     string
		useForEach bool
		useCount   bool
		expected   cty.Value
	}{
		{
			desc: "only one resource block without count or for_each",
			tfCode: `resource "fake_resource" this {
	id = 123
}`,
			expected: cty.MapVal(map[string]cty.Value{
				"fake_resource": cty.MapVal(map[string]cty.Value{
					"resource.fake_resource.this": cty.ObjectVal(map[string]cty.Value{
						"id": cty.StringVal("123"),
					}),
				}),
			}),
		},
		{
			desc: "count",
			tfCode: `
resource "fake_resource" this {}
resource "fake_resource" that {
  count = 2
}
`,
			useCount: true,
			expected: cty.MapVal(map[string]cty.Value{
				"fake_resource": cty.MapVal(map[string]cty.Value{
					"resource.fake_resource.that": cty.ObjectVal(map[string]cty.Value{
						"count": cty.StringVal("2"),
					}),
				}),
			}),
		},
		{
			desc: "for_each",
			tfCode: `
resource "fake_resource" this {}
resource "fake_resource" that {
  for_each = toset([1,2,3])
}
`,
			useForEach: true,
			expected: cty.MapVal(map[string]cty.Value{
				"fake_resource": cty.MapVal(map[string]cty.Value{
					"resource.fake_resource.that": cty.ObjectVal(map[string]cty.Value{
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

			// Use the config to create a ResourceData object
			data := &pkg.ResourceData{
				BaseBlock:    golden.NewBaseBlock(cfg, nil),
				ResourceType: "fake_resource",
				UseCount:     c.useCount,
				UseForEach:   c.useForEach,
			}

			err = data.ExecuteDuringPlan()
			require.NoError(t, err)

			result := golden.Value(data)

			expected := map[string]cty.Value{
				"resource_type": cty.StringVal("fake_resource"),
				"use_count":     cty.BoolVal(c.useCount),
				"use_for_each":  cty.BoolVal(c.useForEach),
				"result":        c.expected,
			}
			assert.Equal(t, expected, result)
		})
	}
}

func TestResourceData_CustomizedToStringShouldContainsAllFields(t *testing.T) {
	stub := gostub.Stub(&filesystem.Fs, fakeFs(map[string]string{
		"/main.tf": `resource "fake_resource" this {
	id = 123
}`,
	}))
	defer stub.Reset()
	cfg, err := pkg.NewMetaProgrammingTFConfig(&pkg.TerraformModuleRef{
		Dir:    "/",
		AbsDir: "/",
	}, nil, nil, nil, context.TODO())
	require.NoError(t, err)

	data := &pkg.ResourceData{
		BaseBlock:    golden.NewBaseBlock(cfg, nil),
		ResourceType: "fake_resource",
		UseCount:     false,
		UseForEach:   false,
	}

	err = data.ExecuteDuringPlan()
	require.NoError(t, err)

	var sut map[string]any
	err = json.Unmarshal([]byte(data.String()), &sut)
	require.NoError(t, err)
	assert.Contains(t, sut, "resource_type")
	assert.Contains(t, sut, "use_count")
	assert.Contains(t, sut, "use_for_each")
	assert.Contains(t, sut, "result")
}
