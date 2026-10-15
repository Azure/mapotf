package pkg

import (
	"path/filepath"

	"golang.org/x/sys/windows"
)

func defaultModuleSourceCLIConfigPaths() (string, string, error) {
	// Terraform uses the Windows known folder, not the overridable APPDATA variable.
	dir, err := windows.KnownFolderPath(windows.FOLDERID_RoamingAppData, 0)
	if err != nil {
		return "", "", err
	}
	return filepath.Join(dir, "terraform.rc"), filepath.Join(dir, "terraform.d"), nil
}
