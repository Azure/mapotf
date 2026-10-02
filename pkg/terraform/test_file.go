package terraform

import (
	"bytes"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/Azure/mapotf/pkg/fs"
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/spf13/afero"
	"github.com/zclconf/go-cty/cty"
)

// TestFilePath resolves one test file relative to its owning module.
func TestFilePath(moduleDir, filename string) (string, error) {
	if moduleDir == "" {
		return "", fmt.Errorf("test-file mode requires an owning module directory")
	}
	if !filepath.IsLocal(filename) || !strings.HasSuffix(filename, ".tftest.hcl") {
		return "", fmt.Errorf("test file %q must be a relative .tftest.hcl path within the owning module", filename)
	}
	return filepath.Join(moduleDir, filename), nil
}

func loadTestFile(mr ModuleRef) (*Module, error) {
	path, err := TestFilePath(mr.AbsDir, mr.TestFile)
	if err != nil {
		return nil, err
	}
	filename := filepath.Clean(mr.TestFile)
	content, err := afero.ReadFile(fs.Fs, path)
	if err != nil {
		return nil, fmt.Errorf("cannot read test file %q: %w", path, err)
	}
	body, err := testFileSyntax(content, filename)
	if err != nil {
		return nil, err
	}
	writeFile, diag := hclwrite.ParseConfig(expandCompactTestBlocks(content, body), filename, hcl.InitialPos)
	if diag.HasErrors() {
		return nil, diag
	}
	m := &Module{
		Dir: mr.Dir, AbsDir: mr.AbsDir, Key: mr.Key, Source: mr.Source,
		Version: mr.Version, GitHash: mr.GitHash, TestFile: filename,
		writeFiles: map[string]*hclwrite.File{filename: writeFile},
		lock:       &sync.Mutex{},
	}
	for i, rb := range body.Blocks {
		if rb.Type != "variables" && rb.Type != "run" && rb.Type != "mock_provider" && rb.Type != "provider" {
			continue
		}
		block := NewBlock(m, rb, writeFile.Body().Blocks()[i])
		address, err := testBlockAddress(rb)
		if err != nil {
			return nil, err
		}
		block.Address = address
		m.TestBlocks = append(m.TestBlocks, block)
	}
	return m, nil
}

// Expand compact bodies at parsed brace boundaries so hclwrite can add attributes
// without discarding comments or creating invalid single-line blocks.
func expandCompactTestBlocks(content []byte, body *hclsyntax.Body) []byte {
	var positions []int
	var visit func(*hclsyntax.Body)
	visit = func(body *hclsyntax.Body) {
		for _, block := range body.Blocks {
			if block.OpenBraceRange.Start.Line == block.CloseBraceRange.Start.Line && len(block.Body.Attributes) > 0 {
				positions = append(positions, block.OpenBraceRange.End.Byte, block.CloseBraceRange.Start.Byte)
			}
			visit(block.Body)
		}
	}
	visit(body)
	sort.Ints(positions)
	var expanded bytes.Buffer
	start := 0
	for _, position := range positions {
		expanded.Write(content[start:position])
		expanded.WriteByte('\n')
		start = position
	}
	expanded.Write(content[start:])
	return expanded.Bytes()
}

func testFileSyntax(content []byte, filename string) (*hclsyntax.Body, error) {
	file, diag := hclsyntax.ParseConfig(content, filename, hcl.InitialPos)
	if diag.HasErrors() {
		return nil, diag
	}
	body := file.Body.(*hclsyntax.Body)
	if len(body.Attributes) != 0 {
		return nil, fmt.Errorf("%s: test files cannot contain top-level attributes", filename)
	}
	seen := make(map[string]bool)
	for _, block := range body.Blocks {
		address, err := testBlockAddress(block)
		if err != nil {
			return nil, err
		}
		if address == "" {
			continue
		}
		identity := strings.TrimPrefix(address, "mock_")
		if seen[identity] {
			return nil, fmt.Errorf("%s: duplicate test block %q", block.Range(), address)
		}
		seen[identity] = true
	}
	return body, nil
}

