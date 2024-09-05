package pkg

import (
	"encoding/json"
	"fmt"
	"github.com/Azure/golden"
	"github.com/Azure/mapotf/pkg/terraform"
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclwrite"
)

var _ Transform = &ConcatBlockBodyTransform{}

type ConcatBlockBodyTransform struct {
	*golden.BaseBlock
	*BaseTransform
	TargetBlockAddress string `hcl:"target_block_address" validate:"required"`
	BlockBody          string `hcl:"block_body" validate:"required"`
}

func (u *ConcatBlockBodyTransform) Type() string {
	return "concat_block_body"
}

func (u *ConcatBlockBodyTransform) Apply() error {
	c := u.BaseBlock.Config().(*MetaProgrammingTFConfig)
	b := c.RootBlock(u.TargetBlockAddress)
	if b == nil {
		return fmt.Errorf("cannot find block: %s", u.TargetBlockAddress)
	}
	cfg, diag := hclwrite.ParseConfig([]byte("concat {\n"+u.BlockBody+"\n}"), "concat.hcl", hcl.InitialPos)
	if diag.HasErrors() {
		return diag
	}
	u.PatchWriteBlock(b, cfg.Body().Blocks()[0].Body())
	return nil
}

func (u *ConcatBlockBodyTransform) PatchWriteBlock(dest terraform.Block, patch *hclwrite.Body) {
	// we cannot patch one-line block
	if dest.Range().Start.Line == dest.Range().End.Line {
		dest.WriteBody().AppendNewline()
	}
	for name, attr := range patch.Attributes() {
		dest.SetAttributeRaw(name, attr.Expr().BuildTokens(nil))
	}
	for _, nb := range patch.Blocks() {
		dest.AppendBlock(nb)
	}
}

func (u *ConcatBlockBodyTransform) String() string {
	content := make(map[string]any)
	content["id"] = u.Id()
	content["target_block_address"] = u.TargetBlockAddress
	content["concat"] = u.BlockBody
	str, err := json.Marshal(content)
	if err != nil {
		panic(err.Error())
	}
	return string(str)
}
