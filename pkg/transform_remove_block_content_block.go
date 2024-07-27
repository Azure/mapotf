package pkg

import (
	"fmt"
	"github.com/Azure/golden"
	"strings"
)

var _ Transform = &RemoveBlockContentBlockTransform{}
var _ mptfBlock = &RemoveBlockContentBlockTransform{}

type RemoveBlockContentBlockTransform struct {
	*golden.BaseBlock
	*BaseTransform
	TargetBlockAddress string   `hcl:"target_block_address"`
	Paths              []string `hcl:"paths"`
}

func (r *RemoveBlockContentBlockTransform) isReservedField(name string) bool {
	reserved := map[string]struct{}{
		"target_block_address": {},
		"for_each":             {},
	}
	_, ok := reserved[name]
	return ok
}

func (r *RemoveBlockContentBlockTransform) Type() string {
	return "remove_block_content"
}

func (r *RemoveBlockContentBlockTransform) Apply() error {
	cfg := r.BaseBlock.Config().(*MetaProgrammingTFConfig)
	b := cfg.RootBlock(r.TargetBlockAddress)
	if b == nil {
		return fmt.Errorf("cannot find block: %s", r.TargetBlockAddress)
	}
	for _, path := range r.Paths {
		path = strings.TrimSpace(path)
		b.RemoveContent(path)
	}
	return nil
}
