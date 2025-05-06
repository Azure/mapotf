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

func TestDataOutput_ExecuteDuringPlan(t *testing.T) {
	cases := []struct {
		desc               string
		tfCode             string
		expectedOutputName string
		expectedNotFound   bool
		expectedValue      cty.Value
	}{
		{
			desc: "single output without filter",
			tfCode: `
output "example_output" {
  value = "example_value"
}`,
			expectedOutputName: "example_output",
			expectedValue:      cty.StringVal(`"example_value"`),
		},
		{
			desc: "filter by output name",
			tfCode: `
		output "output_one" {
		 value = "value_one"
		}
		
		output "output_two" {
		 value = "value_two"
		}`,
			expectedOutputName: "output_two",
			expectedValue:      cty.StringVal(`"value_two"`),
		},
		{
			desc: "no matching output",
			tfCode: `
		output "existing_output" {
		 value = "existing_value"
		}`,
			expectedOutputName: "non_existing_output",
			expectedNotFound:   true,
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

			data := &pkg.DataOutput{
				BaseBlock:          golden.NewBaseBlock(cfg, nil),
				BaseData:           &pkg.BaseData{},
				ExpectedOutputName: c.expectedOutputName,
			}

			err = data.ExecuteDuringPlan()
			require.NoError(t, err)
			if c.expectedNotFound {
				//data.Result.l
				assert.Equal(t, cty.ObjectVal(make(map[string]cty.Value)), data.Result)
			} else {
				outputBlock := data.Result.GetAttr(c.expectedOutputName)
				assert.Equal(t, c.expectedValue, outputBlock.GetAttr("value"))
			}

		})
	}
}
