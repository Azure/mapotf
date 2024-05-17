package cmd

import (
	"fmt"

	"github.com/lonegunmanb/mptf/pkg"
	"github.com/spf13/cobra"
)

func NewPlanCmd() *cobra.Command {
	var tfDir, mptfDir string

	cmd := &cobra.Command{
		Use:   "plan",
		Short: "Generates a plan based on the specified configuration",
		RunE: func(cmd *cobra.Command, args []string) error {
			varFlags, err := varFlags(cmd, args)
			if err != nil {
				return err
			}
			hclBlocks, err := pkg.LoadMPTFHclBlocks(false, mptfDir)
			if err != nil {
				return err
			}
			cfg, err := pkg.NewMetaProgrammingTFConfig(tfDir, hclBlocks, varFlags, cmd.Context())
			if err != nil {
				return err
			}
			plan, err := pkg.RunMetaProgrammingTFPlan(cfg)
			if err != nil {
				return err
			}
			fmt.Println(plan.String())
			return nil
		},
	}

	cmd.Flags().StringVar(&tfDir, "tf-dir", "", "Terraform directory")
	cmd.Flags().StringVar(&mptfDir, "mptf-dir", "", "MPTF directory")

	return cmd
}
