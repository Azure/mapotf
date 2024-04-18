package pkg

import (
	"github.com/Azure/golden"
	"github.com/ahmetb/go-linq/v3"
	"github.com/lonegunmanb/mptf/pkg/terraform"
	"github.com/zclconf/go-cty/cty"
	ctyjson "github.com/zclconf/go-cty/cty/json"
)

var _ Data = &ResourceData{}

type ResourceData struct {
	*BaseData
	*golden.BaseBlock

	ResourceType string    `hcl:"resource_type" json:"resource_type"`
	UseCount     bool      `hcl:"use_count" json:"use_count"`
	UseForEach   bool      `hcl:"use_for_each" json:"use_for_each"`
	Result       cty.Value `attribute:"result"`
}

func (rd *ResourceData) Type() string {
	return "resource"
}

func (rd *ResourceData) ExecuteDuringPlan() error {
	src := rd.BaseBlock.Config().(*MetaProgrammingTFConfig).ResourceBlocks
	res := linq.From(src)
	if rd.ResourceType != "" {
		res = res.Where(func(i interface{}) bool {
			return i.(*terraform.Block).Labels[0] == rd.ResourceType
		})
	}
	if rd.UseForEach {
		res = res.Where(func(i interface{}) bool {
			return i.(*terraform.Block).ForEach != nil
		})
	}
	if rd.UseCount {
		res = res.Where(func(i interface{}) bool {
			return i.(*terraform.Block).Count != nil
		})
	}
	resourceBlocks := make(map[string]map[string]cty.Value)
	res.ForEach(func(i interface{}) {
		b := i.(*terraform.Block)
		t := b.Labels[0]
		address := b.Address
		sm, ok := resourceBlocks[t]
		if !ok {
			sm = make(map[string]cty.Value)
			resourceBlocks[t] = sm
		}
		sm[address] = b.EvalContext()
	})
	rd.Result = golden.ToCtyValue(resourceBlocks)
	return nil
}

func (rd *ResourceData) String() string {
	d := cty.ObjectVal(map[string]cty.Value{
		"resource_type": cty.StringVal(rd.ResourceType),
		"use_count":     cty.BoolVal(rd.UseCount),
		"use_for_each":  cty.BoolVal(rd.UseForEach),
		"result":        rd.Result,
	})
	r, err := ctyjson.Marshal(d, d.Type())
	if err != nil {
		panic(err.Error())
	}
	return string(r)
}
