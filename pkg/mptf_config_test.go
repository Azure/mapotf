package pkg_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/Azure/mapotf/pkg"
	filesystem "github.com/Azure/mapotf/pkg/fs"
	"github.com/prashantv/gostub"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewMetaProgrammingTFConfigShouldLoadTerraformBlocks(t *testing.T) {
	stub := gostub.Stub(&filesystem.Fs, fakeFs(map[string]string{
		"/main.tf": `resource "fake_resource" this {}`,
	}))
	defer stub.Reset()

	sut, err := pkg.NewMetaProgrammingTFConfig(&pkg.TerraformModuleRef{
		Dir:    "/",
		AbsDir: "/",
	}, nil, nil, nil, context.TODO())
	require.NoError(t, err)
	assert.NotEmpty(t, sut.ResourceBlocks)
}

func TestNewMetaProgrammingTFConfigShouldLoadTerraformBlock(t *testing.T) {
	stub := gostub.Stub(&filesystem.Fs, fakeFs(map[string]string{
		"/main.tf": `terraform {}`,
	}))
	defer stub.Reset()

	sut, err := pkg.NewMetaProgrammingTFConfig(&pkg.TerraformModuleRef{
		Dir:    "/",
		AbsDir: "/",
	}, nil, nil, nil, context.TODO())
	require.NoError(t, err)
	assert.NotNil(t, sut.TerraformBlock())
}

func TestMetaProgrammingTFConfigBlocks(t *testing.T) {
	stub := gostub.Stub(&filesystem.Fs, fakeFs(map[string]string{
		"/main.tf": `
			resource "fake_resource" "this" {}
			data "fake_data" "this" {}
			variable "fake_variable" {}
			output "fake_output" {}
			locals {
				fake_local = "value"
			}
			module "fake_module" {
				source = "./module"
			}
			terraform {
				required_version = ">= 0.12"
			}
		`,
	}))
	defer stub.Reset()

	sut, err := pkg.NewMetaProgrammingTFConfig(&pkg.TerraformModuleRef{
		Dir:    "/",
		AbsDir: "/",
	}, nil, nil, nil, context.TODO())
	require.NoError(t, err)

	assert.NotEmpty(t, sut.ResourceBlocks(), "resourceBlocks should not be empty")
	assert.NotEmpty(t, sut.DataBlocks(), "dataBlocks should not be empty")
	assert.NotEmpty(t, sut.VariableBlocks(), "variableBlocks should not be empty")
	assert.NotEmpty(t, sut.OutputBlocks(), "outputBlocks should not be empty")
	assert.NotEmpty(t, sut.LocalBlocks(), "localBlocks should not be empty")
	assert.NotEmpty(t, sut.ModuleBlocks(), "moduleBlocks should not be empty")
	assert.NotNil(t, sut.TerraformBlock(), "terraformBlock should not be nil")
}

func TestModulePathsWhenModulesJsonExists(t *testing.T) {
	stub := gostub.Stub(&filesystem.Fs, fakeFs(map[string]string{
		"/.terraform/modules/modules.json": `{
			"Modules": [
				{
					"Key": "",
					"Source": "",
					"Dir": "."
				},
				{
					"Key": "that",
					"Source": "./module",
					"Dir": "module"
				}
			]
		}`,
	}))
	defer stub.Reset()

	refs, err := pkg.ModuleRefs("/")
	require.NoError(t, err)
	var paths []string
	for _, ref := range refs {
		paths = append(paths, ref.AbsDir)
	}
	pwd, err := os.Getwd()
	require.NoError(t, err)
	assert.Contains(t, paths, pwd)
	assert.Contains(t, paths, filepath.Join(pwd, "module"))
}

func TestModulePathsWhenModulesJsonDoesNotExist(t *testing.T) {
	stub := gostub.Stub(&filesystem.Fs, fakeFs(map[string]string{}))
	defer stub.Reset()

	refs, err := pkg.ModuleRefs(".")
	require.NoError(t, err)
	var paths []string
	for _, ref := range refs {
		paths = append(paths, ref.AbsDir)
	}
	pwd, err := os.Getwd()
	require.NoError(t, err)
	assert.Equal(t, []string{pwd}, paths)
}

func fakeFs(files map[string]string) afero.Fs {
	fs := afero.NewMemMapFs()
	for n, content := range files {
		_ = afero.WriteFile(fs, n, []byte(content), 0644)
	}
	return fs
}
