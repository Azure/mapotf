package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/Azure/mapotf/pkg"
	"github.com/google/uuid"
	"github.com/hashicorp/go-getter/v2"
	"github.com/spf13/afero"
)

func localizeConfigFolder(path string, ctx context.Context) (configPath string, onDefer func(), err error) {
	fs := pkg.MPTFFs
	exists, err := afero.Exists(fs, path)
	if exists && err == nil {
		return path, nil, nil
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
