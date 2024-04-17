package pkg

import (
	"context"
	"github.com/Azure/golden"
	"github.com/lonegunmanb/mptf/pkg/terraform"
)

var _ golden.Config = &MetaProgrammingTFConfig{}

type MetaProgrammingTFConfig struct {
	*golden.BaseConfig
	tfDir          string
	ResourceBlocks []*terraform.Block
	DataBlocks     []*terraform.Block
}

func NewMetaProgrammingTFConfig(tfDir, cfgDir string, ctx context.Context) (golden.Config, error) {
	module, err := terraform.LoadModule(tfDir)
	if err != nil {
		return nil, err
	}
	cfg := &MetaProgrammingTFConfig{
		BaseConfig:     golden.NewBasicConfig(tfDir, ctx),
		tfDir:          tfDir,
		ResourceBlocks: module.ResourceBlocks,
		DataBlocks:     module.DataBlocks,
	}
	return cfg, nil
}
