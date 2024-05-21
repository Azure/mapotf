package main

import (
	"github.com/lonegunmanb/mptf/cmd"
	"os"
)

func main() {
	os.Args, _ = cmd.FilterArgs(os.Args)
	cmd.Execute()
}
