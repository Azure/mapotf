package pkg

import (
	tfjson "github.com/hashicorp/terraform-json"
)

// LookupProviderSchemaForTest is a test-only re-export of the package-private
// lookupProviderSchema helper so external _test packages can exercise the
// case-normalisation and miss-handling behaviour without standing up the
// Terraform CLI.
func LookupProviderSchemaForTest(schemas map[string]*tfjson.ProviderSchema, providerSource, versionConstraint string) (*tfjson.ProviderSchema, error) {
	return lookupProviderSchema(schemas, providerSource, versionConstraint)
}
