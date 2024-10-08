package terraform

import (
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/zclconf/go-cty/cty"
	"strings"
)

var _ Block = new(NestedBlock)

type NestedBlocks map[string][]*NestedBlock

type NestedBlock struct {
	Type string
	*hclsyntax.Block
	selfWriteBlock *hclwrite.Block
	WriteBlock     *hclwrite.Block
	ForEach        *Attribute
	Iterator       *Attribute
	Attributes     map[string]*Attribute
	NestedBlocks   NestedBlocks
}

func (nb *NestedBlock) RemoveContent(path string) {
	segs := strings.Split(path, "/")
	current := segs[0]

	if len(segs) > 1 {
		myNbs, ok := nb.NestedBlocks[current]
		if !ok {
			return
		}
		nextPath := strings.Join(segs[1:], "/")
		for _, myNb := range myNbs {
			myNb.RemoveContent(nextPath)
		}
		return
	}
	_, ok := nb.Attributes[current]
	if ok {
		nb.WriteBody().RemoveAttribute(current)
		return
	}
	myNbs, ok := nb.NestedBlocks[current]
	if !ok {
		return
	}
	block := nb.WriteBlock
	if nb.Type == "dynamic" {
		contentBlock := nb.WriteBlock.Body().Blocks()[0]
		block = contentBlock
	}
	for _, myNb := range myNbs {
		block.Body().RemoveBlock(myNb.selfWriteBlock)
	}
}

func (nb *NestedBlock) SetAttributeRaw(name string, tokens hclwrite.Tokens) {
	unlock := lockBlockFile(nb)
	defer unlock()
	nb.WriteBody().SetAttributeRaw(name, tokens)
}

func (nb *NestedBlock) AppendBlock(block *hclwrite.Block) {
	unlock := lockBlockFile(nb)
	defer unlock()
	nb.WriteBody().AppendBlock(block)
}

func (nb *NestedBlock) WriteBody() *hclwrite.Body {
	return nb.WriteBlock.Body()
}

func (nb *NestedBlock) GetAttributes() map[string]*Attribute {
	return nb.Attributes
}

func (nb *NestedBlock) GetNestedBlocks() NestedBlocks {
	return nb.NestedBlocks
}

func NewNestedBlock(rb *hclsyntax.Block, wb *hclwrite.Block) *NestedBlock {
	if rb.Type == "dynamic" {
		return dynamicNestedBlock(rb, wb)
	}
	return staticNestedBlock(rb, wb)
}

func (nb *NestedBlock) EvalContext() cty.Value {
	v := map[string]cty.Value{}
	for n, a := range nb.Attributes {
		v[n] = cty.StringVal(a.String())
	}
	if nb.ForEach != nil {
		v["for_each"] = cty.StringVal(nb.ForEach.String())
	}
	if nb.Iterator != nil {
		v["iterator"] = cty.StringVal(nb.Iterator.String())
	}
	for k, nbv := range nb.NestedBlocks.Values() {
		v[k] = nbv
	}
	v["mptf"] = nb.MptfObject()

	return cty.ObjectVal(v)
}

func (nb *NestedBlock) String() string {
	return string(nb.selfWriteBlock.BuildTokens(nil).Bytes())
}

func (nb *NestedBlock) MptfObject() cty.Value {
	v := map[string]cty.Value{}
	v["tostring"] = cty.StringVal(nb.String())
	v["range"] = cty.ObjectVal(map[string]cty.Value{
		"file_name":    cty.StringVal(nb.Range().Filename),
		"start_line":   cty.NumberIntVal(int64(nb.Range().Start.Line)),
		"start_column": cty.NumberIntVal(int64(nb.Range().Start.Column)),
		"end_line":     cty.NumberIntVal(int64(nb.Range().End.Line)),
		"end_column":   cty.NumberIntVal(int64(nb.Range().End.Column)),
	})
	return cty.ObjectVal(v)
}

func (nbs NestedBlocks) Values() map[string]cty.Value {
	v := map[string]cty.Value{}
	for k, blocks := range nbs {
		v[k] = ListOfObject(blocks)
	}
	return v
}

func dynamicNestedBlock(rb *hclsyntax.Block, wb *hclwrite.Block) *NestedBlock {
	nb := &NestedBlock{
		Type:           rb.Labels[0],
		selfWriteBlock: wb,
		Block:          rb.Body.Blocks[0],
		WriteBlock:     wb.Body().Blocks()[0],
		ForEach:        NewAttribute("for_each", rb.Body.Attributes["for_each"], wb.Body().GetAttribute("for_each")),
		Attributes:     attributes(rb.Body.Blocks[0].Body, wb.Body().Blocks()[0].Body()),
		NestedBlocks:   nestedBlocks(rb.Body.Blocks[0].Body, wb.Body().Blocks()[0].Body()),
	}
	if iteratorAttr, ok := rb.Body.Attributes["iterator"]; ok {
		nb.Iterator = NewAttribute("iterator", iteratorAttr, wb.Body().GetAttribute("iterator"))
	}
	return nb
}

func staticNestedBlock(rb *hclsyntax.Block, wb *hclwrite.Block) *NestedBlock {
	return &NestedBlock{
		Type:           rb.Type,
		Block:          rb,
		selfWriteBlock: wb,
		WriteBlock:     wb,
		Attributes:     attributes(rb.Body, wb.Body()),
		NestedBlocks:   nestedBlocks(rb.Body, wb.Body()),
	}
}
