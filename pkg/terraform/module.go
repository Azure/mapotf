package terraform

import (
	"path/filepath"
	"strings"
	"sync"

	"github.com/Azure/mapotf/pkg/backup"
	"github.com/Azure/mapotf/pkg/fs"
	"github.com/ahmetb/go-linq/v3"
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/spf13/afero"
)

var wantedTypes = map[string]func(module *Module) *[]*RootBlock{
	"resource": func(m *Module) *[]*RootBlock {
		return &m.ResourceBlocks
	},
	"data": func(m *Module) *[]*RootBlock {
		return &m.DataBlocks
	},
	"module": func(m *Module) *[]*RootBlock {
		return &m.ModuleBlocks
	},
	"terraform": func(m *Module) *[]*RootBlock {
		return &m.TerraformBlocks
	},
	"variable": func(m *Module) *[]*RootBlock { return &m.Variables },
	"output":   func(m *Module) *[]*RootBlock { return &m.Outputs },
}

type Module struct {
	Dir             string
	AbsDir          string
	writeFiles      map[string]*hclwrite.File
	lock            *sync.Mutex
	ResourceBlocks  []*RootBlock
	DataBlocks      []*RootBlock
	ModuleBlocks    []*RootBlock
	TerraformBlocks []*RootBlock
	Variables       []*RootBlock
	Outputs         []*RootBlock
	Locals          []*RootBlock
	Key             string
	Source          string
	Version         string
	GitHash         string
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
		if rb.Type == "locals" {
			m.loadLocals(rb, writeBlocks[i])
			continue
		}
		getter, want := wantedTypes[rb.Type]
		if !want {
			continue
		}
		hclBlock := NewBlock(m, rb, writeBlocks[i])
		blocks := getter(m)
		*blocks = append(*blocks, hclBlock)
	}
	return nil
}

type ModuleRef struct {
	Key     string `json:"Key"`
	Source  string `json:"Source"`
	Dir     string `json:"Dir"`
	AbsDir  string
	Version string `json:"Version"`
	GitHash string
}

func LoadModule(mr ModuleRef) (*Module, error) {
	files, err := afero.ReadDir(fs.Fs, mr.AbsDir)
	if err != nil {
		return nil, err
	}
	m := &Module{
		Dir:        mr.Dir,
		AbsDir:     mr.AbsDir,
		writeFiles: make(map[string]*hclwrite.File),
		lock:       &sync.Mutex{},
		Key:        mr.Key,
		Source:     mr.Source,
		Version:    mr.Version,
		GitHash:    mr.GitHash,
	}
	for _, f := range files {
		if f.IsDir() {
			continue
		}
		if !strings.HasSuffix(f.Name(), ".tf") || f.Name() == "override.tf" || strings.HasSuffix(f.Name(), "_override.tf") {
			continue
		}
		n := filepath.Join(mr.AbsDir, f.Name())
		content, err := afero.ReadFile(fs.Fs, n)
		if err != nil {
			return nil, err
		}
		if err = m.loadConfig(string(content), f.Name()); err != nil {
			return nil, err
		}
	}
	return m, nil
}

func (m *Module) SaveToDisk() error {
	m.lock.Lock()
	defer m.lock.Unlock()
	for fn, wf := range m.writeFiles {
		absPath := filepath.Join(m.Dir, fn)
		exist, err := afero.Exists(fs.Fs, absPath)
		if err != nil {
			return err
		}
		if !exist {
			absNewFilePath := absPath + backup.NewFileExtension
			err = afero.WriteFile(fs.Fs, absNewFilePath, []byte{}, 0644)
			if err != nil {
				return err
			}
		}
		content := wf.Bytes()
		err = afero.WriteFile(fs.Fs, absPath, hclwrite.Format(content), 0644)
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
	if len(tokens) > 1 && tokens[len(tokens)-1].Type != hclsyntax.TokenNewline {
		writeFile.Body().AppendNewline()
	}
	writeFile.Body().AppendBlock(block)
	writeFile.Body().AppendNewline()
}

func (m *Module) loadLocals(rb *hclsyntax.Block, wb *hclwrite.Block) {
	for attrName, attr := range rb.Body.Attributes {
		rootBlock := NewBlock(m, &hclsyntax.Block{
			Type:   "local",
			Labels: []string{attrName},
			Body: &hclsyntax.Body{
				Attributes: map[string]*hclsyntax.Attribute{
					attrName: attr,
				},
				Blocks:   []*hclsyntax.Block{},
				SrcRange: rb.TypeRange,
				EndRange: rb.CloseBraceRange,
			},
			TypeRange:       rb.TypeRange,
			LabelRanges:     rb.LabelRanges,
			OpenBraceRange:  rb.OpenBraceRange,
			CloseBraceRange: rb.CloseBraceRange,
		}, wb)
		m.Locals = append(m.Locals, rootBlock)
	}
}

func (m *Module) Blocks() []*RootBlock {
	var blocks []*RootBlock
	linq.From(m.TerraformBlocks).Concat(linq.From(m.Locals)).
		Concat(linq.From(m.Outputs)).Concat(linq.From(m.Variables)).
		Concat(linq.From(m.DataBlocks)).Concat(linq.From(m.ResourceBlocks)).
		Concat(linq.From(m.ModuleBlocks)).ToSlice(&blocks)
	return blocks
}
