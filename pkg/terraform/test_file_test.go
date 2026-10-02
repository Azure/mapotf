package terraform

import (
	"path/filepath"
	"testing"

	filesystem "github.com/Azure/mapotf/pkg/fs"
	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/prashantv/gostub"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zclconf/go-cty/cty"
)

func TestTestFileIsolation(t *testing.T) {
	fs := afero.NewMemMapFs()
	stub := gostub.Stub(&filesystem.Fs, fs)
	defer stub.Reset()
	root := filepath.Join("fixtures", "module")
	first := filepath.Join("tests", "unit", "first.tftest.hcl")
	second := filepath.Join("tests", "unit", "second.tftest.hcl")
	source := "variables { authored = true }\nmock_provider \"azapi\" {}\nrun \"plan\" { command = plan }\n"
	for _, file := range []string{first, second} {
		require.NoError(t, afero.WriteFile(fs, filepath.Join(root, file), []byte(source), 0644))
	}
	tf := "variable \"ordinary\" {}\n"
	require.NoError(t, afero.WriteFile(fs, filepath.Join(root, "main.tf"), []byte(tf), 0644))
	normal, err := LoadModule(ModuleRef{Dir: root, AbsDir: root})
	require.NoError(t, err)
	assert.Len(t, normal.Variables, 1)
	assert.Empty(t, normal.TestBlocks)

	for _, file := range []string{first, second} {
		module, err := LoadModule(ModuleRef{Dir: root, AbsDir: root, TestFile: file})
		require.NoError(t, err)
		assert.Len(t, module.Blocks(), 3)
		assert.Empty(t, module.Variables)
		assert.Equal(t, "run.plan", module.TestBlocks[2].Address)
		assert.Equal(t, file, module.TestBlocks[2].Range().Filename)
		assert.Equal(t, cty.True, module.TestBlocks[0].EvalContext().GetAttr("authored"))
		if file == first {
			module.TestBlocks[2].SetAttributeRaw("command", hclwrite.TokensForIdentifier("apply"))
			require.NoError(t, module.SaveToDisk())
		}
	}
	unchanged, err := afero.ReadFile(fs, filepath.Join(root, second))
	require.NoError(t, err)
	assert.Equal(t, source, string(unchanged))
	unchanged, err = afero.ReadFile(fs, filepath.Join(root, "main.tf"))
	require.NoError(t, err)
	assert.Equal(t, tf, string(unchanged))
}

func TestTestFileInvalidInput(t *testing.T) {
	cases := []struct {
		name, source, message string
	}{
		{"syntax", `run "x" {`, "Unclosed configuration block"},
		{"root attribute", "location = \"eastus\"\n", "top-level attributes"},
		{"ordinary terraform", `variable "x" {}`, "unsupported test block"},
		{"variables labels", `variables "x" {}`, "requires 0 labels"},
		{"duplicate variables", "variables {}\nvariables {}\n", "duplicate test block"},
		{"duplicate run", "run \"same\" {}\nrun \"same\" {}\n", "duplicate test block"},
		{"missing run name", "run {}\n", "requires 1 labels"},
		{"ambiguous run name", "run \"a.b\" {}\n", "invalid run label"},
		{"duplicate mock", "mock_provider \"azapi\" {}\nmock_provider \"azapi\" {}\n", "duplicate test block"},
		{"duplicate alias", "mock_provider \"azapi\" { alias = \"west\" }\nmock_provider \"azapi\" { alias = \"west\" }\n", "duplicate test block"},
		{"real mock collision", "provider \"azapi\" {}\nmock_provider \"azapi\" {}\n", "duplicate test block"},
		{"dynamic alias", "mock_provider \"azapi\" { alias = var.alias }\n", "provider alias must be"},
		{"invalid alias", "mock_provider \"azapi\" { alias = \"a.b\" }\n", "invalid provider alias"},
		{"nested variables", "variables {\n variables {}\n}\n", "only attributes"},
		{"multiple run variables", "run \"x\" {\n variables {}\n variables {}\n}\n", "duplicate variables"},
		{"multiple run modules", "run \"x\" {\n module { source = \"./a\" }\n module { source = \"./b\" }\n}\n", "duplicate module"},
		{"module labels", "run \"x\" {\n module \"a\" { source = \"./a\" }\n}\n", "must be unlabeled"},
		{"missing source", "run \"x\" {\n module {}\n}\n", "module source must be"},
		{"null source", "run \"x\" {\n module { source = null }\n}\n", "module source must be"},
		{"empty source", "run \"x\" {\n module { source = \"\" }\n}\n", "module source must be"},
		{"numeric source", "run \"x\" {\n module { source = 1 }\n}\n", "module source must be"},
		{"dynamic source", "run \"x\" {\n module { source = var.source }\n}\n", "module source must be"},
		{"template source", "run \"x\" {\n module { source = \"./${var.source}\" }\n}\n", "module source must be"},
		{"invalid nested block", "run \"x\" {\n dynamic {}\n}\n", "unsupported block"},
		{"invalid mock block", "mock_provider \"azapi\" {\n mock_resource {}\n}\n", "requires one type label"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fs := afero.NewMemMapFs()
			stub := gostub.Stub(&filesystem.Fs, fs)
			defer stub.Reset()
			root := "module"
			path := filepath.Join(root, "case.tftest.hcl")
			require.NoError(t, afero.WriteFile(fs, path, []byte(tc.source), 0644))
			_, err := LoadModule(ModuleRef{Dir: root, AbsDir: root, TestFile: "case.tftest.hcl"})
			require.ErrorContains(t, err, tc.message)
			content, err := afero.ReadFile(fs, path)
			require.NoError(t, err)
			assert.Equal(t, tc.source, string(content))
		})
	}
}

