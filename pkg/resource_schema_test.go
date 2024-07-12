package pkg_test

import (
	"context"
	"testing"

	"github.com/Azure/mapotf/pkg"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTerraformCliProviderSchemaRetriever_retrieveLocalProviderSchema(t *testing.T) {
	sut := pkg.NewTerraformCliProviderSchemaRetriever(context.Background())
	schema, err := sut.Get("hashicorp/local", "2.5.1")
	require.NoError(t, err)
	assert.Contains(t, schema.ResourceSchemas, "local_file")
	assert.Contains(t, schema.ResourceSchemas, "local_sensitive_file")
	assert.Contains(t, schema.DataSourceSchemas, "local_file")
	assert.Contains(t, schema.DataSourceSchemas, "local_sensitive_file")
}
