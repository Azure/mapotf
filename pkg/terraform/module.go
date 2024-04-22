package terraform

import (
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/spf13/afero"
	"path/filepath"
	"strings"
	"sync"
)

var Fs = afero.NewOsFs()

type Module struct {
	dir            string
	writeFiles     map[string]*hclwrite.File
	lock           *sync.Mutex
	ResourceBlocks []*RootBlock
	DataBlocks     []*RootBlock
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
	m.writeFiles[filename] = writeFile
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
	m := &Module{
		dir:        dir,
		writeFiles: make(map[string]*hclwrite.File),
		lock:       &sync.Mutex{},
	}
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

func (m *Module) SaveToDisk() error {
	m.lock.Lock()
	defer m.lock.Unlock()
	for fn, wf := range m.writeFiles {
		content := wf.Bytes()
		err := afero.WriteFile(Fs, filepath.Join(m.dir, fn), hclwrite.Format(content), 0644)
		if err != nil {
			return err
		}
	}
	return nil
}

func (m *Module) AddBlock(fileName string, block *hclwrite.Block) {
	func() {
		m.lock.Lock()
		defer m.lock.Unlock()
		if _, ok := m.writeFiles[fileName]; !ok {
			m.writeFiles[fileName] = hclwrite.NewFile()
		}
	}()
	writeFile := m.writeFiles[fileName]
	lock.Lock(fileName)
	defer lock.Unlock(fileName)
	tokens := writeFile.Body().BuildTokens(nil)
	if tokens[len(tokens)-1].Type != hclsyntax.TokenNewline {
		writeFile.Body().AppendNewline()
	}
	writeFile.Body().AppendBlock(block)
	writeFile.Body().AppendNewline()
}
