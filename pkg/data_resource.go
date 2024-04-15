package pkg

import "github.com/Azure/golden"

var _ Data = &ResourceData{}

type ResourceData struct {
	*BaseData
	*golden.BaseBlock

	ResourceType string
	Name         string

	Content map[string]any
}

func (rd *ResourceData) Type() string {
	return "resource"
}

func (rd *ResourceData) AddressLength() int { return 4 }

func (rd *ResourceData) ExecuteDuringPlan() error {
	labels := rd.BaseBlock.HclBlock().Labels
	rd.ResourceType = labels[0]
	rd.Name = labels[1]
	rd.EvalContext()
}
