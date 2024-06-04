package cmd_test

import (
	"context"
	"github.com/Azure/mapotf/pkg"
	"testing"

	"github.com/Azure/mapotf/cmd"
	filesystem "github.com/Azure/mapotf/pkg/fs"
	"github.com/prashantv/gostub"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"os"
)

func TestCleanBackup(t *testing.T) {
	fs := afero.NewMemMapFs()
	_ = afero.WriteFile(fs, "/testTerraform/main.tf.mptfbackup", []byte("backup content"), 0644)
	_ = afero.WriteFile(fs, "/testTerraform/another.tf.mptfbackup", []byte("another backup content"), 0644)
	stub := gostub.Stub(&filesystem.Fs, fs).Stub(
		&os.Args, []string{
			"mapotf",
			"clean-backup",
			"--tf-dir", "/testTerraform",
		}).Stub(&pkg.AbsDir, func(dir string) (string, error) {
		return dir, nil
	})
	defer stub.Reset()

	cmd.Execute(context.Background())

	// Verify that all backup files have been cleaned
	exists, err := afero.Exists(fs, "/testTerraform/main.tf.mptfbackup")
	require.NoError(t, err)
	assert.False(t, exists)

	exists, err = afero.Exists(fs, "/testTerraform/another.tf.mptfbackup")
	require.NoError(t, err)
	assert.False(t, exists)
}
