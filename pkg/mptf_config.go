package pkg

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/Azure/golden"
	"github.com/hashicorp/go-multierror"
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/lonegunmanb/mptf/pkg/terraform"
	"github.com/spf13/afero"
	"path/filepath"
	"strings"
)

var _ golden.Config = &MetaProgrammingTFConfig{}
var MPTFFs = afero.NewOsFs()

type MetaProgrammingTFConfig struct {
	*golden.BaseConfig
	tfDir          string
	resourceBlocks map[string]*terraform.RootBlock
	dataBlocks     map[string]*terraform.RootBlock
	module         *terraform.Module
}

func NewMetaProgrammingTFConfig(tfDir string, hclBlocks []*golden.HclBlock, cliFlagAssignedVars []golden.CliFlagAssignedVariables, ctx context.Context) (*MetaProgrammingTFConfig, error) {
	module, err := terraform.LoadModule(tfDir)
	if err != nil {
		return nil, err
	}
	cfg := &MetaProgrammingTFConfig{
		BaseConfig:     golden.NewBasicConfig(tfDir, "mptf", "mptf", cliFlagAssignedVars, ctx),
		tfDir:          tfDir,
		resourceBlocks: groupByType(module.ResourceBlocks),
		dataBlocks:     groupByType(module.DataBlocks),
		module:         module,
	}
	return cfg, golden.InitConfig(cfg, hclBlocks)
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

func LoadMPTFHclBlocks(ignoreUnsupportedBlock bool, dir string) ([]*golden.HclBlock, error) {
	fs := MPTFFs
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

func ModulePaths(tfDir string) ([]string, error) {
	moduleManifest := filepath.Join(tfDir, ".terraform", "modules", "modules.json")
	exist, err := afero.Exists(MPTFFs, moduleManifest)
	if err != nil {
		return nil, fmt.Errorf("cannot check `modules.json` at %s: %+v", moduleManifest, err)
	}
	if !exist {
		absDir, err := filepath.Abs(tfDir)
		if err != nil {
			return nil, fmt.Errorf("cannot get abs dir for %s: %+v", tfDir, err)
		}
		return []string{absDir}, nil
	}
	var modules = struct {
		Modules []struct {
			Key    string `json:"key"`
			Source string `json:"Source"`
			Dir    string `json:"Dir"`
		} `json:"Modules"`
	}{}
	manifestJson, err := afero.ReadFile(MPTFFs, moduleManifest)
	if err != nil {
		return nil, fmt.Errorf("cannot read `modules.json` at %s: %+v", moduleManifest, err)
	}
	if err = json.Unmarshal(manifestJson, &modules); err != nil {
		return nil, fmt.Errorf("cannot unmarshal `modules.json` at %s: %+v", moduleManifest, err)
	}
	var paths []string
	for _, m := range modules.Modules {
		dir := m.Dir
		absDir, err := filepath.Abs(dir)
		if err != nil {
			return nil, fmt.Errorf("cannot get abs dir for %s: %+v", dir, err)
		}
		paths = append(paths, absDir)
	}
	return paths, nil
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

func groupByType(blocks []*terraform.RootBlock) map[string]*terraform.RootBlock {
	r := make(map[string]*terraform.RootBlock)
	for _, b := range blocks {
		r[b.Address] = b
	}
	return r
}
