package cmd

import (
	"fmt"

	"github.com/Azure/mapotf/pkg"
	"github.com/Azure/mapotf/pkg/backup"
	"github.com/spf13/cobra"
)

func NewClearBackupCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "clean-backup",
		Short: "Reserve all transformed Terraform files, clear backup files, mapotf clean-backup --tf-dir  [path to config files]",
		FParseErrWhitelist: cobra.FParseErrWhitelist{
			UnknownFlags: true,
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return cleanBackup()
		},
	}
}

func cleanBackup() error {
	moduleRefs, err := pkg.ModuleRefs(cf.tfDir)
	if err != nil {
		return err
	}
	for _, tfDir := range moduleRefs {
		d := tfDir
		err = backup.ClearBackup(d.AbsDir)
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
