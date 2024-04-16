package terraform

import (
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/hashicorp/hcl/v2/hclwrite"
	"strings"
)

type Block struct {
	*hclsyntax.Block
	WriteBlock   *hclwrite.Block
	Count        *Attribute
	ForEach      *Attribute
	Attributes   map[string]*Attribute
	NestedBlocks []*NestedBlock
	Type         string
	Labels       []string
	Address      string
}

func NewBlock(rb *hclsyntax.Block, wb *hclwrite.Block) *Block {
	b := &Block{
		Type:       rb.Type,
		Labels:     rb.Labels,
		Address:    strings.Join(append([]string{rb.Type}, rb.Labels...), "."),
		Block:      rb,
		WriteBlock: wb,
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

func attributes(rb *hclsyntax.Body, wb *hclwrite.Body) map[string]*Attribute {
	attributes := rb.Attributes
	r := make(map[string]*Attribute, len(attributes))
	for name, attribute := range attributes {
		r[name] = NewAttribute(name, attribute, wb.GetAttribute(name))
	}
	return r
}

func nestedBlocks(rb *hclsyntax.Body, wb *hclwrite.Body) []*NestedBlock {
	blocks := rb.Blocks
	r := make([]*NestedBlock, len(blocks))
	for i, block := range blocks {
		r[i] = NewNestedBlock(block, wb.Blocks()[i])
	}
	return r
}
