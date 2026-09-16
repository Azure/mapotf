package pkg

import (
	"encoding/json"
	"testing"

	"github.com/Azure/golden"
	"github.com/Azure/mapotf/pkg/terraform"
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zclconf/go-cty/cty"
)

func TestUpdateInPlaceTransform_String(t *testing.T) {
	// Initialize a UpdateInPlaceTransform instance
	updateBlock := hclwrite.NewBlock("patch", []string{})
	u := &UpdateInPlaceTransform{
		BaseBlock:          golden.NewBaseBlock(nil, nil),
		TargetBlockAddress: "resource.fake_resource.this",
		updateBlock:        updateBlock,
	}

	// Call the String() method
	result := u.String()

	// Parse the result as JSON
	var parsed map[string]interface{}
	err := json.Unmarshal([]byte(result), &parsed)
	require.NoError(t, err)
	assert.Equal(t, u.Id(), parsed["id"])
	assert.Equal(t, u.TargetBlockAddress, parsed["target_block_address"])
	assert.Equal(t, `patch{
}
`, parsed["patch"])
}

func TestUpdateInPlaceTransform_ObjectMergeValidatesBeforeMutation(t *testing.T) {
	for _, second := range []string{
		`metadata = local.unsafe`,
		`metadata = { (var.key) = "unsafe" }`,
	} {
		t.Run(second, func(t *testing.T) {
			source := `block {
  metadata = { source = "old", aliases = [azapi.primary] }
  nested {
    ` + second + `
  }
}`
			read, diag := hclsyntax.ParseConfig([]byte(source), "target.tf", hcl.InitialPos)
			require.False(t, diag.HasErrors(), diag.Error())
			write, diag := hclwrite.ParseConfig([]byte(source), "target.tf", hcl.InitialPos)
			require.False(t, diag.HasErrors(), diag.Error())
			dest := terraform.NewBlock(nil, read.Body.(*hclsyntax.Body).Blocks[0], write.Body().Blocks()[0])
			patch, diag := hclwrite.ParseConfig([]byte(`patch {
  metadata = { source = "new" }
  nested {
    metadata = { source = "new" }
  }
}`), "patch", hcl.InitialPos)
			require.False(t, diag.HasErrors(), diag.Error())
			before := string(dest.WriteBlock.BuildTokens(nil).Bytes())
			u := &UpdateInPlaceTransform{MergeObjectAttributes: true}
			require.ErrorContains(t, u.PatchWriteBlock(dest, patch.Body().Blocks()[0]), "nested block")
			require.Equal(t, before, string(dest.WriteBlock.BuildTokens(nil).Bytes()))
		})
	}
}

func TestUpdateInPlaceTransform_ObjectMergeRejectsUnknownFlag(t *testing.T) {
	source := []byte(`transform "update_in_place" this {
  target_block_address = "terraform"
  merge_object_attributes = var.enabled
}`)
	read, diag := hclsyntax.ParseConfig(source, "rule", hcl.InitialPos)
	require.False(t, diag.HasErrors(), diag.Error())
	write, diag := hclwrite.ParseConfig(source, "rule", hcl.InitialPos)
	require.False(t, diag.HasErrors(), diag.Error())
	block := golden.NewHclBlock(read.Body.(*hclsyntax.Body).Blocks[0], write.Body().Blocks()[0], nil)
	u := new(UpdateInPlaceTransform)
	err := u.Decode(block, &hcl.EvalContext{
		Variables: map[string]cty.Value{
			"var": cty.ObjectVal(map[string]cty.Value{"enabled": cty.UnknownVal(cty.Bool)}),
		},
	})
	require.ErrorContains(t, err, "must be a known, non-null bool")
}
