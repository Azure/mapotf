package cmd_test

import (
	"context"
	"github.com/Azure/mapotf/pkg"
	"os"
	"testing"

	"github.com/Azure/mapotf/cmd"
	filesystem "github.com/Azure/mapotf/pkg/fs"
	"github.com/prashantv/gostub"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReset(t *testing.T) {
	// Stub the filesystem
	fs := afero.NewMemMapFs()
	_ = afero.WriteFile(fs, "/testTerraform/main.tf", []byte("original content"), 0644)
	_ = afero.WriteFile(fs, "/testTerraform/main.tf.mptfbackup", []byte("backup content"), 0644)
	stub := gostub.Stub(&filesystem.Fs, fs).Stub(
		&os.Args, []string{
			"mapotf",
			"reset",
			"--tf-dir", "/testTerraform",
		}).Stub(&pkg.AbsDir, func(dir string) (string, error) {
		return dir, nil
	})
	defer stub.Reset()

	cmd.Execute(context.Background())

	// Verify that the original file has been restored
	content, err := afero.ReadFile(fs, "/testTerraform/main.tf")
	require.NoError(t, err)
	assert.Equal(t, "backup content", string(content))

	// Verify that the backup file no longer exists
	exists, err := afero.Exists(fs, "/testTerraform/main.tf.mptfbackup")
	require.NoError(t, err)
	assert.False(t, exists)
	tfFile, err := afero.ReadFile(fs, "/testTerraform/main.tf")
	require.NoError(t, err)
	assert.Equal(t, "backup content", string(tfFile))
}
