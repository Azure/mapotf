package pkg

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/Azure/golden"
	filesystem "github.com/Azure/mapotf/pkg/fs"
	"github.com/Azure/mapotf/pkg/terraform"
	"github.com/hashicorp/go-multierror"
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/spf13/afero"
	"github.com/zclconf/go-cty/cty/function"
)

var _ golden.Config = &MetaProgrammingTFConfig{}

type MetaProgrammingTFConfig struct {
	*golden.BaseConfig
	resourceBlocks map[string]*terraform.RootBlock
	dataBlocks     map[string]*terraform.RootBlock
	variableBlocks map[string]*terraform.RootBlock
	localBlocks    map[string]*terraform.RootBlock
	outputBlocks   map[string]*terraform.RootBlock
	moduleBlocks   map[string]*terraform.RootBlock
	terraformBlock *terraform.RootBlock
	allRootBlocks  []*terraform.RootBlock
	module         *terraform.Module
}

func NewMetaProgrammingTFConfig(m *TerraformModuleRef, varConfigDir *string, hclBlocks []*golden.HclBlock, cliFlagAssignedVars []golden.CliFlagAssignedVariables, ctx context.Context) (*MetaProgrammingTFConfig, error) {
	baseConfig := golden.NewBasicConfigFromArgs(golden.NewBaseConfigArgs{
		Basedir:                  m.AbsDir,
		DslFullName:              "mapotf",
		DslAbbreviation:          "mptf",
		VarConfigDir:             varConfigDir,
		CliFlagAssignedVariables: cliFlagAssignedVars,
		Ctx:                      ctx,
		IgnoreUnknownVariables:   true,
	})
	baseConfig.OverrideFunctions = map[string]function.Function{
		"tohcl": ToHclFunc,
	}
	cfg := &MetaProgrammingTFConfig{
		BaseConfig: baseConfig,
	}
	if err := cfg.reloadTerraformModule(m); err != nil {
		return nil, err
	}
	return cfg, golden.InitConfig(cfg, hclBlocks)
}

