package cmd_test

import (
	"context"
	"github.com/Azure/mapotf/cmd"
	"github.com/Azure/mapotf/pkg"
	filesystem "github.com/Azure/mapotf/pkg/fs"
	"github.com/prashantv/gostub"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"os"
	"testing"
	"time"
)

func TestSuccessfulTransformation(t *testing.T) {
	// Stub the filesystem
	fs := afero.NewMemMapFs()
	_ = afero.WriteFile(fs, "/testData/main.mptf.hcl", []byte(`
data resource "fake_resource" {
  resource_type = "fake_resource"
}

transform update_in_place "fake_resource" {
 for_each = data.resource.fake_resource.result.fake_resource
 target_block_address = each.value.mptf.block_address
 asstring{
   tags = "merge(${try(coalesce(each.value.tags, "{}"), "{}")}, { \n block_address = \"${each.value.mptf.block_address}\" \n file_name = \"${each.value.mptf.range.file_name}\"\n  })"
 }
}
`), 0644)
	terraformCode := `
resource "fake_resource" this {
  tags = {}
}

resource "fake_resource" that {
}
`
	_ = afero.WriteFile(fs, "/testTerraform/main.tf", []byte(terraformCode), 0644)
	stub := gostub.Stub(&filesystem.Fs, fs).Stub(
		&os.Args, []string{
			"mapotf",
			"transform",
			"--tf-dir", "/testTerraform",
			"--mptf-dir", "/testData",
		}).Stub(&pkg.AbsDir, func(dir string) (string, error) {
		return dir, nil
	})
	defer stub.Reset()

	mptfArgs, nonMptfArgs := cmd.FilterArgs(os.Args)
	os.Args = mptfArgs
	cmd.NonMptfArgs = nonMptfArgs

	runWithTimeout(t, func() {
		cmd.Execute(context.Background())
		tfFile, err := afero.ReadFile(fs, "/testTerraform/main.tf")
		require.NoError(t, err)
		tfFileStr := string(tfFile)
		expected := `
resource "fake_resource" this {
  tags = merge({}, {
    block_address = "resource.fake_resource.this"
    file_name     = "main.tf"
  })
}

resource "fake_resource" that {
  tags = merge({}, {
    block_address = "resource.fake_resource.that"
    file_name     = "main.tf"
  })
}
`
		assert.Equal(t, expected, tfFileStr)
		backupTfFilePath := "/testTerraform/main.tf.mptfbackup"
		exists, err := afero.Exists(fs, backupTfFilePath)
		require.NoError(t, err)
		assert.True(t, exists)
		backupFileContent, err := afero.ReadFile(fs, backupTfFilePath)
		require.NoError(t, err)
		assert.Equal(t, terraformCode, string(backupFileContent))

	}, 100*time.Millisecond)
}

func runWithTimeout(t *testing.T, callback func(), timeout time.Duration) {
	done := make(chan bool)
	go func() {
		callback()
		done <- true
	}()
	select {
	case <-done:
		return
	case <-time.After(timeout):
		t.Fatal("timeout")
	}
}
