package pkg

import (
	"github.com/Azure/golden"
	"github.com/Azure/mapotf/pkg/terraform"
	"github.com/ahmetb/go-linq/v3"
	"github.com/zclconf/go-cty/cty"
	ctyjson "github.com/zclconf/go-cty/cty/json"
)

var _ Data = &DataOutput{}

type DataOutput struct {
	*BaseData
	*golden.BaseBlock

	ExpectedOutputName string    `attribute:"name"`
	Result             cty.Value `attribute:"result"`
}

func (d *DataOutput) Type() string {
	return "output"
}

func (d *DataOutput) ExecuteDuringPlan() error {
	src := d.BaseBlock.Config().(*MetaProgrammingTFConfig).OutputBlocks()
	var matched []*terraform.RootBlock
	ds := linq.From(src)

	if d.ExpectedOutputName != "" {
		ds = ds.Where(func(i interface{}) bool {
			return i.(*terraform.RootBlock).Labels[0] == d.ExpectedOutputName
		})
	}

	ds.ToSlice(&matched)

	outputBlocks := make(map[string]cty.Value)
	for _, block := range matched {
		outputName := block.Labels[0]
		outputBlocks[outputName] = block.EvalContext()
	}

	d.Result = cty.ObjectVal(outputBlocks)
	return nil
}

func (d *DataOutput) String() string {
	data := map[string]cty.Value{
		"result": d.Result,
	}

	r, err := ctyjson.Marshal(cty.ObjectVal(data), cty.Object(map[string]cty.Type{
		"result": d.Result.Type(),
	}))
	if err != nil {
		panic(err.Error())
	}
	return string(r)
}
