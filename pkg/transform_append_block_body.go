package pkg

import (
	"encoding/json"
	"fmt"

	"github.com/Azure/golden"
	"github.com/Azure/mapotf/pkg/terraform"
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclwrite"
)

var _ Transform = &AppendBlockBodyTransform{}

type AppendBlockBodyTransform struct {
	*golden.BaseBlock
	*BaseTransform
	TargetBlockAddress string `hcl:"target_block_address" validate:"required"`
	BlockBody          string `hcl:"block_body" validate:"required"`
}

func (u *AppendBlockBodyTransform) Type() string {
	return "append_block_body"
}

func (u *AppendBlockBodyTransform) Apply() error {
	c := u.BaseBlock.Config().(*MetaProgrammingTFConfig)
	b := c.RootBlock(u.TargetBlockAddress)
	if b == nil {
		return fmt.Errorf("cannot find block: %s", u.TargetBlockAddress)
	}
	cfg, diag := hclwrite.ParseConfig([]byte("append {\n"+u.BlockBody+"\n}"), "append.hcl", hcl.InitialPos)
	if diag.HasErrors() {
		return fmt.Errorf("failed to parse block body in %s, body is %s: %s", u.Address(), u.BlockBody, diag.Error())
	}
	u.PatchWriteBlock(b, cfg.Body().Blocks()[0].Body())
	return nil
}

func (u *AppendBlockBodyTransform) PatchWriteBlock(dest terraform.Block, patch *hclwrite.Body) {
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

func (u *AppendBlockBodyTransform) String() string {
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
