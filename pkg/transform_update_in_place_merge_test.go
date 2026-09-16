package pkg_test

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/Azure/mapotf/pkg"
	filesystem "github.com/Azure/mapotf/pkg/fs"
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/prashantv/gostub"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
)

func TestUpdateInPlaceTransform_MergeObjectAttributesFullPlan(t *testing.T) {
	cases := []struct {
		name     string
		flag     string
		provider string
		patch    string
		want     string
		wantErr  string
	}{
		{
			name:     "disabled by default",
			provider: `{ source = "old/azapi", configuration_aliases = [azapi.primary] }`,
			patch:    `{ source = "Azure/azapi" }`,
			want:     `{ source = "Azure/azapi" }`,
		},
		{
			name:     "explicitly disabled",
			flag:     "merge_object_attributes = false",
			provider: `{ source = "old/azapi", configuration_aliases = [azapi.primary] }`,
			patch:    `{ source = "Azure/azapi" }`,
			want:     `{ source = "Azure/azapi" }`,
		},
		{
			name:     "source only preserves version and aliases",
			flag:     "merge_object_attributes = true",
			provider: `{ source = "old/azapi", version = "~> 1.0", configuration_aliases = [azapi.primary, azapi.secondary] }`,
			patch:    `{ source = "Azure/azapi" }`,
			want:     `{ source = "Azure/azapi", version = "~> 1.0", configuration_aliases = [azapi.primary, azapi.secondary] }`,
		},
		{
			name:     "version only preserves source and aliases",
			flag:     "merge_object_attributes = true",
			provider: `{ source = "Azure/azapi", version = "~> 1.0", configuration_aliases = [azapi.primary, azapi.secondary] }`,
			patch:    `{ version = "~> 2.12" }`,
			want:     `{ source = "Azure/azapi", version = "~> 2.12", configuration_aliases = [azapi.primary, azapi.secondary] }`,
		},
		{
			name: "combined update preserves comments and quoted keys",
			flag: "merge_object_attributes = true",
			provider: `{
      # Preserve source documentation.
      "source" = "old/azapi" # Source comment.
      version = "~> 1.0"
      configuration_aliases = [
        azapi.primary, # Primary alias.
        azapi.secondary,
      ]
      extra = { nested = local.keep }
    }`,
			patch: `{ source = "Azure/azapi", "version" = "~> 2.12" }`,
			want: `{
      # Preserve source documentation.
      "source" = "Azure/azapi" # Source comment.
      version = "~> 2.12"
      configuration_aliases = [
        azapi.primary, # Primary alias.
        azapi.secondary,
      ]
      extra = { nested = local.keep }
    }`,
		},
		{
			name:     "append missing fields inline",
			flag:     "merge_object_attributes = true",
			provider: `{ configuration_aliases = [azapi.primary, azapi.secondary] }`,
			patch:    `{ source = "Azure/azapi", version = "~> 2.12" }`,
			want:     `{ configuration_aliases = [azapi.primary, azapi.secondary], source = "Azure/azapi", version = "~> 2.12" }`,
		},
		{
			name: "append missing fields multiline",
			flag: "merge_object_attributes = true",
			provider: `{
      configuration_aliases = [azapi.primary, azapi.secondary] # Keep both.
    }`,
			patch: `{ source = "Azure/azapi", version = "~> 2.12" }`,
			want: `{
      configuration_aliases = [azapi.primary, azapi.secondary] # Keep both.
      source = "Azure/azapi"
      version = "~> 2.12"
    }`,
		},
		{
			name:     "append after trailing comma",
			flag:     "merge_object_attributes = true",
			provider: `{ source = "Azure/azapi", }`,
			patch:    `{ version = "~> 2.12" }`,
			want:     `{ source = "Azure/azapi", version = "~> 2.12" }`,
		},
		{
			name:     "fill empty object",
			flag:     "merge_object_attributes = true",
			provider: `{}`,
			patch:    `{ source = "Azure/azapi", version = "~> 2.12" }`,
			want:     `{ source = "Azure/azapi", version = "~> 2.12" }`,
		},
		{
			name:     "invalid flag type",
			flag:     `merge_object_attributes = "true"`,
			provider: `{ source = "Azure/azapi" }`,
			patch:    `{ version = "~> 2.12" }`,
			wantErr:  "merge_object_attributes",
		},
		{
			name:     "null flag",
			flag:     `merge_object_attributes = null`,
			provider: `{ source = "Azure/azapi" }`,
			patch:    `{ version = "~> 2.12" }`,
			wantErr:  "merge_object_attributes",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			code := func(provider string) string {
				return fmt.Sprintf(`terraform {
  required_providers {
    azapi = %s
    random = {
      source = "hashicorp/random"
      version = "~> 3.6"
    }
  }
}`, provider)
			}
			rule := fmt.Sprintf(`data "terraform" this {}

transform "update_in_place" this {
  depends_on = [data.terraform.this]
  target_block_address = "terraform"
  %s
  asraw {
    required_providers {
      azapi = %s
    }
  }
}`, tc.flag, tc.patch)
			runObjectMergePlan(t, code(tc.provider), rule, code(tc.want), tc.wantErr)
		})
	}
}

