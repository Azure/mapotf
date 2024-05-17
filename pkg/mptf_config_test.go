package pkg_test

import (
	"context"
	"testing"

	"github.com/lonegunmanb/mptf/pkg"
	"github.com/lonegunmanb/mptf/pkg/terraform"
	"github.com/prashantv/gostub"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewMetaProgrammingTFConfigShouldLoadTerraformBlocks(t *testing.T) {
	stub := gostub.Stub(&terraform.Fs, fakeFs(map[string]string{
		"/main.tf": `resource "fake_resource" this {}`,
	}))
	defer stub.Reset()

	sut, err := pkg.NewMetaProgrammingTFConfig("/", nil, nil, context.TODO())
	require.NoError(t, err)
	assert.NotEmpty(t, sut.ResourceBlocks)
}

func fakeFs(files map[string]string) afero.Fs {
	fs := afero.NewMemMapFs()
	for n, content := range files {
		_ = afero.WriteFile(fs, n, []byte(content), 0644)
	}
	return fs
}
