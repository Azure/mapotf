package cmd

import (
	"github.com/spf13/cobra"
)

func NewConsoleCmd() *cobra.Command {
	recursive := false
	cmd := &cobra.Command{
		Use:   "console",
		Short: "Try Terraform expressions at an interactive command prompt",
		FParseErrWhitelist: cobra.FParseErrWhitelist{
			UnknownFlags: true,
		},
		RunE: wrapTerraformCommandWithEphemeralTransform(cf.tfDir, "console", &recursive),
	}

	cmd.Flags().BoolVarP(&recursive, "recursive", "r", false, "With transforms to all modules or not, default to the root module only.")
	return cmd
}

func init() {
	rootCmd.AddCommand(NewConsoleCmd())
}
