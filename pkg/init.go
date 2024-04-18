package pkg

import "github.com/Azure/golden"

func init() {
	golden.RegisterBaseBlock(func() golden.BlockType {
		return new(BaseData)
	})
	registerData()
}

func registerData() {
	golden.RegisterBlock(new(ResourceData))
}
