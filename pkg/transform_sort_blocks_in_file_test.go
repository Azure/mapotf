package pkg_test

import (
	"context"
	"testing"

	"github.com/Azure/golden"
	"github.com/Azure/mapotf/pkg"
	filesystem "github.com/Azure/mapotf/pkg/fs"
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/prashantv/gostub"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSortBlocksInFile(t *testing.T) {
	cases := []struct {
		desc           string
		mptf           string
		initialFiles   map[string]string
		expectedFiles  map[string]string
		wantErr        bool
		errorSubstring string
	}{
		{
			desc: "reorder_within_same_file",
			mptf: `
transform "sort_blocks_in_file" this {
  file_name     = "variables.tf"
  desired_order = ["variable.alpha", "variable.bravo", "variable.charlie"]
}
`,
			initialFiles: map[string]string{
				"/variables.tf": `
variable "charlie" {
  type = string
}

variable "alpha" {
  type = string
}

variable "bravo" {
  type = string
}
`,
			},
			expectedFiles: map[string]string{
				"/variables.tf": `variable "alpha" {
  type = string
}

variable "bravo" {
  type = string
}

variable "charlie" {
  type = string
}

`,
			},
		},
		{
			desc: "pull_blocks_in_from_other_files",
			mptf: `
transform "sort_blocks_in_file" this {
  file_name     = "variables.tf"
  desired_order = ["variable.alpha", "variable.bravo"]
}
`,
			initialFiles: map[string]string{
				"/main.tf": `
variable "alpha" {
  type = string
}

resource "fake_resource" "keep" {
  attr = "keep"
}
`,
				"/variables.tf": `
variable "bravo" {
  type = string
}
`,
			},
			expectedFiles: map[string]string{
				"/main.tf": `
resource "fake_resource" "keep" {
  attr = "keep"
}
`,
				"/variables.tf": `variable "alpha" {
  type = string
}

variable "bravo" {
  type = string
}

`,
			},
		},
		{
			desc: "unlisted_blocks_left_in_place",
			mptf: `
transform "sort_blocks_in_file" this {
  file_name     = "variables.tf"
  desired_order = ["variable.alpha"]
}
`,
			initialFiles: map[string]string{
				"/variables.tf": `
variable "charlie" {
  type = string
}

variable "alpha" {
  type = string
}
`,
			},
			expectedFiles: map[string]string{
				"/variables.tf": `
variable "charlie" {
  type = string
}

variable "alpha" {
  type = string
}

`,
			},
		},
		{
			desc: "unlisted_moved_blocks_left_in_place",
			mptf: `
transform "sort_blocks_in_file" this {
  file_name     = "moved.tf"
  desired_order = ["moved.1"]
}
`,
			initialFiles: map[string]string{
				"/moved.tf": `
moved {
  from = aws_instance.a
  to   = aws_instance.aa
}

moved {
  from = aws_instance.b
  to   = aws_instance.bb
}
`,
			},
			expectedFiles: map[string]string{
				"/moved.tf": `
moved {
  from = aws_instance.a
  to   = aws_instance.aa
}

moved {
  from = aws_instance.b
  to   = aws_instance.bb
}

`,
			},
		},
		{
			desc: "missing_address_returns_error",
			mptf: `
transform "sort_blocks_in_file" this {
  file_name     = "variables.tf"
  desired_order = ["variable.does_not_exist"]
}
`,
			initialFiles: map[string]string{
				"/variables.tf": `
variable "alpha" {
  type = string
}
`,
			},
			expectedFiles:  nil,
			wantErr:        true,
			errorSubstring: "cannot find block",
		},
		{
			desc: "duplicate_address_returns_error",
			mptf: `
transform "sort_blocks_in_file" this {
  file_name     = "variables.tf"
  desired_order = ["variable.alpha", "variable.alpha"]
}
`,
			initialFiles: map[string]string{
				"/variables.tf": `
variable "alpha" {
  type = string
}
`,
			},
			expectedFiles:  nil,
			wantErr:        true,
			errorSubstring: "unique",
		},
	}

	for _, c := range cases {
		t.Run(c.desc, func(t *testing.T) {
			stub := gostub.Stub(&filesystem.Fs, fakeFs(c.initialFiles))
			defer stub.Reset()

			readFile, diag := hclsyntax.ParseConfig([]byte(c.mptf), "test.hcl", hcl.InitialPos)
			require.Falsef(t, diag.HasErrors(), diag.Error())
			writeFile, diag := hclwrite.ParseConfig([]byte(c.mptf), "test.hcl", hcl.InitialPos)
			require.Falsef(t, diag.HasErrors(), diag.Error())
			hclBlock := golden.NewHclBlock(readFile.Body.(*hclsyntax.Body).Blocks[0], writeFile.Body().Blocks()[0], nil)
			cfg, err := pkg.NewMetaProgrammingTFConfig(&pkg.TerraformModuleRef{
				Dir:    "/",
				AbsDir: "/",
			}, nil, []*golden.HclBlock{hclBlock}, nil, context.TODO())
			require.NoError(t, err)
			plan, err := pkg.RunMetaProgrammingTFPlan(cfg)
			if c.wantErr && err != nil {
				if c.errorSubstring != "" {
					assert.Contains(t, err.Error(), c.errorSubstring)
				}
				return
			}
			require.NoError(t, err)

			err = plan.Apply()
			if c.wantErr {
				require.Error(t, err)
				if c.errorSubstring != "" {
					assert.Contains(t, err.Error(), c.errorSubstring)
				}
				return
			}
			require.NoError(t, err)

			for path, expectedContent := range c.expectedFiles {
				actual, err := afero.ReadFile(filesystem.Fs, path)
				require.NoError(t, err)
				expected := formatHcl(expectedContent)
				actualFormatted := formatHcl(string(actual))
				assert.Equal(t, expected, actualFormatted)
			}
		})
	}
}

func TestSortBlocksInFile_EmptyDesiredOrderIsError(t *testing.T) {
	mptf := `
transform "sort_blocks_in_file" this {
  file_name     = "variables.tf"
  desired_order = []
}
`
	stub := gostub.Stub(&filesystem.Fs, fakeFs(map[string]string{
		"/variables.tf": `variable "x" { type = string }`,
	}))
	defer stub.Reset()

	readFile, diag := hclsyntax.ParseConfig([]byte(mptf), "test.hcl", hcl.InitialPos)
	require.Falsef(t, diag.HasErrors(), diag.Error())
	writeFile, diag := hclwrite.ParseConfig([]byte(mptf), "test.hcl", hcl.InitialPos)
	require.Falsef(t, diag.HasErrors(), diag.Error())
	hclBlock := golden.NewHclBlock(readFile.Body.(*hclsyntax.Body).Blocks[0], writeFile.Body().Blocks()[0], nil)
	cfg, err := pkg.NewMetaProgrammingTFConfig(&pkg.TerraformModuleRef{
		Dir:    "/",
		AbsDir: "/",
	}, nil, []*golden.HclBlock{hclBlock}, nil, context.TODO())
	require.NoError(t, err)
	plan, err := pkg.RunMetaProgrammingTFPlan(cfg)
	require.NoError(t, err)
	err = plan.Apply()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "desired_order must not be empty")
}
