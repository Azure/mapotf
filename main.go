package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/lonegunmanb/mptf/cmd"
)

func usage() {
	fmt.Printf("Usage: %s <command> [arguments]\n", os.Args[0])
	fmt.Println(`
The commands are:

plan      Generates a plan based on the specified configuration
apply     Apply the plan
console   Try grept expressions at an interactive command prompt`)
	fmt.Println("\nUse \"grept help [command]\" for more information about a command.")
	os.Exit(0)
}

func main() {
	flag.Usage = usage
	flag.Parse()

	if flag.NArg() == 0 {
		usage()
	}

	command := flag.Arg(0)

	ctx := context.Background()
	var err error

	switch command {
	case "plan":
		cobraCmd := cmd.NewPlanCmd()
		err = cobraCmd.ExecuteContext(ctx)
	case "apply":
		cobraCmd := cmd.NewApplyCmd()
		err = cobraCmd.ExecuteContext(ctx)
	//case "console":
	//	cobraCmd := cmd.NewConsoleCmd()
	//	err = cobraCmd.ExecuteContext(ctx)
	default:
		usage()
	}
	if err != nil {
		fmt.Printf("%+v\n", err)
		os.Exit(1)
	}
}
