package pkg

import (
	"context"
	"encoding/json"
	"errors"
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
		return nil, fmt.Errorf("error creating temp module folder: %w", err)
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
		return nil, fmt.Errorf("error writing temp TF code file: %w", err)
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
	if getErr != nil {
		getErr = fmt.Errorf("error running terraform get: %w", getErr)
	}

	mod, err := loadDownloadedModule(tmpFolder, getErr)
	if err != nil {
		return nil, fmt.Errorf("error fetching module %q version %q: %w", source, version, err)
	}
	return mod, nil
}

func loadDownloadedModule(workDir string, getErr error) (*tfconfig.Module, error) {
	moduleDir, err := downloadedModuleDir(workDir)
	if err != nil {
		return nil, errors.Join(getErr, err)
	}
	hasFiles, err := hasTerraformFiles(moduleDir)
	if err != nil {
		return nil, errors.Join(getErr, err)
	}
	if !hasFiles {
		return nil, errors.Join(getErr, fmt.Errorf("no .tf files in downloaded module directory %q", moduleDir))
	}
	mod, diags := tfconfig.LoadModule(moduleDir)
	if diags.HasErrors() {
		return nil, errors.Join(getErr, fmt.Errorf("error loading module from %s: %w", moduleDir, diags))
	}
	// Wrapper input validation can fail even when the selected module was downloaded.
	return mod, nil
}

func downloadedModuleDir(workDir string) (string, error) {
	dataDir := os.Getenv("TF_DATA_DIR")
	if dataDir == "" {
		dataDir = ".terraform"
	}
	if !filepath.IsAbs(dataDir) {
		dataDir = filepath.Join(workDir, dataDir)
	}
	manifestPath := filepath.Join(dataDir, "modules", "modules.json")
	manifestRoot, err := os.OpenRoot(filepath.Dir(manifestPath))
	if err != nil {
		return "", fmt.Errorf("cannot read module manifest %q: %w", manifestPath, err)
	}
	defer func() {
		_ = manifestRoot.Close()
	}()
	content, err := manifestRoot.ReadFile("modules.json")
	if err != nil {
		return "", fmt.Errorf("cannot read module manifest %q: %w", manifestPath, err)
	}
	var manifest struct {
		Modules []TerraformModuleRef `json:"Modules"`
	}
	if err := json.Unmarshal(content, &manifest); err != nil {
		return "", fmt.Errorf("cannot decode module manifest %q: %w", manifestPath, err)
	}

	var moduleDir string
	matches := 0
	for _, module := range manifest.Modules {
		if module.Key == "x" {
			matches++
			moduleDir = module.Dir
		}
	}
	if matches == 0 {
		return "", fmt.Errorf("module manifest %q has no entry for module %q", manifestPath, "x")
	}
	if matches > 1 {
		return "", fmt.Errorf("module manifest %q has multiple entries for module %q", manifestPath, "x")
	}
	if moduleDir == "" {
		return "", fmt.Errorf("module manifest %q has an empty Dir for module %q", manifestPath, "x")
	}
	if !filepath.IsAbs(moduleDir) {
		moduleDir = filepath.Join(workDir, moduleDir)
	}
	return moduleDir, nil
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
func hasTerraformFiles(dir string) (bool, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false, fmt.Errorf("cannot read downloaded module directory %q: %w", dir, err)
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if strings.HasSuffix(e.Name(), ".tf") {
			return true, nil
		}
	}
	return false, nil
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
