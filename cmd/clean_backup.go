package cmd

import (
	"fmt"
	"github.com/lonegunmanb/mptf/pkg"
	"github.com/lonegunmanb/mptf/pkg/backup"
	"github.com/spf13/cobra"
)

func NewClearBackupCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "clean-backup",
		Short: "Reserve all transformed Terraform files, clear backup files, mptf clean-backup --tf-dir  [path to config files]",
		FParseErrWhitelist: cobra.FParseErrWhitelist{
			UnknownFlags: true,
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return cleanBackup()
		},
	}
}

func cleanBackup() error {
	modulePaths, err := pkg.ModulePaths(cf.tfDir)
	if err != nil {
		return err
	}
	tfDirs := modulePaths
	for _, tfDir := range tfDirs {
		d := tfDir
		err = backup.ClearBackup(d)
		if err != nil {
			return err
		}
	}
	fmt.Println("All backups have been cleaned.")
	return nil
}

func init() {
	rootCmd.AddCommand(NewClearBackupCmd())
}
