package cmd

import (
	"github.com/spf13/cobra"
	"strings"
)

var NonMptfArgs []string

func FilterArgs(inputArgs []string) ([]string, []string) {
	var mptfArgs, nonMptfArgs []string
	mptfArgs = append(mptfArgs, inputArgs[0])
	inputArgs = inputArgs[1:]
	var subCommands = make(map[string]struct{})
	for _, cmd := range append([]*cobra.Command{
		NewTransformCmd(),
		NewDebugCmd(),
		NewResetCmd(),
		NewClearBackupCmd(),
	}, terraformCommands...) {
		subCommands[cmd.Use] = struct{}{}
	}
	mptfVarFlags := map[string]struct{}{
		"--tf-dir":        {},
		"--mptf-dir":      {},
		"--mptf-var":      {},
		"--mptf-var-file": {},
		"--help":          {},
	}
	mptfShortHands := map[string]struct{}{
		"-r": {},
		"-h": {},
	}
	for i := 0; i < len(inputArgs); i++ {
		arg := inputArgs[i]
		if _, isSubCommand := subCommands[arg]; isSubCommand {
			mptfArgs = append(mptfArgs, arg)
		} else if _, isMptfVarFlag := mptfVarFlags[arg]; isMptfVarFlag {
			mptfArgs = append(mptfArgs, arg)
			if i != len(inputArgs)-1 && !strings.HasPrefix(inputArgs[i+1], "-") {
				mptfArgs = append(mptfArgs, inputArgs[i+1])
				i++
			}
		} else if _, isMptfShorthand := mptfShortHands[arg]; isMptfShorthand {
			mptfArgs = append(mptfArgs, arg)
		} else {
			nonMptfArgs = append(nonMptfArgs, arg)
		}
	}
	return mptfArgs, nonMptfArgs
}
