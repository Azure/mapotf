package terraform

import (
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/hashicorp/hcl/v2/hclwrite"
)

type NestedBlock struct {
	Type string
	*hclsyntax.Block
	WriteBlock   *hclwrite.Block
	ForEach      *Attribute
	Attributes   map[string]*Attribute
	NestedBlocks []*NestedBlock
}

func NewNestedBlock(rb *hclsyntax.Block, wb *hclwrite.Block) *NestedBlock {
	if rb.Type == "dynamic" {
		return dynamicBlock(rb, wb)
	}
	return &NestedBlock{
		Type:         rb.Type,
		Block:        rb,
		WriteBlock:   wb,
		Attributes:   attributes(rb.Body, wb.Body()),
		NestedBlocks: nestedBlocks(rb.Body, wb.Body()),
	}
}

func dynamicBlock(rb *hclsyntax.Block, wb *hclwrite.Block) *NestedBlock {
	return &NestedBlock{
		Type:         rb.Labels[0],
		Block:        rb.Body.Blocks[0],
		WriteBlock:   wb.Body().Blocks()[0],
		ForEach:      NewAttribute("for_each", rb.Body.Attributes["for_each"], wb.Body().GetAttribute("for_each")),
		Attributes:   attributes(rb.Body.Blocks[0].Body, wb.Body().Blocks()[0].Body()),
		NestedBlocks: nestedBlocks(rb.Body.Blocks[0].Body, wb.Body().Blocks()[0].Body()),
	}
}
