package pkg_test

import (
	"context"
	"strings"
	"testing"

	"github.com/Azure/golden"
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/lonegunmanb/hclfuncs"
	"github.com/lonegunmanb/mptf/pkg"
	"github.com/lonegunmanb/mptf/pkg/terraform"
	"github.com/prashantv/gostub"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zclconf/go-cty/cty"
)

func TestUpdateInPlaceTransform_Decode(t *testing.T) {
	cases := []struct {
		desc               string
		cfg                string
		expectedPatchBlock string
	}{
		{
			desc: "pure string",
			cfg: `
transform "update_in_place" this {
	target_block_address = "resource.fake_resource.this"
	tags = "{ hello = world }"
}
`,
			expectedPatchBlock: `patch {
	tags = { hello = world }
}`,
		},
		{
			desc: "function call",
			cfg: `
transform "update_in_place" this {
	target_block_address = "resource.fake_resource.this"
	tags = "\"${join("-", ["foo", "bar", "baz"])}\""
}
`,
			expectedPatchBlock: `patch {
	tags = "foo-bar-baz"
}`,
		},
	}
	for _, c := range cases {
		t.Run(c.desc, func(t *testing.T) {
			stub := gostub.Stub(&terraform.Fs, fakeFs(map[string]string{
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
			cfg, err := pkg.NewMetaProgrammingTFConfig("/", "", context.TODO())
			require.NoError(t, err)
			sut := &pkg.UpdateInPlaceTransform{
				BaseBlock: golden.NewBaseBlock(cfg, hclBlock),
			}
			err = sut.Decode(hclBlock, &hcl.EvalContext{
				Variables: map[string]cty.Value{},
				Functions: hclfuncs.Functions("."),
			})
			require.NoError(t, err)
			updateBlock := sut.UpdateBlock()

			actual := string(updateBlock.BuildTokens(hclwrite.Tokens{}).Bytes())
			assert.Equal(t, formatHcl(c.expectedPatchBlock), formatHcl(actual))
		})
	}
}

func formatHcl(inputHcl string) string {
	return strings.TrimSuffix(string(hclwrite.Format([]byte(inputHcl))), "\n")
}
