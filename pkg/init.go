package pkg

import "github.com/Azure/golden"

func init() {
	golden.RegisterBaseBlock(func() golden.BlockType {
		return new(BaseData)
	})
	golden.RegisterBaseBlock(func() golden.BlockType {
		return new(BaseTransform)
	})
	registerData()
	registerTransform()
}

func registerTransform() {
	golden.RegisterBlock(new(UpdateInPlaceTransform))
	golden.RegisterBlock(new(NewBlockTransform))
	golden.RegisterBlock(new(RemoveNestedBlockTransform))
}

func registerData() {
	golden.RegisterBlock(new(ResourceData))
}
