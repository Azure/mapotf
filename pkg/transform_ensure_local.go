package pkg

import (
	"encoding/json"
	"fmt"
	"github.com/Azure/golden"
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclwrite"
)

var _ Transform = &EnsureLocalTransform{}
var _ golden.CustomDecode = &EnsureLocalTransform{}

type EnsureLocalTransform struct {
	*golden.BaseBlock
	*BaseTransform
	LocalName        string `hcl:"name" validate:"required"`
	FallbackFileName string `hcl:"fallback_file_name" validate:"required"`
	writeBlock       *hclwrite.Block
	tokens           hclwrite.Tokens
	newWriteBlock    bool
}

func (u *EnsureLocalTransform) Type() string {
	return "ensure_local"
}

func (u *EnsureLocalTransform) Apply() error {
	u.writeBlock.Body().SetAttributeRaw(u.LocalName, u.tokens)
	if u.newWriteBlock {
		cfg := u.Config().(*MetaProgrammingTFConfig)
		cfg.AddBlock(u.FallbackFileName, u.writeBlock)
	}
	return nil
}

func (u *EnsureLocalTransform) Decode(block *golden.HclBlock, context *hcl.EvalContext) error {
	var err error
	u.LocalName, err = getRequiredStringAttribute("name", block, context)
	if err != nil {
		return err
	}
	u.FallbackFileName, err = getRequiredStringAttribute("fallback_file_name", block, context)
	if err != nil {
		return err
	}
	cfg := u.Config().(*MetaProgrammingTFConfig)
	if b, ok := cfg.localBlocks[fmt.Sprintf("local.%s", u.LocalName)]; ok {
		u.writeBlock = b.WriteBlock
	} else {
		u.writeBlock = hclwrite.NewBlock("locals", []string{})
		u.newWriteBlock = true
	}
	asString, err := getOptionalStringAttribute("value_as_string", block, context)
	if err != nil {
		return err
	}
	raw, asRaw := block.Attributes()["value_as_raw"]
	if asString != nil && asRaw {
		return fmt.Errorf("cannot use both value_as_string and value_as_raw")
	}
	if asString != nil {
		u.tokens, err = stringToHclWriteTokens(*asString)
		if err != nil {
			return err
		}
	}
	if asRaw {
		u.tokens = raw.ExprTokens()
	}
	return nil
}

func (u *EnsureLocalTransform) String() string {
	content := make(map[string]any)
	content["id"] = u.Id()
	content["name"] = u.LocalName
	content["value"] = string(u.tokens.Bytes())
	str, err := json.Marshal(content)
	if err != nil {
		panic(err.Error())
	}
	return string(str)
}
