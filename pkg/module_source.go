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

// TerraformModuleSourceFetcher fetches a Terraform module and returns its
// parsed metadata via terraform-config-inspect.
//
// For local sources (./, ../, absolute paths) the fetcher resolves the source
// against baseDir and loads it directly — no terraform invocation, no temp
// folder. baseDir is required in this case and is normally auto-defaulted by
// the calling data block to the target module's directory.
//
// For remote sources (registry shortcuts, git URLs, etc.) the fetcher writes
// a synthetic wrapper into a temp folder and runs `terraform get` to download
// the module. `terraform get` also validates the wrapper against the target
// module's required inputs; that validation error is tolerated as long as the
// download itself succeeded, because terraform-config-inspect only needs the
// downloaded `.tf` files to parse variable and output declarations.
//
// baseDir is ignored for remote sources but is still expected on every call
// so the data block layer can auto-default it uniformly.
type TerraformModuleSourceFetcher interface {
	Get(source, version, baseDir string) (*tfconfig.Module, error)
}

type TerraformCliModuleSourceFetcher struct {
	ctx context.Context
}

func NewTerraformCliModuleSourceFetcher(ctx context.Context) TerraformModuleSourceFetcher {
	return TerraformCliModuleSourceFetcher{ctx: ctx}
}

func (t TerraformCliModuleSourceFetcher) Get(source, version, baseDir string) (*tfconfig.Module, error) {
	if isLocalSource(source) {
		return loadLocalModule(source, baseDir)
	}
	return t.fetchRemoteModule(source, version)
}

// loadLocalModule resolves a local source (./, ../, absolute) against baseDir
// and parses the module directly via terraform-config-inspect. No terraform
// CLI invocation.
func loadLocalModule(source, baseDir string) (*tfconfig.Module, error) {
	if baseDir == "" {
		return nil, fmt.Errorf("cannot resolve local module source %q without base_dir", source)
	}
	resolved := source
	if !filepath.IsAbs(resolved) {
		resolved = filepath.Join(baseDir, source)
	}
	info, err := os.Stat(resolved)
	if err != nil {
		return nil, fmt.Errorf("cannot stat local module source %q (resolved to %q): %w", source, resolved, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("local module source %q (resolved to %q) is not a directory", source, resolved)
	}
	mod, diags := tfconfig.LoadModule(resolved)
	if diags.HasErrors() {
		return nil, fmt.Errorf("error loading local module from %s: %s", resolved, diags.Error())
	}
	return mod, nil
}

// fetchRemoteModule downloads a remote module via `terraform get` and parses
// it via terraform-config-inspect. Tolerates `terraform get` validation
// errors (missing required args on the synthetic wrapper) as long as the
// download itself succeeded.
func (t TerraformCliModuleSourceFetcher) fetchRemoteModule(source, version string) (*tfconfig.Module, error) {
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
	getErr := tf.Get(t.ctx)

	moduleDir := filepath.Join(tmpFolder, ".terraform", "modules", "x")
	if hasTerraformFiles(moduleDir) {
		mod, diags := tfconfig.LoadModule(moduleDir)
		if diags.HasErrors() {
			return nil, fmt.Errorf("error loading module from %s: %s", moduleDir, diags.Error())
		}
		// Download succeeded — any `terraform get` validation error is
		// irrelevant because terraform-config-inspect only reads the
		// downloaded module's variable and output declarations.
		return mod, nil
	}

	if getErr != nil {
		return nil, fmt.Errorf("error running terraform get for module %q version %q: %w", source, version, getErr)
	}
	return nil, fmt.Errorf("terraform get completed for module %q version %q but no .tf files were downloaded to %s", source, version, moduleDir)
}

// isLocalSource reports whether source refers to a module on the local
// filesystem (and therefore must be resolved against the caller's base_dir
// rather than fetched via terraform get). Matches the same set of prefixes
// Terraform itself treats as local: `./`, `../`, and absolute paths
// (including Windows `C:\foo` and `C:/foo`).
func isLocalSource(source string) bool {
	if source == "" {
		return false
	}
	if strings.HasPrefix(source, "./") || strings.HasPrefix(source, "../") {
		return true
	}
	// Also catch `.\foo` and `..\foo` on Windows.
	if strings.HasPrefix(source, `.\`) || strings.HasPrefix(source, `..\`) {
		return true
	}
	if source == "." || source == ".." {
		return true
	}
	return filepath.IsAbs(source)
}

// hasTerraformFiles reports whether dir contains at least one .tf file.
// `.tf` files always live at module root in standard layouts so a one-level
// scan is sufficient.
func hasTerraformFiles(dir string) bool {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if strings.HasSuffix(e.Name(), ".tf") {
			return true
		}
	}
	return false
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

