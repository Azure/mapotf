package backup

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	filesystem "github.com/Azure/mapotf/pkg/fs"
	"github.com/spf13/afero"
)

const BackupExtension = ".mptfbackup"
const NewFileExtension = ".mptfnew"

func BackupFolder(dir string) error {
	terraformFile, err := afero.Glob(filesystem.Fs, filepath.Join(dir, "*.tf"))
	if err != nil {
		return fmt.Errorf("cannot list terraform files in %s:%+v", dir, err)
	}
	for _, file := range terraformFile {
		backupFile := file + BackupExtension
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

func Reset(dir string) error {
	err := restoreBackup(dir)
	if err != nil {
		return err
	}
	return removeNewFiles(dir)
}

func removeNewFiles(dir string) error {
	newFileIndicators, err := afero.Glob(filesystem.Fs, filepath.Join(dir, "*"+NewFileExtension))
	if err != nil {
		return fmt.Errorf("cannot list new file indicators in %s:%+v", dir, err)
	}
	for _, newFileIndicator := range newFileIndicators {
		newFile, _ := strings.CutSuffix(newFileIndicator, NewFileExtension)
		if err = filesystem.Fs.Remove(newFile); err != nil {
			return fmt.Errorf("cannot delete new file %s:%+v", newFile, err)
		}
		if err = filesystem.Fs.Remove(newFileIndicator); err != nil {
			return fmt.Errorf("cannot delete new file indicator in %s:%+v", newFileIndicator, err)
		}
	}
	return nil
}

func restoreBackup(dir string) error {
	backupFiles, err := afero.Glob(filesystem.Fs, filepath.Join(dir, "*"+BackupExtension))
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
		originalFile := backupFile[:len(backupFile)-len(BackupExtension)] // remove the extension to get the original file name
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
	backupFiles, err := afero.Glob(filesystem.Fs, filepath.Join(dir, "*"+BackupExtension))
	if err != nil {
		return fmt.Errorf("cannot list backup files in %s:%+v", dir, err)
	}
	newFileIndicators, err := afero.Glob(filesystem.Fs, filepath.Join(dir, "*"+NewFileExtension))
	if err != nil {
		return fmt.Errorf("cannot list new file indicators in %s:%+v", dir, err)
	}
	files := append(backupFiles, newFileIndicators...)
	for _, backupFile := range files {
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
