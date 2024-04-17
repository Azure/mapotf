package terraform

import (
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/spf13/afero"
	"path/filepath"
	"strings"
)

var Fs = afero.NewOsFs()

type Module struct {
	ResourceBlocks []*Block
	DataBlocks     []*Block
}

func (m *Module) loadConfig(cfg, filename string) error {
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
		hclBlock := NewBlock(rb, writeBlocks[i])
		if rb.Type == "resource" {
			m.ResourceBlocks = append(m.ResourceBlocks, hclBlock)
		} else if rb.Type == "data" {
			m.DataBlocks = append(m.DataBlocks, hclBlock)
		}
	}
	return nil
}

func LoadModule(dir string) (*Module, error) {
	files, err := afero.ReadDir(Fs, dir)
	if err != nil {
		return nil, err
	}
	m := new(Module)
	for _, f := range files {
		if f.IsDir() {
			continue
		}
		if !strings.HasSuffix(f.Name(), ".tf") {
			continue
		}
		n := filepath.Join(dir, f.Name())
		content, err := afero.ReadFile(Fs, n)
		if err != nil {
			return nil, err
		}
		if err = m.loadConfig(string(content), f.Name()); err != nil {
			return nil, err
		}
	}

	return m, err
}
