package pkg

import (
	"fmt"
	"github.com/Azure/golden"
	"github.com/hashicorp/hcl/v2/hclwrite"
)

var _ Transform = &MoveBlockTransform{}

type MoveBlockTransform struct {
	*golden.BaseBlock
	*BaseTransform
	TargetBlockAddress string `hcl:"target_block_address"`
	FileName           string `hcl:"file_name" validate:"endswith=.tf"`
}

func (m *MoveBlockTransform) Type() string {
	return "move_block"
}

func (m *MoveBlockTransform) Apply() error {
	cfg := m.Config().(*MetaProgrammingTFConfig)
	block := cfg.RootBlock(m.TargetBlockAddress)
	if block == nil {
		return fmt.Errorf("cannot find block: %s", m.TargetBlockAddress)
	}

	// Get the write block from the found block
	writeBlock := block.WriteBlock

	if block.Range().Filename == m.FileName {
		return nil
	}
	cfg.AddBlock(m.FileName, writeBlock)
	cfg.module.RemoveBlock(writeBlock)
	return nil
}

// copyHclBlock creates a copy of an HCL block
func copyHclBlock(block *hclwrite.Block) *hclwrite.Block {
	// Create a new block with the same type and labels
	newBlock := hclwrite.NewBlock(block.Type(), block.Labels())

	// Copy all attributes
	for name, attr := range block.Body().Attributes() {
		newBlock.Body().SetAttributeRaw(name, attr.Expr().BuildTokens(nil))
	}

	// Copy all nested blocks
	for _, nestedBlock := range block.Body().Blocks() {
		newNestedBlock := copyHclBlock(nestedBlock)
		newBlock.Body().AppendBlock(newNestedBlock)
	}

	return newBlock
}
