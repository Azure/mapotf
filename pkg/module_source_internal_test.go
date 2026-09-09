package pkg

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/hashicorp/terraform-config-inspect/tfconfig"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDownloadedModuleDir(t *testing.T) {
	cases := []struct {
		name        string
		moduleDir   string
		dataDir     string
		absoluteDir bool
	}{
		{name: "root_module", moduleDir: ".terraform/modules/x"},
		{name: "nested_module", moduleDir: ".terraform/modules/x/nested/subdir"},
		{name: "absolute_module", absoluteDir: true},
		{name: "relative_data_dir", moduleDir: "terraform-data/modules/x/nested/subdir", dataDir: "relative"},
		{name: "absolute_data_dir_relative_module", moduleDir: "downloaded/nested/subdir", dataDir: "absolute"},
		{name: "absolute_data_dir_absolute_module", dataDir: "absolute", absoluteDir: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			workDir := t.TempDir()
			dataDir := ""
			switch tc.dataDir {
			case "relative":
				dataDir = "terraform-data"
			case "absolute":
				dataDir = t.TempDir()
			}
			t.Setenv("TF_DATA_DIR", dataDir)
			moduleDir := tc.moduleDir
			want := filepath.Join(workDir, filepath.FromSlash(moduleDir))
			if tc.absoluteDir {
				moduleDir = t.TempDir()
				want = moduleDir
			}
			manifestDir := dataDir
			if manifestDir == "" {
				manifestDir = ".terraform"
			}
			if !filepath.IsAbs(manifestDir) {
				manifestDir = filepath.Join(workDir, manifestDir)
			}
			manifestDir = filepath.Join(manifestDir, "modules")
			require.NoError(t, os.MkdirAll(manifestDir, 0o700))
			content, err := json.Marshal(map[string]interface{}{
				"Modules": []TerraformModuleRef{
					{Key: "x.child", Dir: ".terraform/modules/x/child"},
					{Key: "", Dir: "."},
					{Key: "x", Source: "example/module/provider//nested/subdir", Version: "1.2.3", Dir: moduleDir},
					{Key: "other", Dir: ".terraform/modules/other"},
				},
			})
			require.NoError(t, err)
			require.NoError(t, os.WriteFile(filepath.Join(manifestDir, "modules.json"), content, 0o600))

			got, err := downloadedModuleDir(workDir)
			require.NoError(t, err)
			assert.Equal(t, want, got)
		})
	}
}

func TestDownloadedModuleDirInvalidManifest(t *testing.T) {
	t.Setenv("TF_DATA_DIR", "")
	cases := []struct {
		name    string
		content string
		want    string
		syntax  bool
		typeErr bool
	}{
		{name: "malformed_json", content: `{"Modules":`, want: "cannot decode", syntax: true},
		{name: "trailing_json", content: `{"Modules":[]} {}`, want: "cannot decode", syntax: true},
		{name: "invalid_modules_type", content: `{"Modules":{}}`, want: "cannot decode", typeErr: true},
		{name: "invalid_dir_type", content: `{"Modules":[{"Key":"x","Dir":42}]}`, want: "cannot decode", typeErr: true},
		{name: "invalid_key_type", content: `{"Modules":[{"Key":42,"Dir":"path"}]}`, want: "cannot decode", typeErr: true},
		{name: "missing_modules", content: `{}`, want: `no entry for module "x"`},
		{name: "null_manifest", content: `null`, want: `no entry for module "x"`},
		{name: "null_modules", content: `{"Modules":null}`, want: `no entry for module "x"`},
		{name: "empty_modules", content: `{"Modules":[]}`, want: `no entry for module "x"`},
		{name: "null_entry", content: `{"Modules":[null]}`, want: `no entry for module "x"`},
		{name: "root_and_child_only", content: `{"Modules":[{"Key":"","Dir":"."},{"Key":"x.child","Dir":"child"}]}`, want: `no entry for module "x"`},
		{name: "missing_dir", content: `{"Modules":[{"Key":"x"}]}`, want: `empty Dir for module "x"`},
		{name: "empty_dir", content: `{"Modules":[{"Key":"x","Dir":""}]}`, want: `empty Dir for module "x"`},
		{name: "null_dir", content: `{"Modules":[{"Key":"x","Dir":null}]}`, want: `empty Dir for module "x"`},
		{name: "duplicate_x", content: `{"Modules":[{"Key":"x","Dir":"first"},{"Key":"x","Dir":"second"}]}`, want: `multiple entries for module "x"`},
		{name: "duplicate_identical_x", content: `{"Modules":[{"Key":"x","Dir":"same"},{"Key":"x","Dir":"same"}]}`, want: `multiple entries for module "x"`},
		{name: "duplicate_empty_x", content: `{"Modules":[{"Key":"x","Dir":""},{"Key":"x","Dir":"second"}]}`, want: `multiple entries for module "x"`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			workDir := t.TempDir()
			manifestPath := writeModuleSourceManifest(t, workDir, tc.content)
			got, err := downloadedModuleDir(workDir)
			require.ErrorContains(t, err, tc.want)
			assert.ErrorContains(t, err, strconv.Quote(manifestPath))
			assert.Empty(t, got)
			if tc.syntax {
				var cause *json.SyntaxError
				assert.ErrorAs(t, err, &cause)
			}
			if tc.typeErr {
				var cause *json.UnmarshalTypeError
				assert.ErrorAs(t, err, &cause)
			}
		})
	}
}

