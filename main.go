package main

import (
	"github.com/lonegunmanb/mptf/cmd"
	"os"
)

func main() {
	mptfArgs, nonMptfArgs := cmd.FilterArgs(os.Args)
	os.Args = mptfArgs
	cmd.NonMptfArgs = nonMptfArgs
	cmd.Execute()
}
