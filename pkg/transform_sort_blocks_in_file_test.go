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
		{
			desc: "empty_address_returns_error",
			mptf: `
transform "sort_blocks_in_file" this {
  file_name     = "variables.tf"
  desired_order = [""]
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
			errorSubstring: "min",
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

// TestSortBlocksInFile_NoStrayWhitespace pins the file-boundary whitespace
// behaviour: after sorting blocks within a file, the file on disk must not
// have leading blank lines or more than one trailing blank line, and stray
// blank lines left over from removed/re-added blocks must be collapsed. We
// compare raw bytes (no `formatHcl` trim) because the bug we are guarding
// against is precisely about file-boundary blanks.
func TestSortBlocksInFile_NoStrayWhitespace(t *testing.T) {
	mptf := `
transform "sort_blocks_in_file" this {
  file_name     = "variables.tf"
  desired_order = ["variable.alpha", "variable.bravo", "variable.charlie"]
}
`
	stub := gostub.Stub(&filesystem.Fs, fakeFs(map[string]string{
		"/variables.tf": `variable "charlie" {
  type        = string
  description = "C variable"
}

variable "alpha" {
  type        = string
  description = "A variable"
}

variable "bravo" {
  type        = string
  description = "B variable"
}
`,
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
	require.NoError(t, plan.Apply())

	actual, err := afero.ReadFile(filesystem.Fs, "/variables.tf")
	require.NoError(t, err)
	expected := `variable "alpha" {
  type        = string
  description = "A variable"
}

variable "bravo" {
  type        = string
  description = "B variable"
}

variable "charlie" {
  type        = string
  description = "C variable"
}
`
	assert.Equal(t, expected, string(actual))
}

// TestSortBlocksInFile_PullsFromOtherFileNoStrayWhitespace mirrors the AVM
// pre-commit scenario where some `variable` blocks live in `main.tf` and need
// to be pulled into `variables.tf`. The file the blocks come from must not be
// left with stray blank lines, and the destination file must not have leading
// blanks.
func TestSortBlocksInFile_PullsFromOtherFileNoStrayWhitespace(t *testing.T) {
	mptf := `
transform "sort_blocks_in_file" this {
  file_name     = "variables.tf"
  desired_order = ["variable.alpha", "variable.bravo"]
}
`
	stub := gostub.Stub(&filesystem.Fs, fakeFs(map[string]string{
		"/main.tf": `variable "alpha" {
  type = string
}

resource "fake_resource" "keep" {
  attr = "keep"
}

variable "bravo" {
  type = string
}
`,
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
	require.NoError(t, plan.Apply())

	mainActual, err := afero.ReadFile(filesystem.Fs, "/main.tf")
	require.NoError(t, err)
	expectedMain := `resource "fake_resource" "keep" {
  attr = "keep"
}
`
	assert.Equal(t, expectedMain, string(mainActual))

	varsActual, err := afero.ReadFile(filesystem.Fs, "/variables.tf")
	require.NoError(t, err)
	expectedVars := `variable "alpha" {
  type = string
}

variable "bravo" {
  type = string
}
`
	assert.Equal(t, expectedVars, string(varsActual))
}

func TestSortBlocksInFile_EmptyDesiredOrderIsNoOp(t *testing.T) {
	mptf := `
transform "sort_blocks_in_file" this {
  file_name     = "variables.tf"
  desired_order = []
}
`
	originalVars := "variable \"x\" {\n  type = string\n}\n"
	stub := gostub.Stub(&filesystem.Fs, fakeFs(map[string]string{
		"/variables.tf": originalVars,
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
	require.NoError(t, err)
	after, err := afero.ReadFile(filesystem.Fs, "/variables.tf")
	require.NoError(t, err)
	assert.Equal(t, originalVars, string(after))
}
