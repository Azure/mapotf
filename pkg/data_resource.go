package pkg

import (
	"github.com/Azure/golden"
	"github.com/zclconf/go-cty/cty"
)

var _ Data = &ResourceData{}

type ResourceData struct {
	*BaseData
	*golden.BaseBlock
	c *MetaProgrammingTFConfig

	ResourceType string `hcl:"resource_type" json:"resource_type"`
	UseCount     bool   `hcl:"use_count" json:"use_count"`
	UseForEach   bool   `hcl:"use_for_each" json:"use_for_each"`
	Result       cty.Value
}

func (rd *ResourceData) Type() string {
	return "resource"
}

func (rd *ResourceData) AddressLength() int { return 3 }

func (rd *ResourceData) ExecuteDuringPlan() error {

	//labels := rd.BaseBlock.HclBlock().Labels
	//rd.ResourceType = labels[0]
	////rd.Name = labels[1]
	//rd.EvalContext()
	panic("implement me")
}
