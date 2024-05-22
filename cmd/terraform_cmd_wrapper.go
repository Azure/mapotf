package cmd

import (
	"errors"
	"fmt"
	"github.com/spf13/cobra"
	"os"
	"os/exec"
)

func wrapTerraformCommand(cmd string) func(*cobra.Command, []string) error {
	return func(*cobra.Command, []string) error {
		tfArgs := append([]string{cmd}, NonMptfArgs...)
		for _, arg := range tfArgs {
			println(arg)
		}
		tfCmd := exec.Command("terraform", tfArgs...)
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
	}
}
