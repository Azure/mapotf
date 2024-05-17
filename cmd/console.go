package cmd

import (
	"errors"
	"fmt"

	"github.com/Azure/golden"
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/lonegunmanb/mptf/pkg"
	"github.com/peterh/liner"
	"github.com/spf13/cobra"
)

func NewConsoleCmd() *cobra.Command {
	var tfDir, mptfDir string
	replCmd := &cobra.Command{
		Use:   "console",
		Short: "Start REPL mode, grept console [path to config files]",
		RunE:  replFunc(&tfDir, &mptfDir),
	}
	replCmd.Flags().StringVar(&tfDir, "tf-dir", "", "Terraform directory")
	replCmd.Flags().StringVar(&mptfDir, "mptf-dir", "", "MPTF directory")

	return replCmd
}

func replFunc(tfDir, mptfDir *string) func(*cobra.Command, []string) error {
	return func(c *cobra.Command, args []string) error {
		hclBlocks, err := pkg.LoadMPTFHclBlocks(false, *mptfDir)
		if err != nil {
			return err
		}
		cfg, err := pkg.NewMetaProgrammingTFConfig(*tfDir, hclBlocks, nil, c.Context())
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