func testBlockAddress(block *hclsyntax.Block) (string, error) {
	labelCount := 0
	switch block.Type {
	case "run", "mock_provider", "provider":
		labelCount = 1
	case "variables", "test", "override_resource", "override_data", "override_module":
	default:
		return "", fmt.Errorf("%s: unsupported test block %q", block.Range(), block.Type)
	}
	if len(block.Labels) != labelCount {
		return "", fmt.Errorf("%s: test block %q requires %d labels", block.Range(), block.Type, labelCount)
	}
	for _, label := range block.Labels {
		if !hclsyntax.ValidIdentifier(label) {
			return "", fmt.Errorf("%s: invalid %s label %q", block.Range(), block.Type, label)
		}
	}
	address := strings.Join(append([]string{block.Type}, block.Labels...), ".")
	switch block.Type {
	case "variables":
		if len(block.Body.Blocks) != 0 {
			return "", fmt.Errorf("%s: variables must contain only attributes", block.Range())
		}
	case "run":
		seen := make(map[string]bool)
		for _, nested := range block.Body.Blocks {
			switch nested.Type {
			case "variables", "module", "plan_options":
				if seen[nested.Type] {
					return "", fmt.Errorf("%s: duplicate %s block in %s", nested.Range(), nested.Type, address)
				}
				seen[nested.Type] = true
			case "assert", "override_resource", "override_data", "override_module":
			default:
				return "", fmt.Errorf("%s: unsupported block %q in %s", nested.Range(), nested.Type, address)
			}
			if len(nested.Labels) != 0 || len(nested.Body.Blocks) != 0 {
				return "", fmt.Errorf("%s: %s in %s must be unlabeled and contain only attributes", nested.Range(), nested.Type, address)
			}
			if nested.Type == "module" {
				if _, err := testLiteralString(nested.Body.Attributes["source"], "module source", nested.Range()); err != nil {
					return "", err
				}
			}
		}
	case "provider", "mock_provider":
		if alias, ok := block.Body.Attributes["alias"]; ok {
			value, err := testLiteralString(alias, "provider alias", alias.Range())
			if err != nil {
				return "", err
			}
			if !hclsyntax.ValidIdentifier(value) {
				return "", fmt.Errorf("%s: invalid provider alias %q", alias.Range(), value)
			}
			address += "." + value
		}
		if block.Type == "mock_provider" {
			for _, nested := range block.Body.Blocks {
				switch nested.Type {
				case "mock_resource", "mock_data":
					if len(nested.Labels) != 1 || !hclsyntax.ValidIdentifier(nested.Labels[0]) {
						return "", fmt.Errorf("%s: %s requires one type label", nested.Range(), nested.Type)
					}
				case "override_resource", "override_data", "override_module":
					if len(nested.Labels) != 0 {
						return "", fmt.Errorf("%s: %s must be unlabeled", nested.Range(), nested.Type)
					}
				default:
					return "", fmt.Errorf("%s: unsupported mock provider block %q", nested.Range(), nested.Type)
				}
				if len(nested.Body.Blocks) != 0 {
					return "", fmt.Errorf("%s: %s must contain only attributes", nested.Range(), nested.Type)
				}
			}
		}
	case "override_resource", "override_data", "override_module":
		return "", nil
	}
	return address, nil
}

func testLiteralString(attribute *hclsyntax.Attribute, description string, sourceRange hcl.Range) (string, error) {
	if attribute != nil {
		value, diag := attribute.Expr.Value(nil)
		if !diag.HasErrors() && value.IsKnown() && !value.IsNull() && value.Type() == cty.String && strings.TrimSpace(value.AsString()) != "" {
			return value.AsString(), nil
		}
	}
	return "", fmt.Errorf("%s: %s must be a known, non-empty string", sourceRange, description)
}

func testBlockValue(block Block, labels []string, blockType string, metadata cty.Value) cty.Value {
	values := make(map[string]cty.Value)
	attributes := make(map[string]cty.Value)
	for name, attribute := range block.GetAttributes() {
		attributes[name] = evalAttributeValue(attribute)
		values[name] = attributes[name]
	}
	for name, blocks := range block.GetNestedBlocks() {
		nested := make([]cty.Value, 0, len(blocks))
		for _, b := range blocks {
			nested = append(nested, testBlockValue(b, b.Labels, b.Type, b.MptfObject()))
		}
		values[name] = cty.TupleVal(nested)
	}
	reflection := metadata.AsValueMap()
	reflection["attributes"] = cty.ObjectVal(attributes)
	reflection["block_type"] = cty.StringVal(blockType)
	labelValues := make([]cty.Value, 0, len(labels))
	for _, label := range labels {
		labelValues = append(labelValues, cty.StringVal(label))
	}
	reflection["block_labels"] = cty.TupleVal(labelValues)
	reflection["is_empty"] = cty.BoolVal(len(attributes) == 0 && len(block.GetNestedBlocks()) == 0)
	values["mptf"] = cty.ObjectVal(reflection)
	return cty.ObjectVal(values)
}

func testNestedBlocks(read *hclsyntax.Body, write *hclwrite.Body) NestedBlocks {
	result := make(NestedBlocks)
	for i, block := range read.Blocks {
		wb := write.Blocks()[i]
		nested := &NestedBlock{
			Type: block.Type, Block: block, selfWriteBlock: wb, WriteBlock: wb,
			Attributes:   attributes(block.Body, wb.Body()),
			NestedBlocks: testNestedBlocks(block.Body, wb.Body()),
		}
		result[block.Type] = append(result[block.Type], nested)
	}
	return result
}
