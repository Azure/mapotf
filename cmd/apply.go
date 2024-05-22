package cmd

import (
	"github.com/spf13/cobra"
)

func NewApplyCmd() *cobra.Command {
	recursive := false
	cmd := &cobra.Command{
		Use:   "apply",
		Short: "Create or update infrastructure",
		FParseErrWhitelist: cobra.FParseErrWhitelist{
			UnknownFlags: true,
		},
		RunE: wrapTerraformCommandWithEphemeralTransform(cf.tfDir, "apply", &recursive),
	}

	cmd.Flags().BoolVarP(&recursive, "recursive", "r", false, "With transforms to all modules or not, default to the root module only.")
	return cmd
}

func init() {
	rootCmd.AddCommand(NewApplyCmd())
}
