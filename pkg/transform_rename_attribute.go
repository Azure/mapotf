package pkg

import (
	"github.com/Azure/golden"
	"github.com/Azure/mapotf/pkg/terraform"
	"strings"
)

var _ Transform = &RenameAttributeTransform{}

type RenameAttributeTransform struct {
	*golden.BaseBlock
	*BaseTransform
	ResourceType  string   `hcl:"resource_type"`
	AttributePath []string `hcl:"attribute_path" validator:"required"`
	NewName       string   `hcl:"new_name" validator:"required"`
}

func (r *RenameAttributeTransform) Type() string {
	return "rename_attribute"
}

func (r *RenameAttributeTransform) Apply() error {
	cfg := r.BaseBlock.Config().(*MetaProgrammingTFConfig)
	resourceType := r.ResourceType
	blocks := cfg.resourceBlocks
	if strings.HasPrefix(resourceType, "data.") {
		resourceType = strings.TrimPrefix(resourceType, "data.")
		blocks = cfg.dataBlocks
	}
	var matchedBlocks []*terraform.RootBlock
	for _, b := range blocks {
		if b.Type == resourceType {
			matchedBlocks = append(matchedBlocks, b)
		}
	}
	//azurerm_resource_group\.\w+(\[\S+\])?\.location

	return nil
}
