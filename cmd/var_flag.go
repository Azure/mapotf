package cmd

import (
	"errors"
	"github.com/Azure/golden"
)

func processOrderedVarFlags(args []string) ([]golden.CliFlagAssignedVariables, error) {
	var flags []golden.CliFlagAssignedVariables
	for i := 0; i < len(args); i++ {
		if args[i] == "-mptf-var" || args[i] == "-mptf-var-file" {
			if i+1 < len(args) {

				flags = append(flags, Flag{Name: args[i], Value: args[i+1]})
				i++ // skip next arg
			} else {
				return nil, errors.New("missing value for " + args[i])
			}
		}
	}
	return flags, nil
}
