package pkg_test

import (
	"context"
	"github.com/Azure/golden"
	"github.com/Azure/mapotf/pkg"
	filesystem "github.com/Azure/mapotf/pkg/fs"
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/prashantv/gostub"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestRemoveNestedBlock_singlePath(t *testing.T) {
	mptfCfg := `
transform "remove_nested_block" this {
  target_block_address = "resource.fake_resource.this"
  paths = ["nested_block"]
}
`
	stub := gostub.Stub(&filesystem.Fs, fakeFs(map[string]string{
		"/main.tf": `
resource "fake_resource" this {
  nested_block {}
  non_target_block {}
}
`,
	}))
	defer stub.Reset()

	readFile, diag := hclsyntax.ParseConfig([]byte(mptfCfg), "test.hcl", hcl.InitialPos)
	require.Falsef(t, diag.HasErrors(), diag.Error())
	writeFile, diag := hclwrite.ParseConfig([]byte(mptfCfg), "test.hcl", hcl.InitialPos)
	require.Falsef(t, diag.HasErrors(), diag.Error())
	hclBlock := golden.NewHclBlock(readFile.Body.(*hclsyntax.Body).Blocks[0], writeFile.Body().Blocks()[0], nil)
	cfg, err := pkg.NewMetaProgrammingTFConfig(&pkg.TerraformModuleRef{
		Dir:    "/",
		AbsDir: "/",
	}, nil, []*golden.HclBlock{hclBlock}, nil, context.TODO())
	require.NoError(t, err)
	plan, err := pkg.RunMetaProgrammingTFPlan(cfg)
	require.NoError(t, err)
	err = plan.Apply()
	require.NoError(t, err)
	after, err := afero.ReadFile(filesystem.Fs, "/main.tf")
	require.NoError(t, err)
	expected := formatHcl(`
resource "fake_resource" this {
  non_target_block {}
}
`)
	actual := formatHcl(string(after))
	assert.Equal(t, expected, actual)
}
