package cmd

import (
	"context"
	"errors"
	"fmt"
	"github.com/spf13/cobra"
	"os"
	"os/exec"
)

// Build metadata set via -ldflags at release time by goreleaser.
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:     "mapotf",
	Version: fmt.Sprintf("%s (commit %s, built %s)", version, commit, date),
	Short:   "Meta-programming for Terraform / OpenTofu",
	Long: `mapotf applies declarative HCL transforms to a target Terraform or OpenTofu
module: adding telemetry, normalising provider versions, sorting blocks and
attributes, and other repeatable rewrites used by AVM and similar governance
pipelines.`,
	FParseErrWhitelist: cobra.FParseErrWhitelist{
		UnknownFlags: true,
	},
	SilenceErrors: false,
	SilenceUsage:  true,
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute(ctx context.Context) {
	err := rootCmd.ExecuteContext(ctx)
	if err != nil {
		var pe *exec.ExitError
		if errors.As(err, &pe) {
			os.Exit(pe.ExitCode())
		}
		os.Exit(1)
	}
}

func init() {
	pwd, err := os.Getwd()
	if err != nil {
		panic(fmt.Sprintf("error on getting working dir:%s", err.Error()))
	}
	rootCmd.PersistentFlags().StringVar(&cf.tfDir, "tf-dir", pwd, "Terraform directory")
	rootCmd.PersistentFlags().StringSliceVar(&cf.mptfDirs, "mptf-dir", nil, "MPTF directory")

	rootCmd.PersistentFlags().StringSlice("mptf-var", cf.mptfVars, "Set a value for one of the input variables in the root module of the configuration. Use this option more than once to set more than one variable.")
	rootCmd.PersistentFlags().StringSlice("mptf-var-file", cf.mptfVarFiles, "Load variable values from the given file, in addition to the default files mptf.mptfvars and *.auto.mptfvars. Use this option more than once to include more than one variables file.")
}
