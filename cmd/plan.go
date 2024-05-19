package cmd

import (
	"fmt"
	"os"

	"github.com/lonegunmanb/mptf/pkg"
	"github.com/spf13/cobra"
)

func NewPlanCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "plan",
		Short: "Generates a plan based on the specified configuration",
		FParseErrWhitelist: cobra.FParseErrWhitelist{
			UnknownFlags: true,
		},
		RunE: func(cmd *cobra.Command, args []string) error {
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

	return cmd
}

func init() {
	rootCmd.AddCommand(NewPlanCmd())
}
