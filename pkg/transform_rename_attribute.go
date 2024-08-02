package pkg

import (
	"github.com/Azure/golden"
	"github.com/Azure/mapotf/pkg/terraform"
	"strings"
)

var _ Transform = &RenameAttributeTransform{}

//const RenamePattern = `%s\.(\s*\r?\n\s*)?\w+(\[\s*[^]]+\s*\])?\.(\s*\r?\n\s*)?%s`

type Rename struct {
	ResourceType  string   `hcl:"resource_type"`
	AttributePath []string `hcl:"attribute_path" validator:"required"`
	NewName       string   `hcl:"new_name" validator:"required"`
}

type RenameAttributeTransform struct {
	*golden.BaseBlock
	*BaseTransform
	Renames []Rename `hcl:"rename,block"`
}

func (r *RenameAttributeTransform) Type() string {
	return "rename_attribute"
}

func (r *RenameAttributeTransform) Apply() error {
	cfg := r.BaseBlock.Config().(*MetaProgrammingTFConfig)
	for _, rename := range r.Renames {
		r.applyRename(rename, cfg)
	}
	return nil
}

func (r *RenameAttributeTransform) applyRename(rename Rename, cfg *MetaProgrammingTFConfig) {
	resourceType := rename.ResourceType
	blocks := cfg.resourceBlocks
	if strings.HasPrefix(resourceType, "data.") {
		resourceType = strings.TrimPrefix(resourceType, "data.")
		blocks = cfg.dataBlocks
	}
	var matchedBlocks []*terraform.RootBlock
	for _, b := range blocks {
		if b.Labels[0] == resourceType {
			matchedBlocks = append(matchedBlocks, b)
		}
	}
	for _, b := range matchedBlocks {
		if len(rename.AttributePath) == 1 {
			old := rename.AttributePath[0]
			attr, ok := b.WriteBlock.Body().Attributes()[old]
			if !ok {
				continue
			}
			b.WriteBlock.Body().SetAttributeRaw(rename.NewName, attr.Expr().BuildTokens(nil))
			b.WriteBlock.Body().RemoveAttribute(old)
			continue
		}
		nbName := rename.AttributePath[0]
		nestedBlocks, ok := b.NestedBlocks[nbName]
		if !ok {
			continue
		}
		r.RenameAttributeInNestedBlock(nestedBlocks, rename.AttributePath[1:], rename.NewName)
	}
}

func (r *RenameAttributeTransform) RenameAttributeInNestedBlock(blocks []*terraform.NestedBlock, attributePath []string, name string) {
	if len(attributePath) == 1 {
		old := attributePath[0]
		for _, b := range blocks {
			attr, ok := b.WriteBlock.Body().Attributes()[old]
			if !ok {
				continue
			}
			b.WriteBlock.Body().SetAttributeRaw(name, attr.Expr().BuildTokens(nil))
			b.WriteBlock.Body().RemoveAttribute(old)
		}
		return
	}
	nbName := attributePath[0]
	for _, b := range blocks {
		nestedBlocks, ok := b.NestedBlocks[nbName]
		if !ok {
			continue
		}
		r.RenameAttributeInNestedBlock(nestedBlocks, attributePath[1:], name)
	}
}
