package cmd

import (
	"context"
	"errors"
	"fmt"
	"github.com/spf13/cobra"
	"os"
	"os/exec"
)

// Build metadata set via -ldflags at release time.
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
	SilenceErrors:     false,
	SilenceUsage:      true,
	PersistentPreRunE: validateSelectionFlags,
}

func validateSelectionFlags(cmd *cobra.Command, _ []string) error {
	if cmd.Flags().Changed("test-file") || cf.testFile != "" {
		if cf.testFile == "" {
			return fmt.Errorf("--test-file must not be empty")
		}
		switch cmd.Name() {
		case "transform", "debug", "reset", "clean-backup":
		default:
			return fmt.Errorf("--test-file is supported only by transform, debug, reset, and clean-backup")
		}
		if flag := cmd.Flags().Lookup("recursive"); flag != nil && flag.Value.String() == "true" {
			return fmt.Errorf("--test-file cannot be combined with --recursive")
		}
	}
	if cmd.Flags().Changed("eval") || cf.debugEval != "" {
		if cmd.Name() != "debug" {
			return fmt.Errorf("--eval is supported only by debug")
		}
		if cf.debugEval == "" {
			return fmt.Errorf("--eval must not be empty")
		}
	}
	return nil
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
	rootCmd.PersistentFlags().StringVar(&cf.testFile, "test-file", "", "Select one .tftest.hcl file relative to --tf-dir (transform, debug, reset, clean-backup only)")
	rootCmd.PersistentFlags().StringVar(&cf.debugEval, "eval", "", "Evaluate an HCL expression as JSON without applying transforms (debug only)")
	rootCmd.PersistentFlags().StringSliceVar(&cf.mptfDirs, "mptf-dir", nil, "MPTF directory")

	rootCmd.PersistentFlags().StringArray("mptf-var", cf.mptfVars, "Set a value for one of the input variables in the root module of the configuration. Use this option more than once to set more than one variable.")
	rootCmd.PersistentFlags().StringArray("mptf-var-file", cf.mptfVarFiles, "Load variable values from the given file, in addition to the default files mptf.mptfvars and *.auto.mptfvars. Use this option more than once to include more than one variables file.")
}
