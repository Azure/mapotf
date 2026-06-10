package pkg

import (
	"context"
	"fmt"
	"github.com/hashicorp/terraform-exec/tfexec"
	tfjson "github.com/hashicorp/terraform-json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

type TerraformProviderSchemaRetriever interface {
	Get(providerSource, versionConstraint string) (*tfjson.ProviderSchema, error)
}

type TerraformCliProviderSchemaRetriever struct {
	ctx context.Context
}

func NewTerraformCliProviderSchemaRetriever(ctx context.Context) TerraformProviderSchemaRetriever {
	return TerraformCliProviderSchemaRetriever{ctx: ctx}
}

func (t TerraformCliProviderSchemaRetriever) Get(providerSource, versionConstraint string) (*tfjson.ProviderSchema, error) {
	tmpFolder, err := os.MkdirTemp("", "*")
	if err != nil {
		return nil, fmt.Errorf("error creating temp TF code folder: %s", err)
	}
	defer func() {
		_ = os.RemoveAll(tmpFolder)
	}()

	tfProviderCode := fmt.Sprintf(`
terraform {
  required_providers {
    provider = {
      source = "%s"
      version = "%s"
    }
  }
}
`, providerSource, versionConstraint)

	err = os.WriteFile(filepath.Join(tmpFolder, "main.tf"), []byte(tfProviderCode), 0600)
	if err != nil {
		return nil, fmt.Errorf("error writing temp TF code file: %s", err)
	}

	execPath, err := t.getTerraformPath()
	if err != nil {
		return nil, err
	}
	workingDir := tmpFolder
	tf, err := tfexec.NewTerraform(workingDir, execPath)
	if err != nil {
		return nil, fmt.Errorf("error running NewTerraform: %w", err)
	}

	err = tf.Init(t.ctx, tfexec.Upgrade(true))
	if err != nil {
		return nil, fmt.Errorf("error running Init: %s", err)
	}
	schema, err := tf.ProvidersSchema(t.ctx)
	if err != nil {
		return nil, fmt.Errorf("error running providers: %w", err)
	}
	// Look the provider up by its fully-qualified source name in the schema
	// JSON. Delegated to a pure helper so the lookup behaviour (case
	// normalisation, fallback, error on miss) is unit-testable without
	// requiring the Terraform CLI on PATH.
	return lookupProviderSchema(schema.Schemas, providerSource, versionConstraint)
}

// lookupProviderSchema resolves a provider schema by source within the map
// produced by `terraform providers schema -json`. Terraform normalises
// provider source namespaces to lowercase in that output (registry
// namespaces are case-insensitive identifiers per the registry protocol),
// so the lookup lowercases `providerSource` before searching. On miss it
// returns a real error rather than `(nil, nil)` so config-layer callers
// surface the failure instead of dereferencing nil downstream.
func lookupProviderSchema(schemas map[string]*tfjson.ProviderSchema, providerSource, versionConstraint string) (*tfjson.ProviderSchema, error) {
	lowered := strings.ToLower(providerSource)
	src := fmt.Sprintf("registry.terraform.io/%s", lowered)
	if r, ok := schemas[src]; ok && r != nil {
		return r, nil
	}
	// Fall back to a direct lookup on the lowercased source for providers
	// whose schema key already includes a non-default hostname or is
	// otherwise not prefixed with `registry.terraform.io/`.
	if r, ok := schemas[lowered]; ok && r != nil {
		return r, nil
	}
	return nil, fmt.Errorf("provider schema %q not found; ensure `terraform init` succeeds for source %q version %q", src, providerSource, versionConstraint)
}

func (t TerraformCliProviderSchemaRetriever) getTerraformPath() (string, error) {
	var cmd *exec.Cmd

	if t.isWindows() {
		cmd = exec.Command("where", "terraform")
	} else {
		cmd = exec.Command("which", "terraform")
	}

	out, err := cmd.Output()
	if err != nil {
		return "", err
	}

	path := strings.TrimSpace(string(out))
	return path, nil
}

func (t TerraformCliProviderSchemaRetriever) isWindows() bool {
	return runtime.GOOS == "windows"
}
