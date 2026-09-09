package pkg_test

import (
	"context"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/Azure/mapotf/pkg"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
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
	t.Setenv("PATH", "")
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

func TestTerraformCliModuleSourceFetcher_RemoteSourceLocalGit(t *testing.T) {
	for _, tool := range []string{"terraform", "git"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("Skipping test because %s is not available on PATH", tool)
		}
	}

	cases := []struct {
		name        string
		subdir      string
		rootDecoy   bool
		dataDir     string
		emptyModule bool
		missing     bool
		wantError   string
	}{
		{name: "subdirectory_without_root_tf", subdir: "nested/subdir"},
		{name: "subdirectory_with_root_decoy", subdir: "nested/subdir", rootDecoy: true},
		{name: "root_module"},
		{name: "relative_data_dir", subdir: "nested/subdir", dataDir: "relative"},
		{name: "absolute_data_dir", subdir: "nested/subdir", dataDir: "absolute"},
		{name: "empty_subdirectory_with_decoys", subdir: "nested/subdir", rootDecoy: true, emptyModule: true, wantError: "no .tf files"},
		{name: "missing_subdirectory_with_root_decoy", subdir: "missing/subdir", rootDecoy: true, missing: true, wantError: "terraform get"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dataDir := ""
			switch tc.dataDir {
			case "relative":
				dataDir = "terraform-data"
			case "absolute":
				dataDir = t.TempDir()
			}
			t.Setenv("TF_DATA_DIR", dataDir)

			repoDir := filepath.Join(t.TempDir(), "module repository")
			require.NoError(t, os.Mkdir(repoDir, 0o700))
			repo, err := git.PlainInit(repoDir, false)
			require.NoError(t, err)
			worktree, err := repo.Worktree()
			require.NoError(t, err)
			commit := func() string {
				t.Helper()
				require.NoError(t, worktree.AddWithOptions(&git.AddOptions{All: true}))
				hash, err := worktree.Commit("module fixture", &git.CommitOptions{
					Author: &object.Signature{
						Name:  "Mapotf tests",
						Email: "mapotf@example.invalid",
						When:  time.Unix(0, 0),
					},
				})
				require.NoError(t, err)
				return hash.String()
			}

			moduleDir := filepath.Join(repoDir, filepath.FromSlash(tc.subdir))
			if !tc.missing {
				require.NoError(t, os.MkdirAll(moduleDir, 0o700))
				if tc.emptyModule {
					deeperDir := filepath.Join(moduleDir, "unrelated")
					require.NoError(t, os.Mkdir(deeperDir, 0o700))
					require.NoError(t, os.WriteFile(filepath.Join(deeperDir, "main.tf"), []byte(`variable "deeper_decoy" {}`), 0o600))
				} else {
					require.NoError(t, os.WriteFile(filepath.Join(moduleDir, "main.tf"), []byte(`
variable "required_input" {
  type = string
}
variable "optional_input" {
  default = "default"
}
output "selected_module" {
  value = var.required_input
}
`), 0o600))
				}
			}
			if tc.rootDecoy {
				require.NoError(t, os.WriteFile(filepath.Join(repoDir, "decoy.tf"), []byte(`variable "root_decoy" {}`), 0o600))
			}
			ref := commit()
			if tc.wantError == "" {
				require.NoError(t, os.WriteFile(filepath.Join(moduleDir, "main.tf"), []byte(`variable "wrong_revision" {}`), 0o600))
				commit()
			}

			repoURL := url.URL{Scheme: "file", Path: "/" + strings.TrimPrefix(filepath.ToSlash(repoDir), "/")}
			source := "git::" + repoURL.String()
			if tc.subdir != "" {
				source += "//" + tc.subdir
			}
			source += "?ref=" + ref
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			sut := pkg.NewTerraformCliModuleSourceFetcher(ctx)
			mod, err := sut.Get(source, "", "")
			if tc.wantError != "" {
				require.ErrorContains(t, err, tc.wantError)
				assert.Nil(t, mod)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, mod)
			assert.Len(t, mod.Variables, 2)
			require.Contains(t, mod.Variables, "required_input")
			assert.True(t, mod.Variables["required_input"].Required)
			require.Contains(t, mod.Variables, "optional_input")
			assert.False(t, mod.Variables["optional_input"].Required)
			assert.Contains(t, mod.Outputs, "selected_module")
			assert.NotContains(t, mod.Variables, "root_decoy")
			assert.NotContains(t, mod.Variables, "wrong_revision")
		})
	}
}
