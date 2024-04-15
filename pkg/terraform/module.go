package terraform

import (
	"github.com/Azure/golden"
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/spf13/afero"
	"strings"
)

var Fs = afero.NewOsFs()

type Module struct {
	ResourceBlocks []*golden.HclBlock
	DataBlocks     []*golden.HclBlock
}

func (m *Module) LoadConfig(cfg, filename string) error {
	writeFile, diag := hclwrite.ParseConfig([]byte(cfg), filename, hcl.InitialPos)
	if diag.HasErrors() {
		return diag
	}
	readFile, diag := hclsyntax.ParseConfig([]byte(cfg), filename, hcl.InitialPos)
	if diag.HasErrors() {
		return diag
	}
	readBlocks := readFile.Body.(*hclsyntax.Body).Blocks
	writeBlocks := writeFile.Body().Blocks()
	for i, rb := range readBlocks {
		if rb.Type != "resource" && rb.Type != "data" {
			continue
		}
		hclBlock := golden.NewHclBlock(rb, writeBlocks[i], nil)
		if rb.Type == "resource" {
			m.ResourceBlocks = append(m.ResourceBlocks, hclBlock)
		} else if rb.Type == "data" {
			m.DataBlocks = append(m.DataBlocks, hclBlock)
		}
	}
	return nil
}

func (m *Module) LoadModule(dir string) error {
	files, err := afero.ReadDir(Fs, dir)
	if err != nil {
		return err
	}
	for _, f := range files {
		if f.IsDir() {
			continue
		}
		if strings.HasSuffix(f.Name(), ".tf") {
			content, err := afero.ReadFile(Fs, f.Name())
			if err != nil {
				return err
			}
			if err = m.LoadConfig(string(content), f.Name()); err != nil {
				return err
			}
		}
	}
	return nil
}
