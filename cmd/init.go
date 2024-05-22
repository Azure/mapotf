package cmd

import (
	"github.com/spf13/cobra"
)

func NewInitCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Prepare your working directory for other commands",
		FParseErrWhitelist: cobra.FParseErrWhitelist{
			UnknownFlags: true,
		},
		RunE: wrapTerraformCommand("init"),
	}
}

func init() {
	rootCmd.AddCommand(NewInitCmd())
}
