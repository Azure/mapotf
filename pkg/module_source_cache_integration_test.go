package pkg

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/hashicorp/terraform-config-inspect/tfconfig"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMain(m *testing.M) {
	if os.Getenv("MAPOTF_TEST_FAKE_TERRAFORM") == "1" && len(os.Args) > 1 && os.Args[1] == "get" {
		if err := runModuleSourceFakeTerraform(); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func runModuleSourceFakeTerraform() error {
	marker, err := os.CreateTemp(os.Getenv("MAPOTF_TEST_TERRAFORM_CALLS"), "get-")
	if err != nil {
		return err
	}
	if err := marker.Close(); err != nil {
		return err
	}
	mode := os.Getenv("MAPOTF_TEST_TERRAFORM_MODE")
	if mode == "error" {
		return fmt.Errorf("fixture terraform get failure")
	}
	wrapper, diags := tfconfig.LoadModule(".")
	if diags.HasErrors() {
		return diags
	}
	call := wrapper.ModuleCalls["x"]
	if call == nil {
		return fmt.Errorf("missing synthetic module call")
	}
	dataDir := os.Getenv("TF_DATA_DIR")
	if dataDir == "" {
		dataDir = ".terraform"
	}
	moduleDir := filepath.Join(dataDir, "modules", "x")
	if err := os.MkdirAll(moduleDir, 0o700); err != nil {
		return err
	}
	content, err := os.ReadFile(filepath.Join(os.Getenv("MAPOTF_TEST_TERRAFORM_FIXTURE"), "main.tf"))
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(moduleDir, "main.tf"), content, 0o600); err != nil {
		return err
	}
	selected := TerraformModuleRef{Key: "x", Source: call.Source, Version: call.Version, Dir: moduleDir}
	if key, ok := moduleSourceCacheIdentity(selected.Source, selected.Version); ok {
		selected.Source = key.source
	}
	switch mode {
	case "wrong_source":
		selected.Source = "registry.terraform.io/other/module/azure"
	case "wrong_version":
		selected.Version = "9.9.9"
	case "missing_manifest":
		return nil
	}
	manifest, err := json.Marshal(struct {
		Modules []TerraformModuleRef `json:"Modules"`
	}{[]TerraformModuleRef{selected}})
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dataDir, "modules", "modules.json"), manifest, 0o600); err != nil {
		return err
	}
	if mode == "partial" {
		return fmt.Errorf("fixture partial download or wrapper validation failure")
	}
	return nil
}

func setupModuleSourceFakeTerraform(t *testing.T) {
	t.Helper()
	exe, err := os.Executable()
	require.NoError(t, err)
	lookup := "which"
	suffix := ""
	if runtime.GOOS == "windows" {
		lookup = "where"
		suffix = ".exe"
	}
	lookupPath, err := exec.LookPath(lookup)
	require.NoError(t, err)
	binDir := t.TempDir()
	copyExecutable := func(source, name string) {
		t.Helper()
		input, err := os.Open(source)
		require.NoError(t, err)
		output, err := os.OpenFile(filepath.Join(binDir, name+suffix), os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0o700)
		require.NoError(t, err)
		_, err = io.Copy(output, input)
		require.NoError(t, err)
		require.NoError(t, output.Close())
		require.NoError(t, input.Close())
	}
	copyExecutable(exe, "terraform")
	copyExecutable(lookupPath, lookup)
	t.Setenv("PATH", binDir)
	t.Setenv("MAPOTF_TEST_FAKE_TERRAFORM", "1")
	t.Setenv("MAPOTF_TEST_TERRAFORM_MODE", "")
	t.Setenv("MAPOTF_TEST_TERRAFORM_CALLS", t.TempDir())
	t.Setenv("MAPOTF_TEST_TERRAFORM_FIXTURE", moduleSourceFixture(t).Path)
}

func moduleSourceFakeTerraformCalls(t *testing.T) int {
	t.Helper()
	files, err := filepath.Glob(filepath.Join(os.Getenv("MAPOTF_TEST_TERRAFORM_CALLS"), "get-*"))
	require.NoError(t, err)
	return len(files)
}

