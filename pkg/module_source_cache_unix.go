//go:build !windows

package pkg

import (
	"os"
	"path/filepath"
)

func defaultModuleSourceCLIConfigPaths() (string, string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", "", err
	}
	return filepath.Join(home, ".terraformrc"), filepath.Join(home, ".terraform.d"), nil
}