func (c *MetaProgrammingTFConfig) reloadTerraformModule(m *TerraformModuleRef) error {
	module, err := terraform.LoadModule(m.toTerraformPkgType())
	if err != nil {
		return err
	}
	c.resourceBlocks = groupByAddress(module.ResourceBlocks)
	c.dataBlocks = groupByAddress(module.DataBlocks)
	c.moduleBlocks = groupByAddress(module.ModuleBlocks)
	c.variableBlocks = groupByAddress(module.Variables)
	c.outputBlocks = groupByAddress(module.Outputs)
	//for _, localBlock := range module.Locals {
	//	localBlock.WriteBody().AppendNewline()
	//}
	c.localBlocks = groupByAddress(module.Locals)
	if len(module.TerraformBlocks) > 0 {
		c.terraformBlock = module.TerraformBlocks[0]
	}
	c.allRootBlocks = module.Blocks()
	c.module = module
	return nil
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

func (c *MetaProgrammingTFConfig) VariableBlocks() []*terraform.RootBlock {
	return c.slice(c.variableBlocks)
}

func (c *MetaProgrammingTFConfig) OutputBlocks() []*terraform.RootBlock {
	return c.slice(c.outputBlocks)
}

func (c *MetaProgrammingTFConfig) LocalBlocks() []*terraform.RootBlock {
	return c.slice(c.localBlocks)
}

func (c *MetaProgrammingTFConfig) ModuleBlocks() []*terraform.RootBlock {
	return c.slice(c.moduleBlocks)
}

func (c *MetaProgrammingTFConfig) TerraformBlock() *terraform.RootBlock {
	return c.terraformBlock
}

func (c *MetaProgrammingTFConfig) RootBlock(address string) *terraform.RootBlock {
	if strings.HasPrefix(address, "resource.") {
		return c.resourceBlocks[address]
	}
	if strings.HasPrefix(address, "data.") {
		return c.dataBlocks[address]
	}
	if strings.HasPrefix(address, "variable.") {
		return c.variableBlocks[address]
	}
	if strings.HasPrefix(address, "local.") {
		return c.localBlocks[address]
	}
	if strings.HasPrefix(address, "output.") {
		return c.outputBlocks[address]
	}
	if strings.HasPrefix(address, "module.") {
		return c.moduleBlocks[address]
	}
	if address == "terraform" {
		return c.terraformBlock
	}
	return nil
}

func LoadMPTFHclBlocks(ignoreUnsupportedBlock bool, dir string) ([]*golden.HclBlock, error) {
	fs := filesystem.Fs
	matches, err := afero.Glob(fs, filepath.Join(dir, "*.mptf.hcl"))
	if err != nil {
		return nil, err
	}
	if len(matches) == 0 {
		return nil, fmt.Errorf("no `.mptf.hcl` file found at %s", dir)
	}

	var blocks []*golden.HclBlock

	for _, filename := range matches {
		content, fsErr := afero.ReadFile(fs, filename)
		if fsErr != nil {
			err = multierror.Append(err, fsErr)
			continue
		}
		readFile, diag := hclsyntax.ParseConfig(content, filename, hcl.InitialPos)
		if diag.HasErrors() {
			err = multierror.Append(err, diag.Errs()...)
			continue
		}
		writeFile, _ := hclwrite.ParseConfig(content, filename, hcl.InitialPos)
		readBlocks := readFile.Body.(*hclsyntax.Body).Blocks

		writeBlocks := writeFile.Body().Blocks()
		blocks = append(blocks, golden.AsHclBlocks(readBlocks, writeBlocks)...)
	}
	if err != nil {
		return nil, err
	}

	var r []*golden.HclBlock
	for _, b := range blocks {
		if golden.IsBlockTypeWanted(b.Type) {
			r = append(r, b)
			continue
		}
		if !ignoreUnsupportedBlock {
			err = multierror.Append(err, fmt.Errorf("invalid block type: %s %s", b.Type, b.Range().String()))
		}
	}
	return r, err
}

func (c *MetaProgrammingTFConfig) SaveToDisk() error {
	return c.module.SaveToDisk()
}

func ModuleRefs(tfDir string) ([]*TerraformModuleRef, error) {
	moduleManifest := filepath.Join(tfDir, ".terraform", "modules", "modules.json")
	exist, err := afero.Exists(filesystem.Fs, moduleManifest)
	if err != nil {
		return nil, fmt.Errorf("cannot check `modules.json` at %s: %+v", moduleManifest, err)
	}
	if !exist {
		mod, err := NewTerraformRootModuleRef(tfDir)
		if err != nil {
			return nil, err
		}
		return []*TerraformModuleRef{mod}, nil
	}
	var modules = struct {
		Modules []*TerraformModuleRef `json:"Modules"`
	}{}
	manifestJson, err := afero.ReadFile(filesystem.Fs, moduleManifest)
	if err != nil {
		return nil, fmt.Errorf("cannot read `modules.json` at %s: %+v", moduleManifest, err)
	}
	if err = json.Unmarshal(manifestJson, &modules); err != nil {
		return nil, fmt.Errorf("cannot unmarshal `modules.json` at %s: %+v", moduleManifest, err)
	}
	for i, m := range modules.Modules {
		if err := m.Load(); err != nil {
			return nil, fmt.Errorf("cannot load info for %s: %+v", m.Dir, err)
		}
		modules.Modules[i] = m
	}
	return modules.Modules, nil
}

func (c *MetaProgrammingTFConfig) slice(blocks map[string]*terraform.RootBlock) []*terraform.RootBlock {
	var r []*terraform.RootBlock
	for _, b := range blocks {
		r = append(r, b)
	}
	return r
}

func (c *MetaProgrammingTFConfig) AddBlock(filename string, block *hclwrite.Block) {
	c.module.AddBlock(filename, block)
}

func groupByAddress(blocks []*terraform.RootBlock) map[string]*terraform.RootBlock {
	r := make(map[string]*terraform.RootBlock)
	for _, b := range blocks {
		r[b.Address] = b
	}
	return r
}
