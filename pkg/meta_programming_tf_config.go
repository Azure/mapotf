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
	resourceBlocks map[string]*terraform.Block
	dataBlocks     map[string]*terraform.Block
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

func (c *MetaProgrammingTFConfig) ResourceBlocks() []*terraform.Block {
	return c.slice(c.resourceBlocks)
}

func (c *MetaProgrammingTFConfig) DataBlocks() []*terraform.Block {
	return c.slice(c.dataBlocks)
}

func (c *MetaProgrammingTFConfig) TerraformBlock(address string) *terraform.Block {
	if strings.HasPrefix(address, "resource.") {
		return c.resourceBlocks[address]
	}
	if strings.HasPrefix(address, "data.") {
		return c.dataBlocks[address]
	}
	return nil
}

func (c *MetaProgrammingTFConfig) slice(blocks map[string]*terraform.Block) []*terraform.Block {
	var r []*terraform.Block
	for _, b := range blocks {
		r = append(r, b)
	}
	return r
}

func groupByType(blocks []*terraform.Block) map[string]*terraform.Block {
	r := make(map[string]*terraform.Block)
	for _, b := range blocks {
		r[b.Address] = b
	}
	return r
}
