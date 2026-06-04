package pkg

import (
	"fmt"

	"github.com/Azure/golden"
	"github.com/hashicorp/hcl/v2/hclwrite"
)

var _ Transform = &SortBlocksInFileTransform{}

// SortBlocksInFileTransform ensures every block listed in `desired_order`
// lives in `file_name`, in the listed order, after any blocks that are already
// in the file but absent from the list.
//
// Semantics:
//   - For each address in `desired_order`, the block is removed from whichever
//     file currently holds it and appended to `file_name` in the listed order.
//   - Blocks already in `file_name` that are NOT in `desired_order` are left
//     untouched at the top of the file.
//   - An address that does not resolve to a known block is a hard error
//     (silent skip would mask drift, since `desired_order` is usually
//     computed from a data source).
//   - An empty `desired_order` is a no-op. This makes it safe to drive the
//     list from a data source (e.g. `[for v in data.variable.all.result :
//     "variable.${v.name}"]`) in module contexts where the data source
//     resolves to an empty collection — for example `examples/*` directories
//     that have no `variables.tf`.
type SortBlocksInFileTransform struct {
	*golden.BaseBlock
	*BaseTransform
	FileName     string   `hcl:"file_name" validate:"endswith=.tf"`
	DesiredOrder []string `hcl:"desired_order" validate:"unique,dive,min=1"`
}

func (s *SortBlocksInFileTransform) Type() string {
	return "sort_blocks_in_file"
}

func (s *SortBlocksInFileTransform) Apply() error {
	if len(s.DesiredOrder) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(s.DesiredOrder))
	for _, addr := range s.DesiredOrder {
		if _, ok := seen[addr]; ok {
			return fmt.Errorf("sort_blocks_in_file: duplicate block address %q in desired_order", addr)
		}
		seen[addr] = struct{}{}
	}

	cfg := s.Config().(*MetaProgrammingTFConfig)

	writeBlocks := make([]*hclwrite.Block, 0, len(s.DesiredOrder))
	for _, addr := range s.DesiredOrder {
		b := cfg.RootBlock(addr)
		if b == nil {
			return fmt.Errorf("sort_blocks_in_file: cannot find block %q referenced in desired_order", addr)
		}
		writeBlocks = append(writeBlocks, b.WriteBlock)
	}

	// Two passes: first remove everything from its current file, then re-add in order.
	// Doing the removes first prevents collisions when the same file holds source and target.
	for _, wb := range writeBlocks {
		cfg.module.RemoveBlock(wb)
	}
	for _, wb := range writeBlocks {
		cfg.AddBlock(s.FileName, wb)
	}
	return nil
}