func newModuleSourceDataFixture() *ModuleSourceData {
	return &ModuleSourceData{
		BaseData: &BaseData{}, Source: "Azure/naming/azurerm", Version: "0.4.2",
	}
}

func TestModuleSourceDataCacheReusesTerraformInspection(t *testing.T) {
	setupModuleSourceFakeTerraform(t)
	isolateModuleSourceCacheEnvironment(t)
	first := newModuleSourceDataFixture()
	require.NoError(t, first.ExecuteDuringPlan())
	want := first.String()
	for range 3 {
		next := newModuleSourceDataFixture()
		require.NoError(t, next.ExecuteDuringPlan())
		assert.Equal(t, want, next.String())
	}
	assert.Equal(t, 1, moduleSourceFakeTerraformCalls(t))
	alias := newModuleSourceDataFixture()
	alias.Source = "registry.terraform.io/Azure/naming/azurerm"
	require.NoError(t, alias.ExecuteDuringPlan())
	assert.Equal(t, 1, moduleSourceFakeTerraformCalls(t))
	for _, next := range []*ModuleSourceData{
		{BaseData: &BaseData{}, Source: first.Source, Version: "0.4.3"},
		{BaseData: &BaseData{}, Source: "Azure/other/azurerm", Version: first.Version},
	} {
		require.NoError(t, next.ExecuteDuringPlan())
	}
	assert.Equal(t, 3, moduleSourceFakeTerraformCalls(t))

	for range 2 {
		fetcher := NewTerraformCliModuleSourceFetcher(t.Context())
		full, err := fetcher.Get(first.Source, first.Version, "")
		require.NoError(t, err)
		assert.Contains(t, full.ManagedResources, "fixture_resource.untouched")
	}
	assert.Equal(t, 5, moduleSourceFakeTerraformCalls(t), "the exported full-module fetcher must retain its contract")
}

func TestModuleSourceDataCacheRefreshAndDisable(t *testing.T) {
	setupModuleSourceFakeTerraform(t)
	isolateModuleSourceCacheEnvironment(t)
	data := newModuleSourceDataFixture()
	require.NoError(t, data.ExecuteDuringPlan())
	require.NoError(t, os.WriteFile(filepath.Join(os.Getenv("MAPOTF_TEST_TERRAFORM_FIXTURE"), "main.tf"), []byte(`variable "replacement" { default = true }`), 0o600))
	require.NoError(t, data.ExecuteDuringPlan())
	assert.True(t, data.Variables.Type().HasAttribute("required"))
	assert.Equal(t, 1, moduleSourceFakeTerraformCalls(t))

	t.Setenv(envModuleSourceCache, "refresh")
	require.NoError(t, data.ExecuteDuringPlan())
	assert.True(t, data.Variables.Type().HasAttribute("replacement"))
	assert.False(t, data.Variables.Type().HasAttribute("required"))
	t.Setenv(envModuleSourceCache, "on")
	require.NoError(t, data.ExecuteDuringPlan())
	assert.Equal(t, 2, moduleSourceFakeTerraformCalls(t))

	t.Setenv(envModuleSourceCache, "off")
	for range 2 {
		require.NoError(t, data.ExecuteDuringPlan())
	}
	assert.Equal(t, 4, moduleSourceFakeTerraformCalls(t))
}

func TestModuleSourceDataCacheBypassesMutableSources(t *testing.T) {
	setupModuleSourceFakeTerraform(t)
	for _, tc := range []struct{ source, version string }{
		{"Azure/naming/azurerm", ""},
		{"Azure/naming/azurerm", "~> 0.4"},
		{"Azure/naming/azurerm", "= 0.4.2"},
		{"Azure/naming/azurerm", "invalid-version"},
		{"Azure/naming/azurerm//modules/child", "0.4.2"},
		{"private.example.com/org/module/azure", "0.4.2"},
		{"git::https://example.com/module.git?ref=main", ""},
		{"https://example.com/module.zip", ""},
	} {
		t.Run(tc.source+"_"+tc.version, func(t *testing.T) {
			isolateModuleSourceCacheEnvironment(t)
			before := moduleSourceFakeTerraformCalls(t)
			for range 2 {
				data := newModuleSourceDataFixture()
				data.Source, data.Version = tc.source, tc.version
				require.NoError(t, data.ExecuteDuringPlan())
			}
			assert.Equal(t, before+2, moduleSourceFakeTerraformCalls(t))
			files, err := os.ReadDir(os.Getenv(envModuleSourceCacheDir))
			require.NoError(t, err)
			assert.Empty(t, files)
		})
	}
}

