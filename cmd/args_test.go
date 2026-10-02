package cmd_test

import (
	"github.com/Azure/mapotf/cmd"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFilterArgs(t *testing.T) {
	tests := []struct {
		name            string
		inputArgs       []string
		expectedMptf    []string
		expectedNonMptf []string
	}{
		{
			name:         "Native test selection and debug expression",
			inputArgs:    []string{"mapotf", "debug", "--test-file", "tests/unit.tftest.hcl", "--eval", "data.test_file.this.result"},
			expectedMptf: []string{"mapotf", "debug", "--test-file", "tests/unit.tftest.hcl", "--eval", "data.test_file.this.result"},
		},
		{
			name:         "Native test flags with equals",
			inputArgs:    []string{"mapotf", "debug", "--test-file=tests/unit.tftest.hcl", "--eval=data.test_file.this.result"},
			expectedMptf: []string{"mapotf", "debug", "--test-file=tests/unit.tftest.hcl", "--eval=data.test_file.this.result"},
		},
		{
			name:         "Recursive option is not forwarded",
			inputArgs:    []string{"mapotf", "transform", "--test-file", "tests/unit.tftest.hcl", "--recursive"},
			expectedMptf: []string{"mapotf", "transform", "--test-file", "tests/unit.tftest.hcl", "--recursive"},
		},
		{
			name:            "Test with mapotf specific arguments",
			inputArgs:       []string{"mapotf", "transform", "--tf-dir", "/testTerraform", "--mptf-dir", "/testMptf"},
			expectedMptf:    []string{"mapotf", "transform", "--tf-dir", "/testTerraform", "--mptf-dir", "/testMptf"},
			expectedNonMptf: nil,
		},
		{
			name:            "Test with terraform specific arguments",
			inputArgs:       []string{"mapotf", "apply", "-compact-warnings", "input=false", "-var", "a=b", "-auto-approve", "--tf-dir", "/testTerraform", "--mptf-dir", "/testMptf"},
			expectedMptf:    []string{"mapotf", "apply", "--tf-dir", "/testTerraform", "--mptf-dir", "/testMptf"},
			expectedNonMptf: []string{"-compact-warnings", "input=false", "-var", "a=b", "-auto-approve"},
		},
		{
			name:            "Test with terraform var along with mptf var",
			inputArgs:       []string{"mapotf", "apply", "--mptf-var", "mptfa=b", "--mptf-var-file", "mptf.var", "-var", "a=b", "-var-file=\"terraform.tfvars\"", "-var", "c=d"},
			expectedMptf:    []string{"mapotf", "apply", "--mptf-var", "mptfa=b", "--mptf-var-file", "mptf.var"},
			expectedNonMptf: []string{"-var", "a=b", "-var-file=\"terraform.tfvars\"", "-var", "c=d"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mptfArgs, nonMptfArgs := cmd.FilterArgs(tt.inputArgs)
			assert.Equal(t, tt.expectedMptf, mptfArgs)
			assert.Equal(t, tt.expectedNonMptf, nonMptfArgs)
		})
	}
}
