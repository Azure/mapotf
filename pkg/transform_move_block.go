package pkg

import (
	"fmt"
	"github.com/Azure/golden"
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
