package cmd

import (
	"github.com/spf13/cobra"
	"os"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "mptf",
	Short: "A brief description of your application",
	Long: `A longer description that spans multiple lines and likely contains
examples and usage of using your application. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	FParseErrWhitelist: cobra.FParseErrWhitelist{
		UnknownFlags: true,
	},
	// Uncomment the following line if your bare application
	// has an action associated with it:
	// Run: func(cmd *cobra.Command, args []string) { },
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVar(&cf.tfDir, "tf-dir", "", "Terraform directory")
	rootCmd.PersistentFlags().StringVar(&cf.mptfDir, "mptf-dir", "", "MPTF directory")

	rootCmd.PersistentFlags().StringSlice("mptf-var", cf.mptfVars, "Set a value for one of the input variables in the root module of the configuration. Use this option more than once to set more than one variable.")
	rootCmd.PersistentFlags().StringSlice("mptf-var-file", cf.mptfVarFiles, "Load variable values from the given file, in addition to the default files mptf.mptfvars and *.auto.mptfvars. Use this option more than once to include more than one variables file.")
}
