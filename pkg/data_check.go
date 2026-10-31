package pkg

import (
	"fmt"

	"github.com/Azure/golden"
	"github.com/zclconf/go-cty/cty"
	ctyjson "github.com/zclconf/go-cty/cty/json"
)

var _ Data = &DataCheck{}

type DataCheck struct {
	*BaseData
	*golden.BaseBlock

	Result cty.Value `attribute:"result"`
}

func (d *DataCheck) Type() string {
	return "check"
}

func (d *DataCheck) ExecuteDuringPlan() error {
	src := d.BaseBlock.Config().(*MetaProgrammingTFConfig).CheckBlocks()
	checks := make(map[string]cty.Value, len(src))
	for _, block := range src {
		if len(block.Labels) != 1 {
			return fmt.Errorf("check block at %s must have exactly one label", block.Range().String())
		}
		references, err := block.CheckReferences()
		if err != nil {
			return err
		}
		value := block.EvalContext().AsValueMap()
		metadata := value["mptf"].AsValueMap()
		metadata["references"] = references
		value["mptf"] = cty.ObjectVal(metadata)
		checks[block.Labels[0]] = cty.ObjectVal(value)
	}
	d.Result = cty.ObjectVal(checks)
	return nil
}

func (d *DataCheck) String() string {
	data := cty.ObjectVal(map[string]cty.Value{
		"result": d.Result,
	})
	r, err := ctyjson.Marshal(data, data.Type())
	if err != nil {
		panic(err.Error())
	}
	return string(r)
}