func TestTestFileSelectionAndWriteBoundaries(t *testing.T) {
	fs := afero.NewMemMapFs()
	stub := gostub.Stub(&filesystem.Fs, fs)
	defer stub.Reset()
	root := "module"
	source := "run \"same\" {}\n"
	path := filepath.Join(root, "case.tftest.hcl")
	require.NoError(t, afero.WriteFile(fs, path, []byte(source), 0644))
	for _, filename := range []string{"main.tf", filepath.Join("..", "outside.tftest.hcl"), filepath.Join(string(filepath.Separator), "outside.tftest.hcl")} {
		_, err := LoadModule(ModuleRef{Dir: root, AbsDir: root, TestFile: filename})
		require.ErrorContains(t, err, "relative .tftest.hcl path")
	}
	_, err := LoadModule(ModuleRef{Dir: root, AbsDir: root, TestFile: "missing.tftest.hcl"})
	require.ErrorContains(t, err, "cannot read test file")
	module, err := LoadModule(ModuleRef{Dir: root, AbsDir: root, TestFile: "case.tftest.hcl"})
	require.NoError(t, err)
	require.ErrorContains(t, module.AddBlock("other.tftest.hcl", hclwrite.NewBlock("variables", nil)), "can only write")
	require.ErrorContains(t, module.AddBlock("main.tf", hclwrite.NewBlock("variables", nil)), "can only write")
	require.NoError(t, module.AddBlock("case.tftest.hcl", hclwrite.NewBlock("run", []string{"same"})))
	require.ErrorContains(t, module.SaveToDisk(), "duplicate test block")
	content, err := afero.ReadFile(fs, path)
	require.NoError(t, err)
	assert.Equal(t, source, string(content))
}

func TestTestFileDiscoveryIsOptIn(t *testing.T) {
	fs := afero.NewMemMapFs()
	stub := gostub.Stub(&filesystem.Fs, fs)
	defer stub.Reset()
	require.NoError(t, afero.WriteFile(fs, filepath.Join("module", "invalid.tftest.hcl"), []byte("invalid {"), 0644))
	require.NoError(t, afero.WriteFile(fs, filepath.Join("module", "main.tf"), []byte("variable \"x\" {}\n"), 0644))
	_, err := LoadModule(ModuleRef{Dir: "module", AbsDir: "module"})
	require.NoError(t, err)
	require.NoError(t, afero.WriteFile(fs, filepath.Join("module", "valid.tftest.hcl"), []byte("run \"x\" {}\n"), 0644))
	require.NoError(t, afero.WriteFile(fs, filepath.Join("module", "main.tf"), []byte("invalid {"), 0644))
	_, err = LoadModule(ModuleRef{Dir: "module", AbsDir: "module", TestFile: "valid.tftest.hcl"})
	require.NoError(t, err)
}
