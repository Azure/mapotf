package pkg_test

import (
	"context"
	"github.com/spf13/afero"
	"testing"

	"github.com/Azure/golden"
	"github.com/Azure/mapotf/pkg"
	filesystem "github.com/Azure/mapotf/pkg/fs"
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/prashantv/gostub"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConcatBlockBodyTransform_Apply(t *testing.T) {
	cases := []struct {
		desc                  string
		cfg                   string
		expectedConcatedBlock string
	}{
		{
			desc: "concatenate attributes",
			cfg: `
transform "append_block_body" this {
	target_block_address = "resource.fake_resource.this"
	block_body = "tags = { hello = world }"
}
`,
			expectedConcatedBlock: `resource "fake_resource" this {
  tags = { hello = world }
}`,
		},
		{
			desc: "concatenate nested blocks",
			cfg: `
transform "append_block_body" this {
	target_block_address = "resource.fake_resource.this"
	block_body = "nested_block {\n id = 123\n }"
}
`,
			expectedConcatedBlock: `resource "fake_resource" this {
  tags = null
  nested_block { 
    id = 123 
  }
}`,
		},
		{
			desc: "concatenate attributes and nested blocks",
			cfg: `
transform "append_block_body" this {
	target_block_address = "resource.fake_resource.this"
	block_body = "tags = {\n hello = world\n } \n nested_block {\n id = 123\n }"
}
`,
			expectedConcatedBlock: `resource "fake_resource" this {
  tags = { 
    hello = world 
  }
  nested_block { 
    id = 123 
  }
}`,
		},
	}
	for _, c := range cases {
		t.Run(c.desc, func(t *testing.T) {
			stub := gostub.Stub(&filesystem.Fs, fakeFs(map[string]string{
				"/main.tf": `
resource "fake_resource" this {
  tags = null
}`,
			}))
			defer stub.Reset()
			readFile, diag := hclsyntax.ParseConfig([]byte(c.cfg), "test.hcl", hcl.InitialPos)
			require.Falsef(t, diag.HasErrors(), diag.Error())
			writeFile, diag := hclwrite.ParseConfig([]byte(c.cfg), "test.hcl", hcl.InitialPos)
			require.Falsef(t, diag.HasErrors(), diag.Error())
			hclBlock := golden.NewHclBlock(readFile.Body.(*hclsyntax.Body).Blocks[0], writeFile.Body().Blocks()[0], nil)
			cfg, err := pkg.NewMetaProgrammingTFConfig(&pkg.TerraformModuleRef{
				Dir:    "/",
				AbsDir: "/",
			}, nil, []*golden.HclBlock{hclBlock}, nil, context.TODO())
			require.NoError(t, err)
			plan, err := pkg.RunMetaProgrammingTFPlan(cfg)
			require.NoError(t, err)
			require.NotEmpty(t, plan.String())
			require.NoError(t, plan.Apply())
			tfFile, err := afero.ReadFile(filesystem.Fs, "/main.tf")
			require.NoError(t, err)
			actual := string(tfFile)
			assert.Equal(t, formatHcl(c.expectedConcatedBlock), formatHcl(actual))
		})
	}
}
