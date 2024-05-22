package cmd

import (
	"github.com/spf13/cobra"
)

func NewDestroyCmd() *cobra.Command {
	recursive := false
	cmd := &cobra.Command{
		Use:   "destroy",
		Short: "Destroy previously-created infrastructure",
		FParseErrWhitelist: cobra.FParseErrWhitelist{
			UnknownFlags: true,
		},
		RunE: wrapTerraformCommandWithEphemeralTransform(cf.tfDir, "destroy", &recursive),
	}

	cmd.Flags().BoolVarP(&recursive, "recursive", "r", false, "With transforms to all modules or not, default to the root module only.")
	return cmd
}

func init() {
	rootCmd.AddCommand(NewDestroyCmd())
}
