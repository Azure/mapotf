package cmd

import (
	"errors"
	"fmt"
	"github.com/spf13/cobra"
	"os"
	"os/exec"
)

func NewInitCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Prepare your working directory for other commands",
		FParseErrWhitelist: cobra.FParseErrWhitelist{
			UnknownFlags: true,
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			initArgs := append([]string{"init"}, NonMptfArgs...)
			for _, arg := range initArgs {
				println(arg)
			}
			tfCmd := exec.Command("terraform", initArgs...)
			tfCmd.Stdin = os.Stdin
			tfCmd.Stdout = os.Stdout
			// Run the command and pass through exit code
			if err := tfCmd.Run(); err != nil {
				var pe *exec.ExitError
				if errors.As(err, &pe) {
					os.Exit(pe.ExitCode())
				}
				os.Stderr.WriteString(fmt.Sprintf("Error executing command but could not get exit code: %s\n", err))
				os.Exit(1)
			}
			return nil
		},
	}
}

func init() {
	rootCmd.AddCommand(NewInitCmd())
}
