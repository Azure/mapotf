package pkg_test

import (
	"testing"

	"github.com/Azure/golden"
)

func TestUpdateInPlaceTransform_Decode(t *testing.T) {
	cases := []struct{
		desc string
		cfg string
		expectedPatchBlock string
	} {
		{
			desc: "pure string",
			cfg: `
transform "update_in_place" this {
	from
	tags = 
}
`
		},
	}
}