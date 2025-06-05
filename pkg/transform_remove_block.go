package pkg

import (
	"fmt"
	"github.com/Azure/golden"
)

var _ Transform = &RemoveBlockTransform{}

type RemoveBlockTransform struct {
	*golden.BaseBlock
	*BaseTransform
	TargetBlockAddress string `hcl:"target_block_address"`
}

func (r *RemoveBlockTransform) Type() string {
	return "remove_block"
}

func (r *RemoveBlockTransform) Apply() error {
	cfg := r.Config().(*MetaProgrammingTFConfig)
	b := cfg.RootBlock(r.TargetBlockAddress)
	if b == nil {
		return fmt.Errorf("cannot find block: %s", r.TargetBlockAddress)
	}
	cfg.module.RemoveBlock(b.WriteBlock)
	return nil
}
