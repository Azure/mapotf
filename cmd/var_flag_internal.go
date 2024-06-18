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
		if args[i] != "--mptf-var" && args[i] != "--mptf-var-file" {
			continue
		}
		if i+1 == len(args) {
			return nil, errors.New("missing value for " + args[i])
		}
		arg := args[i+1]
		if args[i] == "--mptf-var-file" {
			flags = append(flags, golden.NewCliFlagAssignedVariableFile(arg))
			i++
			continue
		}
		varAssignment := strings.Split(arg, "=")
		if len(varAssignment) != 2 {
			return nil, fmt.Errorf("the given --mptf option \"%s\" is not correctly specified. Must be a variable name and value separated by an equals sign, like --mptf-var key=value", arg)
		}
		flags = append(flags, golden.NewCliFlagAssignedVariable(varAssignment[0], varAssignment[1]))
		i++ // skip next arg
	}
	return flags, nil
}
