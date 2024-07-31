package terraform

import (
	"fmt"
	"sort"
	"strings"

	"github.com/Azure/golden"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/zclconf/go-cty/cty"
)

var _ Block = new(RootBlock)

var RootBlockReflectionInformation = func(v map[string]cty.Value, b *RootBlock) {
	var moduleObj = cty.ObjectVal(map[string]cty.Value{
		"key":      cty.StringVal(""),
		"version":  cty.StringVal(""),
		"source":   cty.StringVal(""),
		"dir":      cty.StringVal(""),
		"abs_dir":  cty.StringVal(""),
		"git_hash": cty.StringVal(""),
	})
	if b.module != nil {
		moduleObj = cty.ObjectVal(map[string]cty.Value{
			"key":      cty.StringVal(b.module.Key),
			"version":  cty.StringVal(b.module.Version),
			"source":   cty.StringVal(b.module.Source),
			"dir":      cty.StringVal(b.module.Dir),
			"abs_dir":  cty.StringVal(b.module.AbsDir),
			"git_hash": cty.StringVal(b.module.GitHash),
		})
	}
	labels := golden.ToCtyValue(b.Labels)
	v["mptf"] = cty.ObjectVal(map[string]cty.Value{
		"block_address":     cty.StringVal(b.Address),
		"terraform_address": cty.StringVal(blockAddressToRef(b.Address)),
		"block_type":        cty.StringVal(b.Type),
		"block_labels":      labels,
		"module":            moduleObj,
		"range": cty.ObjectVal(map[string]cty.Value{
			"file_name":    cty.StringVal(b.Range().Filename),
			"start_line":   cty.NumberIntVal(int64(b.Range().Start.Line)),
			"start_column": cty.NumberIntVal(int64(b.Range().Start.Column)),
			"end_line":     cty.NumberIntVal(int64(b.Range().End.Line)),
			"end_column":   cty.NumberIntVal(int64(b.Range().End.Column)),
		}),
	})
}

func blockAddressToRef(address string) string {
	if strings.HasPrefix(address, "resource.") {
		return strings.TrimPrefix(address, "resource.")
	}
	return address
}

type RootBlock struct {
	*hclsyntax.Block
	module       *Module
	WriteBlock   *hclwrite.Block
	Count        *Attribute
	ForEach      *Attribute
	Attributes   map[string]*Attribute
	NestedBlocks NestedBlocks
	Type         string
	Labels       []string
	Address      string
}

func (b *RootBlock) RemoveContent(path string) {
	unlock := lockBlockFile(b)
	defer unlock()
	segs := strings.Split(path, "/")
	_, ok := b.Attributes[segs[0]]
	if ok {
		b.WriteBody().RemoveAttribute(segs[0])
		return
	}
	nbs, ok := b.NestedBlocks[segs[0]]
	if !ok {
		return
	}
	if len(segs) == 1 {
		for _, nb := range nbs {
			b.WriteBody().RemoveBlock(nb.selfWriteBlock)
		}
		return
	}
	for _, nb := range nbs {
		nb.RemoveContent(strings.Join(segs[1:], "/"))
	}
}

func (b *RootBlock) SetAttributeRaw(name string, tokens hclwrite.Tokens) {
	unlock := lockBlockFile(b)
	defer unlock()
	b.WriteBody().SetAttributeRaw(name, tokens)
}

func (b *RootBlock) AppendBlock(block *hclwrite.Block) {
	unlock := lockBlockFile(b)
	defer unlock()
	b.WriteBody().AppendBlock(block)
}

func (b *RootBlock) WriteBody() *hclwrite.Body {
	return b.WriteBlock.Body()
}

func (b *RootBlock) GetAttributes() map[string]*Attribute {
	return b.Attributes
}

func (b *RootBlock) GetNestedBlocks() NestedBlocks {
	return b.NestedBlocks
}

func NewBlock(m *Module, rb *hclsyntax.Block, wb *hclwrite.Block) *RootBlock {
	b := &RootBlock{
		Type:       rb.Type,
		Labels:     rb.Labels,
		Address:    strings.Join(append([]string{rb.Type}, rb.Labels...), "."),
		Block:      rb,
		WriteBlock: wb,
		module:     m,
	}
	if countAttr, ok := rb.Body.Attributes["count"]; ok {
		b.Count = NewAttribute("count", countAttr, wb.Body().GetAttribute("count"))
	}
	if forEachAttr, ok := rb.Body.Attributes["for_each"]; ok {
		b.ForEach = NewAttribute("for_each", forEachAttr, wb.Body().GetAttribute("for_each"))
	}
	b.Attributes = attributes(rb.Body, wb.Body())
	b.NestedBlocks = nestedBlocks(rb.Body, wb.Body())
	return b
}

func (b *RootBlock) EvalContext() cty.Value {
	v := map[string]cty.Value{}
	RootBlockReflectionInformation(v, b)
	for n, a := range b.Attributes {
		v[n] = cty.StringVal(a.String())
	}
	if b.Count != nil {
		v["count"] = cty.StringVal(b.Count.String())
	}
	if b.ForEach != nil {
		v["for_each"] = cty.StringVal(b.ForEach.String())
	}
	for k, values := range b.NestedBlocks.Values() {
		v[k] = values
	}
	return cty.ObjectVal(v)
}

func (b *RootBlock) dagAddress() string {
	switch b.Type {
	case "resource":
		{
			return fmt.Sprintf("resource.%s.%s", b.Labels[0], b.Labels[1])
		}
	case "data":
		{
			return fmt.Sprintf("data.%s.%s", b.Labels[0], b.Labels[1])
		}
	case "locals":
		{
			for attrName, _ := range b.Attributes {
				return fmt.Sprintf("local.%s", attrName)
			}
		}
	case "module":
		{
			return fmt.Sprintf("module.%s", b.Labels[0])
		}
	case "variable":
		{
			return fmt.Sprintf("var.%s", b.Labels[0])
		}
	case "output":
		{
			return fmt.Sprintf("output.%s", b.Labels[0])
		}
	case "terraform":
		{
			return "terraform"
		}
	}
	return ""
}

func attributes(rb *hclsyntax.Body, wb *hclwrite.Body) map[string]*Attribute {
	attributes := rb.Attributes
	r := make(map[string]*Attribute, len(attributes))
	for name, attribute := range attributes {
		r[name] = NewAttribute(name, attribute, wb.GetAttribute(name))
	}
	return r
}

func nestedBlocks(rb *hclsyntax.Body, wb *hclwrite.Body) NestedBlocks {
	blocks := rb.Blocks
	r := make(map[string][]*NestedBlock)
	for i, block := range blocks {
		nb := NewNestedBlock(block, wb.Blocks()[i])
		r[nb.Type] = append(r[nb.Type], nb)
	}
	for _, v := range r {
		sort.Slice(v, func(i, j int) bool {
			return v[i].Block.Range().Start.Line < v[j].Block.Range().Start.Line
		})
	}
	return r
}
