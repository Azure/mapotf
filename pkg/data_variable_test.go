package pkg_test

import (
	"context"
	"testing"

	"github.com/Azure/golden"
	"github.com/Azure/mapotf/pkg"
	filesystem "github.com/Azure/mapotf/pkg/fs"
	"github.com/prashantv/gostub"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zclconf/go-cty/cty"
)

func TestDataVariable_ExecuteDuringPlan(t *testing.T) {
	cases := []struct {
		desc                 string
		tfCode               string
		variableName         string
		variableType         string
		expectNoMatch        bool
		expectedVariableName string
		expectedAttributes   map[string]cty.Value
	}{
		{
			desc: "single variable without type filter",
			tfCode: `
variable "example_var" {
  type    = string
  default = "value"
}`,
			expectedVariableName: "example_var",
			expectedAttributes: map[string]cty.Value{
				"type":    cty.StringVal("string"),
				"default": cty.StringVal(`"value"`),
			},
		},
		{
			desc: "filter by variable name",
			tfCode: `
		variable "my_var" {
		 type    = string
		 default = "hello"
		}
		
		variable "other_var" {
		 type    = number
		 default = 42
		}
		`,
			variableName:         "my_var",
			expectedVariableName: "my_var",
			expectedAttributes: map[string]cty.Value{
				"type":    cty.StringVal("string"),
				"default": cty.StringVal(`"hello"`),
			},
		},
		{
			desc: "filter by variable type",
			tfCode: `
		variable "var1" {
		 type = string
		}
		
		variable "var2" {
		 type = number
		}
		`,
			variableType:         "number",
			expectedVariableName: "var2",
			expectedAttributes: map[string]cty.Value{
				"type": cty.StringVal("number"),
			},
		},
		{
			desc: "no match",
			tfCode: `
		variable "var1" {
		 type = string
		}
		`,
			expectNoMatch: true,
			variableType:  "number",
			expectedAttributes: map[string]cty.Value{
				"type": cty.StringVal("number"),
			},
		},
	}

	for _, c := range cases {
		t.Run(c.desc, func(t *testing.T) {
			stub := gostub.Stub(&filesystem.Fs, fakeFs(map[string]string{
				"/main.tf": c.tfCode,
			}))
			defer stub.Reset()

			cfg, err := pkg.NewMetaProgrammingTFConfig(&pkg.TerraformModuleRef{
				Dir:    "/",
				AbsDir: "/",
			}, nil, nil, nil, context.TODO())
			require.NoError(t, err)

			data := &pkg.DataVariable{
				BaseBlock:        golden.NewBaseBlock(cfg, nil),
				BaseData:         &pkg.BaseData{},
				ExpectedNameName: c.variableName,
				ExpectedType:     c.variableType,
			}

			err = data.ExecuteDuringPlan()
			require.NoError(t, err)

			result := data.Result
			if c.expectNoMatch {
				assert.Equal(t, cty.ObjectVal(make(map[string]cty.Value)), result)
				return
			}
			object, ok := result.AsValueMap()[c.expectedVariableName]
			require.True(t, ok)
			for k, expected := range c.expectedAttributes {
				attr := object.GetAttr(k)
				assert.Equal(t, expected, attr)
			}
		})
	}
}
