package pkg_test

import (
	"context"
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

func TestDataLocal_QueryLocalBlocks(t *testing.T) {
	cases := []struct {
		desc     string
		tfCode   string
		name     string
		expected cty.Value
	}{
		{
			desc: "single local block",
			tfCode: `locals {
  foo = "bar"
}`,
			expected: cty.ObjectVal(map[string]cty.Value{
				"foo": cty.StringVal("\"bar\""),
			}),
		},
		{
			desc: "multiple local blocks",
			tfCode: `locals {
  foo = "bar"
  baz = 123
}`,
			expected: cty.ObjectVal(map[string]cty.Value{
				"foo": cty.StringVal("\"bar\""),
				"baz": cty.StringVal("123"),
			}),
		},
		{
			desc: "filter by name",
			tfCode: `locals {
  foo = "bar"
  baz = 123
}`,
			name: "foo",
			expected: cty.ObjectVal(map[string]cty.Value{
				"foo": cty.StringVal("\"bar\""),
			}),
		},
		{
			desc: "empty locals",
			tfCode: `resource "fake_resource" "test" {
  id = 123
}`,
			expected: cty.EmptyObjectVal,
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

			// Use the config to create a DataLocal object
			data := &pkg.DataLocal{
				BaseBlock:        golden.NewBaseBlock(cfg, nil),
				ExpectedNameName: c.name,
			}

			err = data.ExecuteDuringPlan()
			require.NoError(t, err)

			result := golden.Value(data)

			expected := map[string]cty.Value{
				"name":   cty.StringVal(c.name),
				"result": c.expected,
			}
			assert.Equal(t, expected, result)
		})
	}
}

func TestDataLocal_ComplexValues(t *testing.T) {
	stub := gostub.Stub(&filesystem.Fs, fakeFs(map[string]string{
		"/main.tf": `locals {
  nested = {
    first = {
      second = "nested value"
    }
  }
}`,
	}))
	defer stub.Reset()
	cfg, err := pkg.NewMetaProgrammingTFConfig(&pkg.TerraformModuleRef{
		Dir:    "/",
		AbsDir: "/",
	}, nil, nil, nil, context.TODO())
	require.NoError(t, err)

	data := &pkg.DataLocal{
		BaseBlock: golden.NewBaseBlock(cfg, nil),
	}

	err = data.ExecuteDuringPlan()
	require.NoError(t, err)

	result := data.Result.AsValueMap()
	require.Contains(t, result, "nested")
	nested := result["nested"].AsString()
	assert.Equal(t, `{
    first = {
      second = "nested value"
    }
  }`, nested)
}
