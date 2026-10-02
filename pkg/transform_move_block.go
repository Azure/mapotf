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
	FileName           string `hcl:"file_name"`
}

func (m *MoveBlockTransform) Type() string {
	return "move_block"
}

func (m *MoveBlockTransform) Apply() error {
	cfg := m.Config().(*MetaProgrammingTFConfig)
	if err := cfg.validateTargetFile(m.FileName); err != nil {
		return err
	}
	block := cfg.RootBlock(m.TargetBlockAddress)
	if block == nil {
		return fmt.Errorf("cannot find block: %s", m.TargetBlockAddress)
	}

	// Get the write block from the found block
	writeBlock := block.WriteBlock

	if block.Range().Filename == m.FileName {
		return nil
	}
	// RemoveBlock must precede AddBlock: the new structured AppendBlock used
	// by AddBlock requires the block not already be owned by another body.
	// (Same pattern as sort_blocks_in_file.)
	cfg.module.RemoveBlock(writeBlock)
	return cfg.AddBlock(m.FileName, writeBlock)
}
