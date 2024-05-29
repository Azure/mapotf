package cmd

import (
	"context"
	"fmt"
	"github.com/Azure/golden"
	"github.com/Azure/mapotf/pkg"
	"github.com/Azure/mapotf/pkg/backup"
	"github.com/spf13/cobra"
	"os"
)

func NewTransformCmd() *cobra.Command {
	recursive := false

	transformCmd := &cobra.Command{
		Use:   "transform",
		Short: "Apply the transforms, mapotf transform [-r] --tf-dir [] --mptf-dir  [path to config files], support mutilple mptf dirs",
		FParseErrWhitelist: cobra.FParseErrWhitelist{
			UnknownFlags: true,
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			_, err := transform(recursive, cmd.Context())
			return err
		},
	}

	transformCmd.Flags().BoolVarP(&recursive, "recursive", "r", false, "Apply transforms to all modules or not, default to the root module only.")
	return transformCmd
}

func transform(recursive bool, ctx context.Context) ([]func(), error) {
	varFlags, err := varFlags(os.Args)
	var restore []func()
	if err != nil {
		return nil, err
	}
	rootMod, err := pkg.NewTerraformRootModuleRef(".")
	moduleRefs := []*pkg.TerraformModuleRef{
		rootMod,
	}
	if recursive {
		modulePaths, err := pkg.ModuleRefs(cf.tfDir)
		if err != nil {
			return nil, err
		}
		moduleRefs = modulePaths
	}
	for _, moduleRef := range moduleRefs {
		d := moduleRef
		err = backup.BackupFolder(d.AbsDir)
		restore = append(restore, func() {
			_ = backup.RestoreBackup(d.AbsDir)
		})
		if err != nil {
			return restore, err
		}
	}
	var mptfDirs []string
	for _, dir := range cf.mptfDirs {
		localizedDir, dispose, err := localizeConfigFolder(dir, ctx)
		if err != nil {
			return restore, err
		}
		if dispose != nil {
			defer dispose()
		}
		mptfDirs = append(mptfDirs, localizedDir)
	}
	for _, mptfDir := range mptfDirs {
		hclBlocks, err := pkg.LoadMPTFHclBlocks(false, mptfDir)
		if err != nil {
			return nil, err
		}
		for _, tfDir := range moduleRefs {
			err = applyTransform(tfDir, hclBlocks, varFlags, ctx)
			if err != nil {
				return nil, err
			}
		}
	}
	fmt.Println("Transforms applied successfully.")
	return restore, nil
}

func applyTransform(m *pkg.TerraformModuleRef, hclBlocks []*golden.HclBlock, varFlags []golden.CliFlagAssignedVariables, ctx context.Context) error {
	cfg, err := pkg.NewMetaProgrammingTFConfig(m, &cf.tfDir, hclBlocks, varFlags, ctx)
	if err != nil {
		return err
	}
	plan, err := pkg.RunMetaProgrammingTFPlan(cfg)
	if err != nil {
		return err
	}
	if len(plan.Transforms) == 0 {
		fmt.Println("No transforms to apply.")
		return nil
	}
	fmt.Println(plan.String())
	err = plan.Apply()
	if err != nil {
		return fmt.Errorf("error applying plan: %s\n", err.Error())
	}
	return nil
}

func init() {
	rootCmd.AddCommand(NewTransformCmd())
}
