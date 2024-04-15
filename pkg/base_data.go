package pkg

import "github.com/Azure/golden"

type Data interface {
	golden.PlanBlock
	Data()
}

type BaseData struct {
	*MetaProgrammingTFConfig
}

func (bd *BaseData) BlockType() string {
	return "data"
}

func (bd *BaseData) Data() {}

func (bd *BaseData) CanExecutePrePlan() bool {
	return false
}
