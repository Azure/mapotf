package cmd

import (
	"context"
	"fmt"
	filesystem "github.com/Azure/mapotf/pkg/fs"
	"os"
	"path/filepath"

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
	result, err := getter.Get(ctx, tmp, path)
	if err != nil {
		return "", cleaner, err
	}
	if result == nil {
		return "", cleaner, fmt.Errorf("cannot get config path")
	}
	return result.Dst, cleaner, nil
}
