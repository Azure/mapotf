package cmd

import "github.com/spf13/cobra"

func FilterArgs(inputArgs []string) ([]string, []string) {
	var mptfArgs, nonMptfArgs []string
	mptfArgs = append(mptfArgs, inputArgs[0])
	var subCommands = make(map[string]struct{})
	for _, cmd := range []*cobra.Command{
		NewTransformCmd(),
		NewPlanCmd(),
		NewConsoleCmd(),
	} {
		subCommands[cmd.Use] = struct{}{}
	}
	mptfVarFlags := map[string]struct{}{
		"--tf-dir":        {},
		"--mptf-dir":      {},
		"--mptf-var":      {},
		"--mptf-var-file": {},
	}
	for i := 0; i < len(inputArgs); i++ {
		arg := inputArgs[i]
		if _, isSubCommand := subCommands[arg]; isSubCommand {
			mptfArgs = append(mptfArgs, arg)
		} else if _, isMptfVarFlag := mptfVarFlags[arg]; isMptfVarFlag {
			mptfArgs = append(mptfArgs, arg)
			mptfArgs = append(mptfArgs, inputArgs[i+1])
			i++
		} else if arg == "-a" {
			mptfArgs = append(mptfArgs, arg)
		} else {
			nonMptfArgs = append(nonMptfArgs, arg)
		}
	}
	return mptfArgs, nonMptfArgs
}