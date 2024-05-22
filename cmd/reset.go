package cmd

import (
	"fmt"

	"github.com/lonegunmanb/mptf/pkg"
	"github.com/lonegunmanb/mptf/pkg/backup"
	"github.com/spf13/cobra"
)

func NewResetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "reset",
		Short: "Reset all transformed Terraform files, mptf reset --tf-dir",
		FParseErrWhitelist: cobra.FParseErrWhitelist{
			UnknownFlags: true,
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			modulePaths, err := pkg.ModulePaths(cf.tfDir)
			if err != nil {
				return err
			}
			tfDirs := modulePaths
			for _, tfDir := range tfDirs {
				d := tfDir
				err = backup.RestoreBackup(d)
				if err != nil {
					return err
				}
			}
			fmt.Println("All transforms have been reverted.")
			return nil
		},
	}
}

func init() {
	rootCmd.AddCommand(NewResetCmd())
}
