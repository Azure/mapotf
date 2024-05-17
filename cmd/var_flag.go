package cmd

import (
	"errors"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/Azure/golden"
)

func varFlags(cmd *cobra.Command, args []string) ([]golden.CliFlagAssignedVariables, error) {
	var flags []golden.CliFlagAssignedVariables
	for i := 0; i < len(args); i++ {
		if args[i] == "-mptf-var" || args[i] == "-mptf-var-file" {
			if i+1 < len(args) {
				if args[i] == "-mptf-var" {
					flags = append(flags, golden.NewCliFlagAssignedVariable(args[i], args[i+1]))
				} else {
					flags = append(flags, golden.NewCliFlagAssignedVariableFile(args[i+1]))
				}
				i++ // skip next arg
			} else {
				return nil, errors.New("missing value for " + args[i])
			}
		}
	}
	cmd.Flags().AddFlag(&pflag.Flag{
		Name:     "mptf-var",
		Usage:    "Set a value for one of the input variables in the root module of the configuration. Use this option more than once to set more than one variable.",
		Value:    nil,
		DefValue: "'foo=bar'",
	})
	cmd.Flags().AddFlag(&pflag.Flag{
		Name:  "mptf-var-file",
		Usage: "Load variable values from the given file, in addition to the default files mptf.mptfvars and *.auto.mptfvars. Use this option more than once to include more than one variables file.",
	})
	return flags, nil
}