func TestUpdateInPlaceTransform_MergeObjectAttributesSeparateUpdates(t *testing.T) {
	for _, initial := range []string{
		`terraform {
  required_providers {
    azapi = { configuration_aliases = [azapi.primary, azapi.secondary] }
  }
}`,
		`terraform {
  required_providers {}
}`,
		`terraform {}`,
	} {
		t.Run(initial, func(t *testing.T) {
			wantProvider := `{ source = "Azure/azapi", version = "~> 2.12" }`
			if initial == `terraform {
  required_providers {
    azapi = { configuration_aliases = [azapi.primary, azapi.secondary] }
  }
}` {
				wantProvider = `{ configuration_aliases = [azapi.primary, azapi.secondary], source = "Azure/azapi", version = "~> 2.12" }`
			}
			runObjectMergePlan(t, initial, `
transform "update_in_place" source {
  target_block_address = "terraform"
  merge_object_attributes = true
  asraw {
    required_providers {
      azapi = { source = "Azure/azapi" }
    }
  }
}

transform "update_in_place" version {
  depends_on = [transform.update_in_place.source]
  target_block_address = "terraform"
  merge_object_attributes = true
  asraw {
    required_providers {
      azapi = { version = "~> 2.12" }
    }
  }
}`, fmt.Sprintf(`terraform {
  required_providers {
    azapi = %s
  }
}`, wantProvider), "")
		})
	}
}

func TestUpdateInPlaceTransform_MergeObjectAttributesPatchSources(t *testing.T) {
	for name, patch := range map[string]string{
		"asraw": `asraw {
  metadata = { source = "Azure/azapi" }
  enabled = true
}`,
		"asstring": `asstring {
  metadata = "{ source = \"Azure/azapi\" }"
  enabled = "true"
}`,
		"dynamic body": `dynamic_block_body = <<-PATCH
metadata = { source = "Azure/azapi" }
enabled = true
PATCH`,
	} {
		t.Run(name, func(t *testing.T) {
			runObjectMergePlan(t, `resource "fake_resource" this {
  metadata = { source = "old/azapi", aliases = [azapi.primary] }
  enabled = false
}`, fmt.Sprintf(`transform "update_in_place" this {
  target_block_address = "resource.fake_resource.this"
  merge_object_attributes = 1 == 1
  %s
}`, patch), `resource "fake_resource" this {
  metadata = { source = "Azure/azapi", aliases = [azapi.primary] }
  enabled = true
}`, "")
		})
	}
}

func TestUpdateInPlaceTransform_MergeObjectAttributesMultipleNestedPatches(t *testing.T) {
	runObjectMergePlan(t, `terraform {}`, `
transform "update_in_place" this {
  target_block_address = "terraform"
  merge_object_attributes = true
  asraw {
    required_providers {
      azapi = { source = "Azure/azapi" }
    }
  }
  asstring {
    required_providers {
      azapi = "{ version = \"~> 2.12\" }"
    }
  }
}`, `terraform {
  required_providers {
    azapi = { source = "Azure/azapi", version = "~> 2.12" }
  }
}`, "")
}