func TestDownloadedModuleDirUnreadableManifest(t *testing.T) {
	t.Setenv("TF_DATA_DIR", "")
	t.Run("missing", func(t *testing.T) {
		workDir := t.TempDir()
		_, err := downloadedModuleDir(workDir)
		require.ErrorContains(t, err, "cannot read module manifest")
		assert.ErrorIs(t, err, os.ErrNotExist)
		assert.ErrorContains(t, err, strconv.Quote(filepath.Join(workDir, ".terraform", "modules", "modules.json")))
	})
	t.Run("directory", func(t *testing.T) {
		workDir := t.TempDir()
		manifestPath := filepath.Join(workDir, ".terraform", "modules", "modules.json")
		require.NoError(t, os.MkdirAll(manifestPath, 0o700))
		_, err := downloadedModuleDir(workDir)
		require.ErrorContains(t, err, "cannot read module manifest")
		var cause *os.PathError
		assert.ErrorAs(t, err, &cause)
	})
}

func TestLoadDownloadedModule(t *testing.T) {
	t.Setenv("TF_DATA_DIR", "")
	getErr := errors.New("terraform get wrapper input validation failed")
	for _, cliErr := range []error{nil, getErr} {
		name := "successful_get"
		if cliErr != nil {
			name = "failed_get"
		}
		t.Run(name, func(t *testing.T) {
			cases := []struct {
				name        string
				manifest    string
				missingDir  bool
				fileDir     bool
				emptyModule bool
				invalidTF   bool
				want        string
			}{
				{name: "valid", manifest: `{"Modules":[{"Key":"x","Dir":"downloaded"}]}`},
				{name: "missing_manifest", want: "cannot read module manifest"},
				{name: "malformed_manifest", manifest: `{`, want: "cannot decode module manifest"},
				{name: "no_matching_module", manifest: `{"Modules":[{"Key":"","Dir":"."}]}`, want: `no entry for module "x"`},
				{name: "missing_directory", manifest: `{"Modules":[{"Key":"x","Dir":"downloaded"}]}`, missingDir: true, want: "cannot read downloaded module directory"},
				{name: "directory_is_file", manifest: `{"Modules":[{"Key":"x","Dir":"downloaded"}]}`, fileDir: true, want: "cannot read downloaded module directory"},
				{name: "empty_module_with_deeper_decoy", manifest: `{"Modules":[{"Key":"x","Dir":"downloaded"}]}`, emptyModule: true, want: "no .tf files"},
				{name: "invalid_module", manifest: `{"Modules":[{"Key":"x","Dir":"downloaded"}]}`, invalidTF: true, want: "error loading module"},
			}
			for _, tc := range cases {
				t.Run(tc.name, func(t *testing.T) {
					workDir := t.TempDir()
					require.NoError(t, os.WriteFile(filepath.Join(workDir, "main.tf"), []byte(`variable "wrapper_decoy" {}`), 0o600))
					legacyDir := filepath.Join(workDir, ".terraform", "modules", "x")
					require.NoError(t, os.MkdirAll(legacyDir, 0o700))
					require.NoError(t, os.WriteFile(filepath.Join(legacyDir, "main.tf"), []byte(`variable "root_decoy" {}`), 0o600))
					if tc.manifest != "" {
						writeModuleSourceManifest(t, workDir, tc.manifest)
					}
					moduleDir := filepath.Join(workDir, "downloaded")
					if tc.fileDir {
						require.NoError(t, os.WriteFile(moduleDir, []byte("not a directory"), 0o600))
					} else if !tc.missingDir {
						require.NoError(t, os.Mkdir(moduleDir, 0o700))
						switch {
						case tc.emptyModule:
							require.NoError(t, os.Mkdir(filepath.Join(moduleDir, "deeper"), 0o700))
							require.NoError(t, os.WriteFile(filepath.Join(moduleDir, "deeper", "main.tf"), []byte(`variable "nested_decoy" {}`), 0o600))
							require.NoError(t, os.Mkdir(filepath.Join(moduleDir, "not-a-file.tf"), 0o700))
						case tc.invalidTF:
							require.NoError(t, os.WriteFile(filepath.Join(moduleDir, "main.tf"), []byte(`variable "`), 0o600))
						default:
							require.NoError(t, os.WriteFile(filepath.Join(moduleDir, "main.tf"), []byte(`variable "required_input" {}`), 0o600))
						}
					}

					mod, err := loadDownloadedModule(workDir, cliErr)
					if tc.want == "" {
						require.NoError(t, err)
						require.NotNil(t, mod)
						assert.Len(t, mod.Variables, 1)
						require.Contains(t, mod.Variables, "required_input")
						assert.True(t, mod.Variables["required_input"].Required)
						return
					}
					require.ErrorContains(t, err, tc.want)
					assert.Nil(t, mod)
					if cliErr != nil {
						assert.ErrorIs(t, err, getErr)
					}
					if tc.missingDir || tc.manifest == "" {
						assert.ErrorIs(t, err, os.ErrNotExist)
					}
					if tc.fileDir {
						var cause *os.PathError
						assert.ErrorAs(t, err, &cause)
					}
					if tc.invalidTF {
						var cause tfconfig.Diagnostics
						assert.ErrorAs(t, err, &cause)
					}
					if tc.manifest == "{" {
						var cause *json.SyntaxError
						assert.ErrorAs(t, err, &cause)
					}
				})
			}
		})
	}
}

func writeModuleSourceManifest(t *testing.T, workDir, content string) string {
	t.Helper()
	manifestDir := filepath.Join(workDir, ".terraform", "modules")
	require.NoError(t, os.MkdirAll(manifestDir, 0o700))
	manifestPath := filepath.Join(manifestDir, "modules.json")
	require.NoError(t, os.WriteFile(manifestPath, []byte(content), 0o600))
	return manifestPath
}
