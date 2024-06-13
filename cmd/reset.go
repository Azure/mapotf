package cmd

import (
	"fmt"

	"github.com/Azure/mapotf/pkg"
	"github.com/Azure/mapotf/pkg/backup"
	"github.com/spf13/cobra"
)

func NewResetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "reset",
		Short: "Reset all transformed Terraform files, mapotf reset --tf-dir",
		FParseErrWhitelist: cobra.FParseErrWhitelist{
			UnknownFlags: true,
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return reset()
		},
	}
}

func reset() error {
	moduleRefs, err := pkg.ModuleRefs(cf.tfDir)
	if err != nil {
		return err
	}
	for _, tfDir := range moduleRefs {
		d := tfDir
		err = backup.Reset(d.AbsDir)
		if err != nil {
			return err
		}
	}
	fmt.Println("All transforms have been reverted.")
	return nil
}

func init() {
	rootCmd.AddCommand(NewResetCmd())
}
