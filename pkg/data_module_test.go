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

func TestDataModule_ExecuteDuringPlan(t *testing.T) {
	cases := []struct {
		desc               string
		tfCode             string
		moduleName         string
		expectNoMatch      bool
		expectedModuleName string
		expectedAttrs      map[string]cty.Value
	}{
		{
			desc: "single_module_no_filter",
			tfCode: `
module "naming" {
  source  = "./modules/naming"
  version = "1.0.0"
}`,
			expectedModuleName: "naming",
			expectedAttrs: map[string]cty.Value{
				"source":  cty.StringVal(`./modules/naming`),
				"version": cty.StringVal(`1.0.0`),
			},
		},
		{
			desc: "filter_by_module_name",
			tfCode: `
module "first" {
  source = "./modules/first"
}

module "second" {
  source = "./modules/second"
}
`,
			moduleName:         "second",
			expectedModuleName: "second",
			expectedAttrs: map[string]cty.Value{
				"source": cty.StringVal(`./modules/second`),
			},
		},
		{
			desc: "no_match",
			tfCode: `
module "first" {
  source = "./modules/first"
}
`,
			moduleName:    "nonexistent",
			expectNoMatch: true,
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

			data := &pkg.DataModule{
				BaseBlock:          golden.NewBaseBlock(cfg, nil),
				BaseData:           &pkg.BaseData{},
				ExpectedModuleName: c.moduleName,
			}
			err = data.ExecuteDuringPlan()
			require.NoError(t, err)

			result := data.Result
			if c.expectNoMatch {
				assert.Equal(t, cty.ObjectVal(make(map[string]cty.Value)), result)
				return
			}
			object, ok := result.AsValueMap()[c.expectedModuleName]
			require.True(t, ok)
			for k, expected := range c.expectedAttrs {
				attr := object.GetAttr(k)
				assert.Equal(t, expected, attr)
			}
		})
	}
}
