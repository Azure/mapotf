package cmd

import (
	"errors"
	"fmt"
	"os"

	"github.com/Azure/golden"
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/lonegunmanb/mptf/pkg"
	"github.com/peterh/liner"
	"github.com/spf13/cobra"
)

func NewConsoleCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "console",
		Short: "Start REPL mode, grept console [path to config files]",
		FParseErrWhitelist: cobra.FParseErrWhitelist{
			UnknownFlags: true,
		},
		RunE: replFunc(),
	}
}

func replFunc() func(*cobra.Command, []string) error {
	return func(c *cobra.Command, args []string) error {
		tfDir := cf.tfDir
		mptfDir := cf.mptfDirs[0]
		varFlags, err := varFlags(os.Args)
		if err != nil {
			return err
		}
		hclBlocks, err := pkg.LoadMPTFHclBlocks(false, mptfDir)
		if err != nil {
			return err
		}
		cfg, err := pkg.NewMetaProgrammingTFConfig(tfDir, hclBlocks, varFlags, c.Context())
		if err != nil {
			return err
		}
		_, err = pkg.RunMetaProgrammingTFPlan(cfg)
		if err != nil {
			return err
		}
		line := liner.NewLiner()
		defer func() {
			_ = line.Close()
		}()

		line.SetCtrlCAborts(true)
		fmt.Println("Entering debuging mode, press `quit` or `exit` or Ctrl+C to quit.")

		for {
			if input, err := line.Prompt("debug> "); err == nil {
				if input == "quit" || input == "exit" {
					return nil
				}
				line.AppendHistory(input)
				expression, diag := hclsyntax.ParseExpression([]byte(input), "repl.hcl", hcl.InitialPos)
				if diag.HasErrors() {
					fmt.Printf("%s\n", diag.Error())
					continue
				}
				value, diag := expression.Value(cfg.EvalContext())
				if diag.HasErrors() {
					fmt.Printf("%s\n", diag.Error())
					continue
				}
				fmt.Println(golden.CtyValueToString(value))
			} else if errors.Is(err, liner.ErrPromptAborted) {
				fmt.Println("Aborted")
				break
			} else {
				fmt.Println("Error reading line: ", err)
				break
			}
		}

		return nil
	}
}

func init() {
	rootCmd.AddCommand(NewConsoleCmd())
}
