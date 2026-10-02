package cmd

import (
	"testing"

	"github.com/Azure/golden"
	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVarFlagsWithoutEqualSign(t *testing.T) {
	args := []string{"--mptf-var", "testVar"}
	_, err := varFlags(args)
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "is not correctly specified. Must be a variable name and value separated by an equals sign, like --mptf-var key=value")
}

func TestVarFlagsWithVarFile(t *testing.T) {
	args := []string{"--mptf-var-file", "testVarFile"}
	expected := []golden.CliFlagAssignedVariables{
		golden.NewCliFlagAssignedVariableFile("testVarFile"),
	}

	result, err := varFlags(args)

	assert.NoError(t, err, "Unexpected error: %v", err)
	assert.Equal(t, expected, result, "Expected %+v, got %+v", expected, result)
}

func TestVarFlagsWithVarFile_incorrectFlag(t *testing.T) {
	args := []string{"--mptf-var-file"}
	_, err := varFlags(args)
	assert.NotNil(t, err, "Unexpected error: %v", err)
	assert.Contains(t, err.Error(), "missing value for --mptf-var-file")
}

func TestVarFlagsWithoutVarAssignment(t *testing.T) {
	args := []string{"--mptf-var"}
	_, err := varFlags(args)
	assert.NotNil(t, err, "Expected error but got nil")
	assert.Contains(t, err.Error(), "missing value for --mptf-var")
}

func TestVarFlagsPreserveLiteralCollections(t *testing.T) {
	flags := rootCmd.PersistentFlags()
	for _, name := range []string{"mptf-var", "mptf-var-file"} {
		flag := flags.Lookup(name)
		value := flag.Value.(pflag.SliceValue)
		original := append([]string(nil), value.GetSlice()...)
		changed := flag.Changed
		t.Cleanup(func() {
			require.NoError(t, value.Replace(original))
			flag.Changed = changed
		})
	}
	list := `new_location_modules=["C:\\fixture\\module","C:\\fixture\\module\\modules\\child"]`
	args := []string{
		"--mptf-var", list,
		"--mptf-var", `value="contains, commas and = equals"`,
		"--mptf-var-file", `C:\fixture\with,comma.mptfvars`,
	}
	require.NoError(t, flags.Parse(args))
	raw, err := flags.GetStringArray("mptf-var")
	require.NoError(t, err)
	assert.Equal(t, []string{list, `value="contains, commas and = equals"`}, raw)
	assignments, err := varFlags(args)
	require.NoError(t, err)
	assert.Equal(t, []golden.CliFlagAssignedVariables{
		golden.NewCliFlagAssignedVariable("new_location_modules", `["C:\\fixture\\module","C:\\fixture\\module\\modules\\child"]`),
		golden.NewCliFlagAssignedVariable("value", `"contains, commas and = equals"`),
		golden.NewCliFlagAssignedVariableFile(`C:\fixture\with,comma.mptfvars`),
	}, assignments)
}
