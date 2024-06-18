package pkg

import (
	"github.com/Azure/golden"
)

type Transform interface {
	golden.ApplyBlock
	Transform()
}

type BaseTransform struct{}

func (bt *BaseTransform) BlockType() string       { return "transform" }
func (bt *BaseTransform) AddressLength() int      { return 3 }
func (bt *BaseTransform) CanExecutePrePlan() bool { return false }
func (bt *BaseTransform) Transform()              {}
