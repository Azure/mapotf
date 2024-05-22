package cmd

import (
	"context"
	"errors"
	"fmt"
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

type localizedMptfDir struct {
	path    string
	dispose func()
}

func (l localizedMptfDir) Dispose() {
	if l.dispose != nil {
		l.dispose()
	}
}

func (c *commonFlags) MptfDirs(ctx context.Context) ([]localizedMptfDir, error) {
	var r []localizedMptfDir
	for _, originalDir := range c.mptfDirs {
		localizedPath, disposeFunc, err := localizeConfigFolder(originalDir, ctx)
		if err != nil {
			for _, localizedDir := range r {
				localizedDir.Dispose()
			}
			return nil, fmt.Errorf("cannot get config path: %s: %+v", originalDir, err)
		}
		r = append(r, localizedMptfDir{path: localizedPath, dispose: disposeFunc})
	}
	return r, nil
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
