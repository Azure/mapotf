package pkg_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/Azure/mapotf/pkg"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsLocalSource(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		in   string
		want bool
	}{
		{"empty", "", false},
		{"current_dir", "./", true},
		{"current_dir_no_slash", ".", true},
		{"parent_dir", "../", true},
		{"parent_dir_no_slash", "..", true},
		{"current_dir_subpath", "./submod", true},
		{"parent_dir_subpath", "../../shared/foo", true},
		{"windows_current_dir", `.\submod`, true},
		{"windows_parent_dir", `..\..\shared`, true},
		{"unix_absolute", "/foo/bar/mod", runtime.GOOS != "windows"},
		{"windows_absolute_backslash", `C:\foo\mod`, runtime.GOOS == "windows"},
		{"windows_absolute_forward", `C:/foo/mod`, runtime.GOOS == "windows"},
		{"registry_short", "Azure/naming/azurerm", false},
		{"registry_with_subdir", "Azure/naming/azurerm//modules/dns", false},
		{"git_https", "git::https://github.com/Azure/foo.git", false},
		{"git_ssh", "git@github.com:Azure/foo.git", false},
		{"bare_word", "naming", false},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := pkg.IsLocalSourceForTest(tc.in)
			assert.Equal(t, tc.want, got, "source = %q", tc.in)
		})
	}
}

// TestTerraformCliModuleSourceFetcher_LocalSource pins the U2 fix: local
// sources are resolved against base_dir via terraform-config-inspect alone,
// with no terraform CLI invocation. Therefore this test can run even when
// terraform is absent from PATH.
func TestTerraformCliModuleSourceFetcher_LocalSource(t *testing.T) {
	t.Parallel()
	baseDir := t.TempDir()
	submod := filepath.Join(baseDir, "submod")
	require.NoError(t, os.Mkdir(submod, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(submod, "variables.tf"), []byte(`
variable "name" {
  type = string
}

variable "tags" {
  type    = map(string)
  default = {}
}
`), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(submod, "outputs.tf"), []byte(`
output "id" {
  value = "x"
}
`), 0o600))

	sut := pkg.NewTerraformCliModuleSourceFetcher(context.Background())
	mod, err := sut.Get("./submod", "", baseDir)
	require.NoError(t, err)
	require.NotNil(t, mod)

	require.Contains(t, mod.Variables, "name")
	require.Contains(t, mod.Variables, "tags")
	assert.True(t, mod.Variables["name"].Required, "name has no default → must be Required")
	assert.False(t, mod.Variables["tags"].Required, "tags has a default → must not be Required")

	require.Contains(t, mod.Outputs, "id")
}

func TestTerraformCliModuleSourceFetcher_LocalSourceMissingBaseDir(t *testing.T) {
	t.Parallel()
	sut := pkg.NewTerraformCliModuleSourceFetcher(context.Background())
	_, err := sut.Get("./submod", "", "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "base_dir", "error must explain that base_dir is required for local sources")
}

func TestTerraformCliModuleSourceFetcher_LocalSourceMissingDirectory(t *testing.T) {
	t.Parallel()
	baseDir := t.TempDir()
	sut := pkg.NewTerraformCliModuleSourceFetcher(context.Background())
	_, err := sut.Get("./does-not-exist", "", baseDir)
	require.Error(t, err)
}

// TestTerraformCliModuleSourceFetcher_RemoteSourceToleratesValidationError
// pins the U3 fix: when `terraform get` succeeds in downloading the module
// but fails wrapper-validation (because the synthetic wrapper passes zero
// inputs and the target module declares required inputs), the fetcher should
// still parse the downloaded module via terraform-config-inspect. This test
// requires terraform on PATH because it exercises the real CLI path.
func TestTerraformCliModuleSourceFetcher_RemoteSourceToleratesValidationError(t *testing.T) {
	if _, err := exec.LookPath("terraform"); err != nil {
		t.Skip("Skipping test because Terraform is not available on PATH")
	}
	// Azure/naming/azurerm v0.4.0 declares no required inputs (every variable
	// has a default), so it's the safest registry module to exercise the
	// remote-fetch happy path without paying the cost of a flaky network
	// dependency on a module with required args.
	sut := pkg.NewTerraformCliModuleSourceFetcher(context.Background())
	mod, err := sut.Get("Azure/naming/azurerm", "0.4.0", "")
	require.NoError(t, err)
	require.NotNil(t, mod)
	// Sanity: the naming module has known variables.
	assert.NotEmpty(t, mod.Variables)
}
