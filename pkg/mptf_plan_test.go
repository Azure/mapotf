package pkg_test

import (
	"context"
	"github.com/Azure/mapotf/pkg"
	filesystem "github.com/Azure/mapotf/pkg/fs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"path/filepath"
	"testing"

	"github.com/prashantv/gostub"
)

func TestMetaProgrammingTFPlan_OnlyTransformThatHasTargetShouldBeInThePlan(t *testing.T) {
	gostub.Stub(&filesystem.Fs, fakeFs(map[string]string{
		filepath.Join("terraform", "main.tf"): `resource "fake_resource" this {
}`,
		filepath.Join("mptf", "main.mptf.hcl"): `data "resource" fake_resource {
  resource_type = "fake_resource"
}

data "resource" fake_resource2 {
  resource_type = "fake_resource2"
}

transform "update_in_place" fake_resource {
  for_each = try(data.resource.fake_resource.result.fake_resource, [])
  target_block_address = each.value.mptf.block_address
  id = each.value.mptf.block_address
}

transform "update_in_place" fake_resource2 {
  for_each = try(data.resource.fake_resource2.result.fake_resource2, [])
  target_block_address = each.value.mptf.block_address
  id = each.value.mptf.block_address
}
`,
	}))
	hclBlocks, err := pkg.LoadMPTFHclBlocks(false, "mptf")
	require.NoError(t, err)
	cfg, err := pkg.NewMetaProgrammingTFConfig(&pkg.TerraformModuleRef{
		Dir:    "terraform",
		AbsDir: "terraform",
	}, nil, hclBlocks, nil, context.TODO())
	require.NoError(t, err)
	plan, err := pkg.RunMetaProgrammingTFPlan(cfg)
	require.NoError(t, err)
	assert.Len(t, plan.Transforms, 1)
	assert.Equal(t, "resource.fake_resource.this", plan.Transforms[0].(*pkg.UpdateInPlaceTransform).TargetBlockAddress)
}