func TestUpdateInPlaceTransform_MergeObjectAttributesUnsafeFullPlan(t *testing.T) {
	cases := []struct {
		name   string
		target string
		patch  string
		error  string
	}{
		{"nonliteral target", `local.metadata`, `asraw { metadata = { source = "new" } }`, "target must be a literal object"},
		{"unsupported target key", `{ (var.key) = "old" }`, `asraw { metadata = { source = "new" } }`, "invalid target object"},
		{"unsupported patch key", `{ source = "old" }`, `asraw { metadata = { (var.key) = "new" } }`, "invalid patch object"},
		{"conflicting patch shape", `{ source = "old" }`, `asraw { metadata = "new" }`, "non-object patch"},
		{"malformed string patch", `{ source = "old" }`, `asstring { metadata = "{ source = }" }`, "cannot parse patch expression"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			runObjectMergePlan(t, fmt.Sprintf(`resource "fake_resource" this {
  metadata = %s
}`, tc.target), fmt.Sprintf(`transform "update_in_place" this {
  target_block_address = "resource.fake_resource.this"
  merge_object_attributes = true
  %s
}`, tc.patch), "", tc.error)
		})
	}
}

func TestUpdateInPlaceTransform_MergeObjectAttributesDynamicNestedBlocks(t *testing.T) {
	source := `resource "fake_resource" this {
  dynamic "nested" {
    for_each = var.values
    iterator = entry
    content {
      metadata = { source = "old", aliases = [azapi.primary] }
      other = entry.value
    }
  }
  nested {
    metadata = { source = "old", aliases = [azapi.secondary] }
  }
}`
	want := `resource "fake_resource" this {
  dynamic "nested" {
    for_each = var.values
    iterator = entry
    content {
      metadata = { source = "new", aliases = [azapi.primary] }
      other = entry.value
    }
  }
  nested {
    metadata = { source = "new", aliases = [azapi.secondary] }
  }
}`
	runObjectMergePlan(t, source, `transform "update_in_place" this {
  target_block_address = "resource.fake_resource.this"
  merge_object_attributes = true
  asraw {
    nested {
      metadata = { source = "new" }
    }
  }
}`, want, "")
}

func TestUpdateInPlaceTransform_MergeObjectAttributesInlineBlock(t *testing.T) {
	runObjectMergePlan(t, `resource "fake_resource" this { metadata = { source = "old", aliases = [azapi.primary] } }`,
		`transform "update_in_place" this {
  target_block_address = "resource.fake_resource.this"
  merge_object_attributes = true
  asraw {
    metadata = { source = "new" }
  }
}`, `resource "fake_resource" this { metadata = { source = "new", aliases = [azapi.primary] } }`, "")
}

func TestUpdateInPlaceTransform_MergeObjectAttributesRejectsUnsafeInlineExpansion(t *testing.T) {
	runObjectMergePlan(t, `resource "fake_resource" this { metadata = { source = "old" } }`,
		`transform "update_in_place" this {
  target_block_address = "resource.fake_resource.this"
  merge_object_attributes = true
  asraw {
    metadata = { source = "new" }
    extra = true
  }
}`, "", "cannot safely apply object merge patch")
}

func runObjectMergePlan(t *testing.T, source, rule, want, wantErr string) {
	t.Helper()
	tfFile := filepath.Join("terraform", "main.tf")
	fs := fakeFs(map[string]string{
		tfFile:                                 source,
		filepath.Join("mptf", "main.mptf.hcl"): rule,
	})
	stub := gostub.Stub(&filesystem.Fs, fs)
	defer stub.Reset()

	var previous string
	for run := 0; run < 2; run++ {
		blocks, err := pkg.LoadMPTFHclBlocks(false, "mptf")
		require.NoError(t, err)
		cfg, err := pkg.NewMetaProgrammingTFConfig(&pkg.TerraformModuleRef{
			Dir: "terraform", AbsDir: "terraform",
		}, nil, blocks, nil, context.TODO())
		require.NoError(t, err)
		plan, err := pkg.RunMetaProgrammingTFPlan(cfg)
		if err == nil {
			err = plan.Apply()
		}
		if wantErr != "" {
			require.ErrorContains(t, err, wantErr)
			content, readErr := afero.ReadFile(fs, tfFile)
			require.NoError(t, readErr)
			require.Equal(t, source, string(content))
			return
		}
		require.NoError(t, err)
		content, err := afero.ReadFile(fs, tfFile)
		require.NoError(t, err)
		_, diag := hclsyntax.ParseConfig(content, tfFile, hcl.InitialPos)
		require.False(t, diag.HasErrors(), diag.Error())
		require.Equal(t, formatHcl(want), formatHcl(string(content)))
		if run > 0 {
			require.Equal(t, previous, string(content), "a second run must not change the output")
		}
		previous = string(content)
	}
}
