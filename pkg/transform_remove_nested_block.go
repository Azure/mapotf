package pkg

import (
	"fmt"
	"github.com/Azure/golden"
	"strings"
)

var _ Transform = &RemoveNestedBlockTransform{}
var _ mptfBlock = &RemoveNestedBlockTransform{}

type RemoveNestedBlockTransform struct {
	*golden.BaseBlock
	*BaseTransform
	TargetBlockAddress string   `hcl:"target_block_address"`
	Paths              []string `hcl:"paths"`
}

func (r *RemoveNestedBlockTransform) isReservedField(name string) bool {
	reserved := map[string]struct{}{
		"target_block_address": {},
		"for_each":             {},
	}
	_, ok := reserved[name]
	return ok
}

func (r *RemoveNestedBlockTransform) Type() string {
	return "remove_nested_block"
}

func (r *RemoveNestedBlockTransform) Apply() error {
	cfg := r.BaseBlock.Config().(*MetaProgrammingTFConfig)
	b := cfg.RootBlock(r.TargetBlockAddress)
	if b == nil {
		return fmt.Errorf("cannot find block: %s", r.TargetBlockAddress)
	}
	for _, path := range r.Paths {
		path = strings.TrimSpace(path)
		b.RemoveNestedBlock(path)
	}
	return nil
}
