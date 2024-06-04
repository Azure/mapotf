package cmd

import (
	"github.com/spf13/cobra"
	"os"
	"os/exec"
)

func wrapTerraformCommandWithEphemeralTransform(tfDir, tfCmd string, recursive *bool) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) error {
		restores, err := transform(*recursive, cmd.Context())
		if err != nil {
			return err
		}
		for _, restore := range restores {
			r := restore
			defer r()
		}
		return wrapTerraformCommand(tfDir, tfCmd)(cmd, args)
	}
}

func wrapTerraformCommand(tfDir, cmd string) func(*cobra.Command, []string) error {
	return func(c *cobra.Command, args []string) error {
		tfArgs := append([]string{cmd}, NonMptfArgs...)
		tfCmd := exec.CommandContext(c.Context(), "terraform", tfArgs...)
		tfCmd.Dir = tfDir
		tfCmd.Stdin = os.Stdin
		tfCmd.Stdout = os.Stdout
		tfCmd.Stderr = os.Stderr
		// Run the command and pass through exit code
		err := tfCmd.Run()
		return err
	}
}
