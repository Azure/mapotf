package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	filesystem "github.com/Azure/mapotf/pkg/fs"

	"github.com/Azure/mapotf/pkg"
	"github.com/google/uuid"
	"github.com/hashicorp/go-getter/v2"
	"github.com/spf13/afero"
)

func localizeConfigFolder(path string, ctx context.Context) (configPath string, onDefer func(), err error) {
	absPath, err := pkg.AbsDir(path)
	if err == nil {
		exists, err := afero.Exists(filesystem.Fs, absPath)
		if exists && err == nil {
			return path, nil, nil
		}
	}
	tmp := filepath.Join(os.TempDir(), uuid.NewString())
	cleaner := func() {
		_ = os.RemoveAll(tmp)
	}

	pwd, err := os.Getwd()
	if err != nil {
		return "", cleaner, fmt.Errorf("failed to get current working directory: %w", err)
	}

	req := &getter.Request{
		Src: path,
		Dst: tmp,
		Pwd: pwd,
	}

	getter := getter.Client{
		DisableSymlinks: true,
	}

	result, err := getter.Get(ctx, req)
	if err != nil {
		return "", cleaner, err
	}

	if result == nil {
		return "", cleaner, fmt.Errorf("cannot get config path")
	}

	return result.Dst, cleaner, nil
}
