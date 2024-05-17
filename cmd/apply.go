package cmd

import (
	"bufio"
	"fmt"
	"github.com/lonegunmanb/mptf/pkg"
	"github.com/spf13/cobra"
	"os"
	"strings"
)

func NewApplyCmd() *cobra.Command {
	var tfDir, mptfDir string
	auto := false

	applyCmd := &cobra.Command{
		Use:   "apply",
		Short: "Apply the plan, mptf apply [-a] [path to config files]",
		RunE: func(cmd *cobra.Command, args []string) error {
			hclBlocks, err := pkg.LoadMPTFHclBlocks(false, mptfDir)
			if err != nil {
				return err
			}
			cfg, err := pkg.NewMetaProgrammingTFConfig(tfDir, hclBlocks, nil, cmd.Context())
			if err != nil {
				return err
			}
			plan, err := pkg.RunMetaProgrammingTFPlan(cfg)
			if err != nil {
				return err
			}
			fmt.Println(plan.String())
			if !auto {
				reader := bufio.NewReader(os.Stdin)
				fmt.Print("Do you want to apply this plan? Only `yes` would be accepted. (yes/no): ")
				text, _ := reader.ReadString('\n')
				text = strings.ToLower(strings.TrimSpace(text))

				if text != "yes" {
					return nil
				}
			}
			err = plan.Apply()
			if err != nil {
				return fmt.Errorf("error applying plan: %s\n", err.Error())
			}
			fmt.Println("Plan applied successfully.")
			return nil
		},
	}

	applyCmd.Flags().BoolVarP(&auto, "auto", "a", false, "Apply fixes without confirmation")
	applyCmd.Flags().StringVar(&tfDir, "tf-dir", "", "Terraform directory")
	applyCmd.Flags().StringVar(&mptfDir, "mptf-dir", "", "MPTF directory")

	return applyCmd
}
