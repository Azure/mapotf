package pkg

import (
	"github.com/Azure/golden"
	"github.com/Azure/mapotf/pkg/terraform"
	"github.com/ahmetb/go-linq/v3"
	"github.com/zclconf/go-cty/cty"
	ctyjson "github.com/zclconf/go-cty/cty/json"
)

var _ Data = &DataVariable{}

type DataVariable struct {
	*BaseData
	*golden.BaseBlock

	ExpectedNameName string    `hcl:"name,optional"`
	ExpectedType     string    `hcl:"type,optional"`
	Result           cty.Value `attribute:"result"`
}

func (dd *DataVariable) Type() string {
	return "variable"
}

func (dd *DataVariable) ExecuteDuringPlan() error {
	src := dd.BaseBlock.Config().(*MetaProgrammingTFConfig).VariableBlocks()
	var matched []*terraform.RootBlock
	ds := linq.From(src)
	if dd.ExpectedNameName != "" {
		ds = ds.Where(func(i interface{}) bool {
			return i.(*terraform.RootBlock).Labels[0] == dd.ExpectedNameName
		})
	}
	if dd.ExpectedType != "" {

		ds = ds.Where(func(i interface{}) bool {
			typeAttr, ok := i.(*terraform.RootBlock).Attributes["type"]
			if !ok {
				return false
			}
			typeVal := typeAttr.String()
			return typeVal == dd.ExpectedType
		})
	}
	ds.ToSlice(&matched)
	variableBlocks := make(map[string]cty.Value)
	for _, block := range matched {
		variableName := block.Labels[0]
		variableBlocks[variableName] = block.EvalContext()
	}
	dd.Result = cty.ObjectVal(variableBlocks)
	return nil
}

func (dd *DataVariable) String() string {
	d := cty.ObjectVal(map[string]cty.Value{
		"name":   cty.StringVal(dd.ExpectedNameName),
		"type":   cty.StringVal(dd.ExpectedType),
		"result": dd.Result,
	})
	r, err := ctyjson.Marshal(d, d.Type())
	if err != nil {
		panic(err.Error())
	}
	return string(r)
}
