package pkg

import (
	"encoding/json"
	"github.com/Azure/golden"
	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
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
