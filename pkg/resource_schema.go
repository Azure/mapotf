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
	src := fmt.Sprintf("registry.terraform.io/%s", providerSource)
	r, ok := schema.Schemas[src]
	if !ok {
		src = providerSource
		r = schema.Schemas[src]
	}

	return r, nil
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

	return strings.TrimSpace(string(out)), nil
}

func (t TerraformCliProviderSchemaRetriever) isWindows() bool {
	return runtime.GOOS == "windows"
}
