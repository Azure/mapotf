package cmd

import (
	"errors"
	"github.com/Azure/golden"
	"strings"
)

var cf = &commonFlags{}

type commonFlags struct {
	tfDir        string
	mptfDirs     []string
	mptfVars     []string
	mptfVarFiles []string
}

func varFlags(args []string) ([]golden.CliFlagAssignedVariables, error) {
	var flags []golden.CliFlagAssignedVariables
	for i := 0; i < len(args); i++ {
		if args[i] == "--mptf-var" || args[i] == "--mptf-var-file" {
			if i+1 < len(args) {
				arg := args[i+1]
				if args[i] == "--mptf-var" {
					varAssignment := strings.Split(arg, "=")
					flags = append(flags, golden.NewCliFlagAssignedVariable(varAssignment[0], varAssignment[1]))
				} else {
					flags = append(flags, golden.NewCliFlagAssignedVariableFile(arg))
				}
				i++ // skip next arg
			} else {
				return nil, errors.New("missing value for " + args[i])
			}
		}
	}
	return flags, nil
}
