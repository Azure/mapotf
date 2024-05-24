package main

import (
	"os"

	"github.com/Azure/mapotf/cmd"
)

func main() {
	mptfArgs, nonMptfArgs := cmd.FilterArgs(os.Args)
	os.Args = mptfArgs
	cmd.NonMptfArgs = nonMptfArgs
	cmd.Execute()
}
