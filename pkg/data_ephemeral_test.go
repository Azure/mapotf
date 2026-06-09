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

func TestEphemeralData_QueryEphemeralBlocks(t *testing.T) {
	cases := []struct {
		desc       string
		tfCode     string
		useForEach bool
		useCount   bool
		expected   cty.Value
	}{
		{
			desc: "only one ephemeral block without count or for_each",
			tfCode: `ephemeral "fake_ephemeral" this {
	id = 123
}`,
			expected: cty.ObjectVal(map[string]cty.Value{
				"fake_ephemeral": cty.ObjectVal(map[string]cty.Value{
					"this": cty.ObjectVal(map[string]cty.Value{
						"id": cty.NumberIntVal(123),
					}),
				}),
			}),
		},
		{
			desc: "count",
			tfCode: `
ephemeral "fake_ephemeral" this {}
ephemeral "fake_ephemeral" that {
  count = 2
}
`,
			useCount: true,
			expected: cty.ObjectVal(map[string]cty.Value{
				"fake_ephemeral": cty.ObjectVal(map[string]cty.Value{
					"that": cty.ObjectVal(map[string]cty.Value{
						"count": cty.NumberIntVal(2),
					}),
				}),
			}),
		},
		{
			desc: "for_each",
			tfCode: `
ephemeral "fake_ephemeral" this {}
ephemeral "fake_ephemeral" that {
  for_each = toset([1,2,3])
}
`,
			useForEach: true,
			expected: cty.ObjectVal(map[string]cty.Value{
				"fake_ephemeral": cty.ObjectVal(map[string]cty.Value{
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

			data := &pkg.EphemeralData{
				BaseBlock:     golden.NewBaseBlock(cfg, nil),
				EphemeralType: "fake_ephemeral",
				UseCount:      c.useCount,
				UseForEach:    c.useForEach,
			}

			err = data.ExecuteDuringPlan()
			require.NoError(t, err)

			result := golden.Value(data)

			expected := map[string]cty.Value{
				"ephemeral_type": cty.StringVal("fake_ephemeral"),
				"use_count":      cty.BoolVal(c.useCount),
				"use_for_each":   cty.BoolVal(c.useForEach),
				"result":         c.expected,
			}
			assertCtyMapRawEquals(t, expected, result)
		})
	}
}

func TestEphemeralData_CustomizedToStringShouldContainsAllFields(t *testing.T) {
	stub := gostub.Stub(&filesystem.Fs, fakeFs(map[string]string{
		"/main.tf": `ephemeral "fake_ephemeral" this {
	id = 123
}`,
	}))
	defer stub.Reset()
	cfg, err := pkg.NewMetaProgrammingTFConfig(&pkg.TerraformModuleRef{
		Dir:    "/",
		AbsDir: "/",
	}, nil, nil, nil, context.TODO())
	require.NoError(t, err)

	data := &pkg.EphemeralData{
		BaseBlock:     golden.NewBaseBlock(cfg, nil),
		EphemeralType: "fake_ephemeral",
		UseCount:      false,
		UseForEach:    false,
	}

	err = data.ExecuteDuringPlan()
	require.NoError(t, err)

	var sut map[string]any
	err = json.Unmarshal([]byte(data.String()), &sut)
	require.NoError(t, err)
	assert.Contains(t, sut, "ephemeral_type")
	assert.Contains(t, sut, "use_count")
	assert.Contains(t, sut, "use_for_each")
	assert.Contains(t, sut, "result")
}

// TestEphemeralData_RootBlockAddressLookup pins the address-prefix routing in
// MetaProgrammingTFConfig.RootBlock: an "ephemeral.<type>.<name>" address must
// resolve to the corresponding ephemeral RootBlock so transforms like
// move_block can target it.
func TestEphemeralData_RootBlockAddressLookup(t *testing.T) {
	stub := gostub.Stub(&filesystem.Fs, fakeFs(map[string]string{
		"/main.tf": `ephemeral "fake_ephemeral" "this" {
  id = 123
}`,
	})).Stub(&terraform.RootBlockReflectionInformation, func(map[string]cty.Value, *terraform.RootBlock) {})
	defer stub.Reset()

	cfg, err := pkg.NewMetaProgrammingTFConfig(&pkg.TerraformModuleRef{
		Dir:    "/",
		AbsDir: "/",
	}, nil, nil, nil, context.TODO())
	require.NoError(t, err)

	block := cfg.RootBlock("ephemeral.fake_ephemeral.this")
	require.NotNil(t, block, "ephemeral.fake_ephemeral.this should be addressable via RootBlock")
	assert.Equal(t, "ephemeral", block.Type)
	assert.Equal(t, []string{"fake_ephemeral", "this"}, block.Labels)
}
