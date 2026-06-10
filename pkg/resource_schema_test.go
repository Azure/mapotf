package pkg_test

import (
	"context"
	"os/exec"
	"runtime"
	"strings"
	"testing"

	"github.com/Azure/mapotf/pkg"
	tfjson "github.com/hashicorp/terraform-json"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTerraformCliProviderSchemaRetriever_retrieveLocalProviderSchema(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping test on Windows since setup Terraform on windows seems not work with this test")
	}
	if _, err := exec.LookPath("terraform"); err != nil {
		t.Skip("Skipping test because Terraform is not available on PATH")
	}
	sut := pkg.NewTerraformCliProviderSchemaRetriever(context.Background())
	schema, err := sut.Get("hashicorp/local", "2.5.1")
	require.NoError(t, err)
	assert.Contains(t, schema.ResourceSchemas, "local_file")
	assert.Contains(t, schema.ResourceSchemas, "local_sensitive_file")
	assert.Contains(t, schema.DataSourceSchemas, "local_file")
	assert.Contains(t, schema.DataSourceSchemas, "local_sensitive_file")
}

// TestLookupProviderSchema_CaseInsensitive pins the v0.1.4 fix for #101:
// `terraform providers schema -json` writes provider source namespaces
// lowercase. A `provider_source` written with registry display casing
// (e.g. `Azure/azapi`) must still resolve to the lowercased schema key.
// The pre-v0.1.4 behaviour returned (nil, nil), causing a downstream nil
// dereference; the fix returns the matching schema for lookups that differ
// only in case and a real error when no schema is found.
func TestLookupProviderSchema_CaseInsensitive(t *testing.T) {
	want := &tfjson.ProviderSchema{
		ResourceSchemas: map[string]*tfjson.Schema{
			"azapi_resource": {Block: &tfjson.SchemaBlock{}},
		},
	}
	schemas := map[string]*tfjson.ProviderSchema{
		"registry.terraform.io/azure/azapi": want,
	}

	t.Run("registry-display casing resolves to lowercase key", func(t *testing.T) {
		got, err := pkg.LookupProviderSchemaForTest(schemas, "Azure/azapi", "~> 2.0")
		require.NoError(t, err)
		assert.Same(t, want, got)
	})

	t.Run("already-lowercase source resolves", func(t *testing.T) {
		got, err := pkg.LookupProviderSchemaForTest(schemas, "azure/azapi", "~> 2.0")
		require.NoError(t, err)
		assert.Same(t, want, got)
	})

	t.Run("non-registry hostname falls back to direct lookup", func(t *testing.T) {
		alt := &tfjson.ProviderSchema{ResourceSchemas: map[string]*tfjson.Schema{}}
		altSchemas := map[string]*tfjson.ProviderSchema{
			"example.com/custom/provider": alt,
		}
		got, err := pkg.LookupProviderSchemaForTest(altSchemas, "example.com/Custom/Provider", "~> 1.0")
		require.NoError(t, err)
		assert.Same(t, alt, got)
	})

	t.Run("miss returns real error, not nil-nil", func(t *testing.T) {
		got, err := pkg.LookupProviderSchemaForTest(schemas, "does-not/exist", "~> 1.0")
		require.Error(t, err)
		assert.Nil(t, got)
		msg := err.Error()
		assert.Contains(t, msg, `"registry.terraform.io/does-not/exist"`)
		assert.Contains(t, msg, `"does-not/exist"`)
		assert.True(t, strings.Contains(msg, "terraform init"),
			"expected error to mention `terraform init`, got: %q", msg)
	})
}
