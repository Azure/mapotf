package pkg_test

import (
	"context"
	"github.com/Azure/mapotf/pkg"
	"testing"

	filesystem "github.com/Azure/mapotf/pkg/fs"
	"github.com/prashantv/gostub"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTransformEnsureLocal(t *testing.T) {
	cases := []struct {
		desc       string
		mptfConfig string
		tfConfig   string
		expectedTf string
	}{
		{
			desc: "replace value astring",
			mptfConfig: `transform "ensure_local" this{
	name = "this"
    fallback_file_name = "main.tf"
	value_as_string = "local.that"
}`,
			tfConfig: `locals {
	this = "hello"
}`,
			expectedTf: `locals {
	this = local.that
}`,
		},
		{
			desc: "replace value asraw",
			mptfConfig: `transform "ensure_local" this{
	name = "this"
    fallback_file_name = "main.tf"
	value_as_raw = local.that
}`,
			tfConfig: `locals {
	this = "hello"
}`,
			expectedTf: `locals {
	this = local.that
}`,
		},
	}
	for _, c := range cases {
		t.Run(c.desc, func(t *testing.T) {
			mockFs := fakeFs(map[string]string{
				"/main.tf":       c.tfConfig,
				"/main.mptf.hcl": c.mptfConfig,
			})
			stub := gostub.Stub(&filesystem.Fs, mockFs)
			defer stub.Reset()

			hclBlocks, err := pkg.LoadMPTFHclBlocks(false, "/")
			require.NoError(t, err)
			cfg, err := pkg.NewMetaProgrammingTFConfig(&pkg.TerraformModuleRef{
				Dir:    "/",
				AbsDir: "/",
			}, nil, hclBlocks, nil, context.TODO())
			require.NoError(t, err)
			plan, err := pkg.RunMetaProgrammingTFPlan(cfg)
			require.NoError(t, err)
			require.NoError(t, plan.Apply())

			file, err := afero.ReadFile(mockFs, "/main.tf")
			require.NoError(t, err)
			assert.Equal(t, formatHcl(c.expectedTf), formatHcl(string(file)))
		})
	}
}
