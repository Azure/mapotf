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

func TestUpdateInPlaceTransform_SingleMergeBlockParse(t *testing.T) {
	cases := []struct {
		name   string
		source string
		error  string
	}{
		{"normal", `patch { value = local.expression }`, ""},
		{"nested", "patch {\n nested { value = [azapi.primary] }\n}", ""},
		{"malformed", "patch {\n value =\n}", "single-block.tf:"},
		{"empty", "", "exactly one HCL"},
		{"no block", `value = "not a block"`, "exactly one HCL"},
		{"extra block", "patch {}\nextra {}", "exactly one HCL"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			read, readDiag := hclsyntax.ParseConfig([]byte(tc.source), "single-block.tf", hcl.InitialPos)
			readBlock, readErr := singleMergeSyntaxBlock(read, readDiag)
			write, writeDiag := hclwrite.ParseConfig([]byte(tc.source), "single-block.tf", hcl.InitialPos)
			writeBlock, writeErr := singleMergeWriteBlock(write, writeDiag)
			if tc.error != "" {
				require.ErrorContains(t, readErr, tc.error)
				require.Nil(t, readBlock)
				require.ErrorContains(t, writeErr, tc.error)
				require.Nil(t, writeBlock)
				if readDiag.HasErrors() {
					require.Equal(t, readDiag, readErr)
				}
				if writeDiag.HasErrors() {
					require.Equal(t, writeDiag, writeErr)
				}
				return
			}
			require.NoError(t, readErr)
			require.NoError(t, writeErr)
			require.NotNil(t, readBlock)
			require.NotNil(t, writeBlock)
			require.Equal(t, "patch", readBlock.Type)
			require.Equal(t, "patch", writeBlock.Type())
			require.Equal(t, tc.source, string(writeBlock.BuildTokens(nil).Bytes()))
		})
	}
}

func TestUpdateInPlaceTransform_SingleMergeSyntaxBlockShape(t *testing.T) {
	validBlock := &hclsyntax.Block{Type: "patch", Body: &hclsyntax.Body{}}
	cases := []struct {
		name  string
		file  *hcl.File
		error string
	}{
		{"nil file", nil, "non-nil HCL syntax file"},
		{"missing body", &hcl.File{}, "non-nil HCL syntax body"},
		{"wrong body type", &hcl.File{Body: hcl.EmptyBody()}, "non-nil HCL syntax body"},
		{"typed nil body", &hcl.File{Body: (*hclsyntax.Body)(nil)}, "non-nil HCL syntax body"},
		{"missing block", &hcl.File{Body: &hclsyntax.Body{}}, "exactly one HCL syntax block"},
		{"nil block", &hcl.File{Body: &hclsyntax.Body{Blocks: hclsyntax.Blocks{nil}}}, "non-nil HCL syntax block and body"},
		{"missing block body", &hcl.File{Body: &hclsyntax.Body{Blocks: hclsyntax.Blocks{&hclsyntax.Block{}}}}, "non-nil HCL syntax block and body"},
		{"valid block", &hcl.File{Body: &hclsyntax.Body{Blocks: hclsyntax.Blocks{validBlock}}}, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			block, err := singleMergeSyntaxBlock(tc.file, nil)
			if tc.error != "" {
				require.ErrorContains(t, err, tc.error)
				require.Nil(t, block)
				return
			}
			require.NoError(t, err)
			require.Same(t, validBlock, block)
		})
	}
}

func TestUpdateInPlaceTransform_SingleMergeWriteBlockShape(t *testing.T) {
	t.Run("nil file", func(t *testing.T) {
		block, err := singleMergeWriteBlock(nil, nil)
		require.ErrorContains(t, err, "non-nil HCL write file")
		require.Nil(t, block)
	})
	t.Run("empty file", func(t *testing.T) {
		block, err := singleMergeWriteBlock(hclwrite.NewEmptyFile(), nil)
		require.ErrorContains(t, err, "exactly one HCL write block")
		require.Nil(t, block)
	})
	t.Run("valid block", func(t *testing.T) {
		file := hclwrite.NewEmptyFile()
		want := file.Body().AppendNewBlock("patch", nil)
		block, err := singleMergeWriteBlock(file, nil)
		require.NoError(t, err)
		require.Same(t, want, block)
	})
}
