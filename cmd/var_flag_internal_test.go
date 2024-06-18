package cmd

import (
	"testing"

	"github.com/Azure/golden"
	"github.com/stretchr/testify/assert"
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
