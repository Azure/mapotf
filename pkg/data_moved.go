package pkg

import (
	"github.com/Azure/golden"
	"github.com/zclconf/go-cty/cty"
	ctyjson "github.com/zclconf/go-cty/cty/json"
)

var _ Data = &DataMoved{}

// DataMoved exposes every `moved` block in the target Terraform module as a
// map keyed by a synthetic, declaration-order index ("0", "1", ...). Moved
// blocks have no native label, so the index is the only stable way to address
// them individually.
//
// Each value is the block's EvalContext — `from` and `to` attribute strings
// plus an `mptf` metadata sub-object.
type DataMoved struct {
	*BaseData
	*golden.BaseBlock

	Result cty.Value `attribute:"result"`
}

func (d *DataMoved) Type() string {
	return "moved"
}

func (d *DataMoved) ExecuteDuringPlan() error {
	src := d.BaseBlock.Config().(*MetaProgrammingTFConfig).MovedBlocks()
	movedBlocks := make(map[string]cty.Value, len(src))
	for _, block := range src {
		if len(block.Labels) == 0 {
			continue
		}
		movedBlocks[block.Labels[0]] = block.EvalContext()
	}
	d.Result = cty.ObjectVal(movedBlocks)
	return nil
}

func (d *DataMoved) String() string {
	data := cty.ObjectVal(map[string]cty.Value{
		"result": d.Result,
	})
	r, err := ctyjson.Marshal(data, data.Type())
	if err != nil {
		panic(err.Error())
	}
	return string(r)
}
