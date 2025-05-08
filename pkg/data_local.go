package pkg

import (
	"github.com/Azure/golden"
	"github.com/Azure/mapotf/pkg/terraform"
	"github.com/ahmetb/go-linq/v3"
	"github.com/zclconf/go-cty/cty"
	ctyjson "github.com/zclconf/go-cty/cty/json"
)

var _ Data = &DataLocal{}

type DataLocal struct {
	*BaseData
	*golden.BaseBlock

	ExpectedNameName string    `hcl:"name,optional"`
	Result           cty.Value `attribute:"result"`
}

func (dl *DataLocal) Type() string {
	return "local"
}

func (dl *DataLocal) ExecuteDuringPlan() error {
	src := dl.BaseBlock.Config().(*MetaProgrammingTFConfig).LocalBlocks()
	var matched []*terraform.RootBlock
	ds := linq.From(src)
	if dl.ExpectedNameName != "" {
		ds = ds.Where(func(i interface{}) bool {
			return i.(*terraform.RootBlock).Labels[0] == dl.ExpectedNameName
		})
	}
	ds.ToSlice(&matched)
	localBlocks := make(map[string]cty.Value)
	for _, block := range matched {
		for name, attr := range block.Attributes {
			localBlocks[name] = cty.StringVal(attr.String())
		}
	}
	dl.Result = cty.ObjectVal(localBlocks)
	return nil
}

func (dl *DataLocal) String() string {
	d := cty.ObjectVal(map[string]cty.Value{
		"name":   cty.StringVal(dl.ExpectedNameName),
		"result": dl.Result,
	})
	r, err := ctyjson.Marshal(d, d.Type())
	if err != nil {
		panic(err.Error())
	}
	return string(r)
}