func TestModuleSourceDataCachePreservesFetchErrors(t *testing.T) {
	setupModuleSourceFakeTerraform(t)
	for _, mode := range []string{"error", "partial", "wrong_source", "wrong_version", "missing_manifest"} {
		t.Run(mode, func(t *testing.T) {
			isolateModuleSourceCacheEnvironment(t)
			t.Setenv("MAPOTF_TEST_TERRAFORM_MODE", mode)
			before := moduleSourceFakeTerraformCalls(t)
			for range 2 {
				data := newModuleSourceDataFixture()
				err := data.ExecuteDuringPlan()
				switch mode {
				case "error":
					require.ErrorContains(t, err, "terraform get")
				case "missing_manifest":
					require.ErrorContains(t, err, "module manifest")
				default:
					require.NoError(t, err)
					assert.True(t, data.Variables.Type().HasAttribute("required"))
				}
			}
			assert.Equal(t, before+2, moduleSourceFakeTerraformCalls(t))
			entries, err := filepath.Glob(filepath.Join(os.Getenv(envModuleSourceCacheDir), "*.json"))
			require.NoError(t, err)
			assert.Empty(t, entries, "failures, tolerated errors and mismatched selections must never populate the cache")
		})
	}
}

func TestModuleSourceDataWarmCacheDoesNotHideChangedCLIContext(t *testing.T) {
	setupModuleSourceFakeTerraform(t)
	for _, name := range []string{"TF_CLI_CONFIG_FILE", "TF_TOKEN_registry_terraform_io", "TF_CLI_ARGS_get", "TF_DATA_DIR"} {
		t.Run(name, func(t *testing.T) {
			isolateModuleSourceCacheEnvironment(t)
			data := newModuleSourceDataFixture()
			require.NoError(t, data.ExecuteDuringPlan())
			before := moduleSourceFakeTerraformCalls(t)
			t.Setenv(name, "changed-context")
			t.Setenv("MAPOTF_TEST_TERRAFORM_MODE", "error")
			require.ErrorContains(t, data.ExecuteDuringPlan(), "terraform get")
			assert.Equal(t, before+1, moduleSourceFakeTerraformCalls(t))
		})
	}
}

func TestModuleSourceDataLocalEditsRemainVisible(t *testing.T) {
	isolateModuleSourceCacheEnvironment(t)
	t.Setenv("PATH", "")
	base := t.TempDir()
	dir := filepath.Join(base, "local")
	require.NoError(t, os.Mkdir(dir, 0o700))
	path := filepath.Join(dir, "main.tf")
	require.NoError(t, os.WriteFile(path, []byte(`variable "before" {}`), 0o600))
	data := &ModuleSourceData{BaseData: &BaseData{}, Source: "./local", BaseDir: base}
	require.NoError(t, data.ExecuteDuringPlan())
	assert.True(t, data.Variables.Type().HasAttribute("before"))
	require.NoError(t, os.WriteFile(path, []byte("variable \"after\" { default = null }\noutput \"new\" { value = true }\n"), 0o600))
	require.NoError(t, data.ExecuteDuringPlan())
	assert.False(t, data.Variables.Type().HasAttribute("before"))
	assert.True(t, data.Variables.Type().HasAttribute("after"))
	assert.True(t, data.Outputs.Type().HasAttribute("new"))
	assert.Zero(t, data.RequiredVariables.LengthInt())
	assert.Equal(t, 1, data.OptionalVariables.LengthInt())
	require.NoError(t, os.WriteFile(path, []byte(`variable "`), 0o600))
	require.ErrorContains(t, data.ExecuteDuringPlan(), "error loading local module")
	files, err := os.ReadDir(os.Getenv(envModuleSourceCacheDir))
	require.NoError(t, err)
	assert.Empty(t, files)
}
