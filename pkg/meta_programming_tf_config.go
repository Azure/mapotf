package pkg

import (
	"context"
	"github.com/Azure/golden"
	"github.com/lonegunmanb/mptf/pkg/terraform"
	"strings"
)

var _ golden.Config = &MetaProgrammingTFConfig{}

type MetaProgrammingTFConfig struct {
	*golden.BaseConfig
	tfDir          string
	resourceBlocks map[string]*terraform.RootBlock
	dataBlocks     map[string]*terraform.RootBlock
}

func NewMetaProgrammingTFConfig(tfDir, cfgDir string, ctx context.Context) (*MetaProgrammingTFConfig, error) {
	module, err := terraform.LoadModule(tfDir)
	if err != nil {
		return nil, err
	}
	cfg := &MetaProgrammingTFConfig{
		BaseConfig:     golden.NewBasicConfig(tfDir, ctx),
		tfDir:          tfDir,
		resourceBlocks: groupByType(module.ResourceBlocks),
		dataBlocks:     groupByType(module.DataBlocks),
	}
	return cfg, nil
}

func (c *MetaProgrammingTFConfig) Init(hclBlocks []*golden.HclBlock) error {
	return golden.InitConfig(c, hclBlocks)
}

func (c *MetaProgrammingTFConfig) ResourceBlocks() []*terraform.RootBlock {
	return c.slice(c.resourceBlocks)
}

func (c *MetaProgrammingTFConfig) DataBlocks() []*terraform.RootBlock {
	return c.slice(c.dataBlocks)
}

func (c *MetaProgrammingTFConfig) TerraformBlock(address string) *terraform.RootBlock {
	if strings.HasPrefix(address, "resource.") {
		return c.resourceBlocks[address]
	}
	if strings.HasPrefix(address, "data.") {
		return c.dataBlocks[address]
	}
	return nil
}

func (c *MetaProgrammingTFConfig) slice(blocks map[string]*terraform.RootBlock) []*terraform.RootBlock {
	var r []*terraform.RootBlock
	for _, b := range blocks {
		r = append(r, b)
	}
	return r
}

func groupByType(blocks []*terraform.RootBlock) map[string]*terraform.RootBlock {
	r := make(map[string]*terraform.RootBlock)
	for _, b := range blocks {
		r[b.Address] = b
	}
	return r
}
