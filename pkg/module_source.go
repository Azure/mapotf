package pkg

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/hashicorp/terraform-config-inspect/tfconfig"
	"github.com/hashicorp/terraform-exec/tfexec"
)

// TerraformModuleSourceFetcher fetches a remote Terraform module (registry
// reference, git URL, etc.) and returns its parsed metadata via
// terraform-config-inspect. Backed by `terraform get`, so only modules are
// downloaded — no provider plugins.
type TerraformModuleSourceFetcher interface {
	Get(source, version string) (*tfconfig.Module, error)
}

type TerraformCliModuleSourceFetcher struct {
	ctx context.Context
}

func NewTerraformCliModuleSourceFetcher(ctx context.Context) TerraformModuleSourceFetcher {
	return TerraformCliModuleSourceFetcher{ctx: ctx}
}

func (t TerraformCliModuleSourceFetcher) Get(source, version string) (*tfconfig.Module, error) {
	tmpFolder, err := os.MkdirTemp("", "mapotf-module-*")
	if err != nil {
		return nil, fmt.Errorf("error creating temp module folder: %s", err)
	}
	defer func() {
		_ = os.RemoveAll(tmpFolder)
	}()

	var versionLine string
	if version != "" {
		versionLine = fmt.Sprintf("  version = %q\n", version)
	}
	tfCode := fmt.Sprintf(`module "x" {
  source = %q
%s}
`, source, versionLine)
	if err := os.WriteFile(filepath.Join(tmpFolder, "main.tf"), []byte(tfCode), 0600); err != nil {
		return nil, fmt.Errorf("error writing temp TF code file: %s", err)
	}

	execPath, err := t.getTerraformPath()
	if err != nil {
		return nil, err
	}
	tf, err := tfexec.NewTerraform(tmpFolder, execPath)
	if err != nil {
		return nil, fmt.Errorf("error running NewTerraform: %w", err)
	}
	if err := tf.Get(t.ctx); err != nil {
		return nil, fmt.Errorf("error running terraform get for module %q version %q: %w", source, version, err)
	}

	moduleDir := filepath.Join(tmpFolder, ".terraform", "modules", "x")
	mod, diags := tfconfig.LoadModule(moduleDir)
	if diags.HasErrors() {
		return nil, fmt.Errorf("error loading module from %s: %s", moduleDir, diags.Error())
	}
	return mod, nil
}

func (t TerraformCliModuleSourceFetcher) getTerraformPath() (string, error) {
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

func (t TerraformCliModuleSourceFetcher) isWindows() bool {
	return runtime.GOOS == "windows"
}
