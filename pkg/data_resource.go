package pkg

import (
	"github.com/Azure/golden"
	"github.com/Azure/mapotf/pkg/terraform"
	"github.com/ahmetb/go-linq/v3"
	"github.com/zclconf/go-cty/cty"
	ctyjson "github.com/zclconf/go-cty/cty/json"
)

var _ Data = &ResourceData{}

type ResourceData struct {
	*BaseData
	*golden.BaseBlock

	ResourceType string    `hcl:"resource_type,optional"`
	UseCount     bool      `hcl:"use_count,optional" default:"false"`
	UseForEach   bool      `hcl:"use_for_each,optional" default:"false"`
	Result       cty.Value `attribute:"result"`
}

func (rd *ResourceData) Type() string {
	return "resource"
}

func (rd *ResourceData) ExecuteDuringPlan() error {
	src := rd.BaseBlock.Config().(*MetaProgrammingTFConfig).ResourceBlocks()
	var matched []*terraform.RootBlock
	res := linq.From(src)
	if rd.ResourceType != "" {
		res = res.Where(func(i interface{}) bool {
			return i.(*terraform.RootBlock).Labels[0] == rd.ResourceType
		})
	}
	if rd.UseForEach {
		res = res.Where(func(i interface{}) bool {
			return i.(*terraform.RootBlock).ForEach != nil
		})
	}
	if rd.UseCount {
		res = res.Where(func(i interface{}) bool {
			return i.(*terraform.RootBlock).Count != nil
		})
	}
	res.ToSlice(&matched)
	resourceBlocks := make(map[string]map[string]cty.Value)
	for _, b := range matched {
		resourceType := b.Labels[0]
		m, ok := resourceBlocks[resourceType]
		if !ok {
			m = make(map[string]cty.Value)
			resourceBlocks[resourceType] = m
		}
		m[b.Labels[1]] = b.EvalContext()
	}
	obj := make(map[string]cty.Value)
	for k, m := range resourceBlocks {
		obj[k] = cty.ObjectVal(m)
	}
	rd.Result = cty.ObjectVal(obj)
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
