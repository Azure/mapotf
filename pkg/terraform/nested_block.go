package terraform

import (
	"github.com/ahmetb/go-linq/v3"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/zclconf/go-cty/cty"
	"github.com/zclconf/go-cty/cty/convert"
)

type NestedBlocks map[string][]*NestedBlock

type NestedBlock struct {
	Type string
	*hclsyntax.Block
	WriteBlock   *hclwrite.Block
	ForEach      *Attribute
	Attributes   map[string]*Attribute
	NestedBlocks NestedBlocks
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

func (nb *NestedBlock) EvalContext() cty.Value {
	v := map[string]cty.Value{}
	for n, a := range nb.Attributes {
		v[n] = cty.StringVal(a.String())
	}
	if nb.ForEach != nil {
		v["for_each"] = cty.StringVal(nb.ForEach.String())
	}
	for k, nbv := range nb.NestedBlocks.Values() {
		v[k] = nbv
	}

	return cty.ObjectVal(v)
}

func (nbs NestedBlocks) Values() map[string]cty.Value {
	v := map[string]cty.Value{}
	for k, blocks := range nbs {
		var values []cty.Value
		allTypes := make(map[string]cty.Type)
		for _, b := range blocks {
			value := b.EvalContext()
			values = append(values, value)
			attributeTypes := value.Type().AttributeTypes()
			for n, t := range attributeTypes {
				allTypes[n] = t
			}
		}
		var allFields []string
		linq.From(allTypes).Select(func(i interface{}) interface{} {
			return i.(linq.KeyValue).Key
		}).ToSlice(&allFields)
		finalType := cty.ObjectWithOptionalAttrs(allTypes, allFields)
		var convertedValues []cty.Value
		for _, v := range values {
			cv, err := convert.Convert(v, finalType)
			if err != nil {
				panic(err)
			}
			convertedValues = append(convertedValues, cv)
		}
		v[k] = cty.ListVal(convertedValues)
	}
	return v
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
