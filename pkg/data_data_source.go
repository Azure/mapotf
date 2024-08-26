package pkg

import (
	"github.com/Azure/golden"
	"github.com/Azure/mapotf/pkg/terraform"
	"github.com/ahmetb/go-linq/v3"
	"github.com/zclconf/go-cty/cty"
	ctyjson "github.com/zclconf/go-cty/cty/json"
)

var _ Data = &DataSourceData{}

type DataSourceData struct {
	*BaseData
	*golden.BaseBlock

	DataSourceType string    `hcl:"data_source_type,optional"`
	UseCount       bool      `hcl:"use_count,optional" default:"false"`
	UseForEach     bool      `hcl:"use_for_each,optional" default:"false"`
	Result         cty.Value `attribute:"result"`
}

func (dd *DataSourceData) Type() string {
	return "data"
}

func (dd *DataSourceData) ExecuteDuringPlan() error {
	src := dd.BaseBlock.Config().(*MetaProgrammingTFConfig).DataBlocks()
	var matched []*terraform.RootBlock
	ds := linq.From(src)
	if dd.DataSourceType != "" {
		ds = ds.Where(func(i interface{}) bool {
			return i.(*terraform.RootBlock).Labels[0] == dd.DataSourceType
		})
	}
	if dd.UseForEach {
		ds = ds.Where(func(i interface{}) bool {
			return i.(*terraform.RootBlock).ForEach != nil
		})
	}
	if dd.UseCount {
		ds = ds.Where(func(i interface{}) bool {
			return i.(*terraform.RootBlock).Count != nil
		})
	}
	ds.ToSlice(&matched)
	dataBlocks := make(map[string]map[string]cty.Value)
	for _, b := range matched {
		dataType := b.Labels[0]
		m, ok := dataBlocks[dataType]
		if !ok {
			m = make(map[string]cty.Value)
			dataBlocks[dataType] = m
		}
		m[b.Labels[1]] = b.EvalContext()
	}
	obj := make(map[string]cty.Value)
	for k, m := range dataBlocks {
		obj[k] = cty.ObjectVal(m)
	}
	dd.Result = cty.ObjectVal(obj)
	return nil
}

func (dd *DataSourceData) String() string {
	d := cty.ObjectVal(map[string]cty.Value{
		"data_source_type": cty.StringVal(dd.DataSourceType),
		"use_count":        cty.BoolVal(dd.UseCount),
		"use_for_each":     cty.BoolVal(dd.UseForEach),
		"result":           dd.Result,
	})
	r, err := ctyjson.Marshal(d, d.Type())
	if err != nil {
		panic(err.Error())
	}
	return string(r)
}
