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
	golden.RegisterBlock(new(RemoveBlockContentBlockTransform))
	golden.RegisterBlock(new(RenameAttributeOrNestedBlockTransform))
	golden.RegisterBlock(new(RegexReplaceExpressionTransform))
	golden.RegisterBlock(new(AppendBlockBodyTransform))
	golden.RegisterBlock(new(EnsureLocalTransform))
}

func registerData() {
	golden.RegisterBlock(new(ResourceData))
	golden.RegisterBlock(new(ProviderSchemaData))
	golden.RegisterBlock(new(TerraformData))
	golden.RegisterBlock(new(DataSourceData))
	golden.RegisterBlock(new(DataVariable))
	golden.RegisterBlock(new(DataOutput))
	golden.RegisterBlock(new(DataLocal))
}
