package pkg

import (
	"github.com/Azure/golden"
	"github.com/Azure/mapotf/pkg/terraform"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/hashicorp/hcl/v2/hclwrite"
	"strings"
)

var _ Transform = &RenameAttributeOrNestedBlockTransform{}

type Rename struct {
	ResourceType  string   `hcl:"resource_type"`
	AttributePath []string `hcl:"attribute_path" validator:"required"`
	NewName       string   `hcl:"new_name" validator:"required"`
}

type RenameAttributeOrNestedBlockTransform struct {
	*golden.BaseBlock
	*BaseTransform
	Renames []Rename `hcl:"rename,block"`
}

func (r *RenameAttributeOrNestedBlockTransform) Type() string {
	return "rename_block_element"
}

func (r *RenameAttributeOrNestedBlockTransform) Apply() error {
	cfg := r.BaseBlock.Config().(*MetaProgrammingTFConfig)
	for _, rename := range r.Renames {
		r.applyRename(rename, cfg)
	}
	return nil
}

func (r *RenameAttributeOrNestedBlockTransform) applyRename(rename Rename, cfg *MetaProgrammingTFConfig) {
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
	r.rename(castBlockSlice(matchedBlocks), rename.AttributePath, rename.NewName)
}

func (r *RenameAttributeOrNestedBlockTransform) rename(blocks []terraform.Block, attributePath []string, newName string) {
	if len(attributePath) == 1 {
		old := attributePath[0]
		for _, b := range blocks {
			body := b.WriteBody()
			attr, ok := body.Attributes()[old]
			if ok {
				body.SetAttributeRaw(newName, attr.Expr().BuildTokens(nil))
				body.RemoveAttribute(old)
				continue
			}
			for _, nb := range body.Blocks() {
				if r.nestedBlockType(nb) != old {
					continue
				}
				r.setNestedBlockType(nb, old, newName)
			}
		}
		return
	}
	nbName := attributePath[0]
	for _, b := range blocks {
		nestedBlocks, ok := b.GetNestedBlocks()[nbName]
		if !ok {
			continue
		}
		r.rename(castBlockSlice(nestedBlocks), attributePath[1:], newName)
	}
}

func (r *RenameAttributeOrNestedBlockTransform) nestedBlockType(nb *hclwrite.Block) string {
	if nb.Type() == "dynamic" {
		return nb.Labels()[0]
	}
	return nb.Type()
}

func (r *RenameAttributeOrNestedBlockTransform) setNestedBlockType(nb *hclwrite.Block, oldName, newName string) {
	if nb.Type() == "dynamic" {
		nb.SetLabels([]string{newName})
		nb.Body().SetAttributeRaw("iterator", hclwrite.Tokens{&hclwrite.Token{
			Type:  hclsyntax.TokenIdent,
			Bytes: []byte(oldName),
		}})
	} else {
		nb.SetType(newName)
	}
}

func castBlockSlice[T terraform.Block](s []T) []terraform.Block {
	ret := make([]terraform.Block, len(s))
	for i, v := range s {
		ret[i] = v
	}
	return ret
}
