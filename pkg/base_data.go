package pkg

import "github.com/Azure/golden"

type Data interface {
	golden.PlanBlock
	Data()
}

type BaseData struct{}

func (bd *BaseData) BlockType() string {
	return "data"
}

func (bd *BaseData) Data() {}

func (bd *BaseData) AddressLength() int { return 3 }

func (bd *BaseData) CanExecutePrePlan() bool {
	return false
}
