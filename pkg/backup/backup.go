package backup

import (
	"fmt"
	"os"
	"path/filepath"

	filesystem "github.com/Azure/mapotf/pkg/fs"
	"github.com/spf13/afero"
)

const Extension = ".mptfbackup"

func BackupFolder(dir string) error {
	terraformFile, err := afero.Glob(filesystem.Fs, filepath.Join(dir, "*.tf"))
	if err != nil {
		return fmt.Errorf("cannot list terraform files in %s:%+v", dir, err)
	}
	for _, file := range terraformFile {
		backupFile := file + Extension
		exist, err := afero.Exists(filesystem.Fs, backupFile)
		if err != nil {
			return fmt.Errorf("cannot check backup file %s:%+v", backupFile, err)
		}
		if exist {
			continue
		}
		// create the backup file, then copy the content of the terraform file to the backup file, with the same permission
		content, err := afero.ReadFile(filesystem.Fs, file)
		if err != nil {
			return fmt.Errorf("cannot read terraform file %s:%+v", file, err)
		}
		// get permission of the terraform file
		info, err := filesystem.Fs.Stat(file)
		if err != nil {
			return fmt.Errorf("cannot get permission of terraform file %s:%+v", file, err)
		}
		// write the content to the backup file
		if err = afero.WriteFile(filesystem.Fs, backupFile, content, info.Mode()); err != nil {
			return fmt.Errorf("cannot write backup file %s:%+v", backupFile, err)
		}
	}
	return nil
}

func RestoreBackup(dir string) error {
	backupFiles, err := afero.Glob(filesystem.Fs, filepath.Join(dir, "*"+Extension))
	if err != nil {
		return fmt.Errorf("cannot list backup files in %s:%+v", dir, err)
	}
	for _, backupFile := range backupFiles {
		// read the content of the backup file
		content, err := afero.ReadFile(filesystem.Fs, backupFile)
		if err != nil {
			return fmt.Errorf("cannot read backup file %s:%+v", backupFile, err)
		}
		// write the content to the original file
		originalFile := backupFile[:len(backupFile)-len(Extension)] // remove the extension to get the original file name
		info, err := getFilePerm(originalFile, backupFile, err)
		if err != nil {
			return err
		}
		if err = afero.WriteFile(filesystem.Fs, originalFile, content, info.Mode()); err != nil {
			return fmt.Errorf("cannot write original file %s:%+v", originalFile, err)
		}
		// delete the backup file
		if err = filesystem.Fs.Remove(backupFile); err != nil {
			return fmt.Errorf("cannot delete backup file %s:%+v", backupFile, err)
		}
	}
	return nil
}

func ClearBackup(dir string) error {
	backupFiles, err := afero.Glob(filesystem.Fs, filepath.Join(dir, "*"+Extension))
	if err != nil {
		return fmt.Errorf("cannot list backup files in %s:%+v", dir, err)
	}
	for _, backupFile := range backupFiles {
		// delete the backup file
		if err = filesystem.Fs.Remove(backupFile); err != nil {
			return fmt.Errorf("cannot delete backup file %s:%+v", backupFile, err)
		}
	}
	return nil
}

func getFilePerm(originalFile string, backupFile string, err error) (os.FileInfo, error) {
	var info os.FileInfo
	for _, path := range []string{originalFile, backupFile} {
		info, err = filesystem.Fs.Stat(path)
		if err == nil {
			break
		}
	}
	if err != nil {
		return nil, fmt.Errorf("cannot get permission of backup file %s:%+v", backupFile, err)
	}
	return info, nil
}
