package pkg

import (
	"github.com/Azure/golden"
	"github.com/Azure/mapotf/pkg/terraform"
	"github.com/ahmetb/go-linq/v3"
	"github.com/zclconf/go-cty/cty"
	ctyjson "github.com/zclconf/go-cty/cty/json"
)

var _ Data = &DataModule{}

// DataModule exposes every `module` block in the target Terraform module as a
// map keyed by module name. Each value is the block's full EvalContext —
// stringified attributes plus an `mptf` metadata sub-object.
//
// Optional `name` filter narrows the result to a single module block.
type DataModule struct {
	*BaseData
	*golden.BaseBlock

	ExpectedModuleName string    `hcl:"name,optional"`
	Result             cty.Value `attribute:"result"`
}

func (d *DataModule) Type() string {
	return "module"
}

func (d *DataModule) ExecuteDuringPlan() error {
	src := d.BaseBlock.Config().(*MetaProgrammingTFConfig).ModuleBlocks()
	var matched []*terraform.RootBlock
	ds := linq.From(src)
	if d.ExpectedModuleName != "" {
		ds = ds.Where(func(i interface{}) bool {
			return i.(*terraform.RootBlock).Labels[0] == d.ExpectedModuleName
		})
	}
	ds.ToSlice(&matched)

	moduleBlocks := make(map[string]cty.Value)
	for _, block := range matched {
		moduleBlocks[block.Labels[0]] = block.EvalContext()
	}
	d.Result = cty.ObjectVal(moduleBlocks)
	return nil
}

func (d *DataModule) String() string {
	data := cty.ObjectVal(map[string]cty.Value{
		"name":   cty.StringVal(d.ExpectedModuleName),
		"result": d.Result,
	})
	r, err := ctyjson.Marshal(data, data.Type())
	if err != nil {
		panic(err.Error())
	}
	return string(r)
}
