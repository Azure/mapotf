package pkg

import (
	"regexp"
	"strings"

	"github.com/Azure/golden"
	"github.com/hashicorp/go-multierror"
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/hashicorp/hcl/v2/hclwrite"
)

var _ Transform = &RegexReplaceExpressionTransform{}

type RegexReplaceExpressionTransform struct {
	*golden.BaseBlock
	*BaseTransform
	Regex       string `hcl:"regex" validator:"required"`
	Replacement string `hcl:"replacement"`
}

func (r *RegexReplaceExpressionTransform) Type() string {
	return "regex_replace_expression"
}

func (r *RegexReplaceExpressionTransform) Apply() error {
	cfg := r.BaseBlock.Config().(*MetaProgrammingTFConfig)
	re, err := regexp.Compile(r.Regex)
	if err != nil {
		return err
	}
	for _, block := range cfg.allRootBlocks {
		if subErr := r.applyRegexReplace(block.WriteBlock.Body(), block.Range().Filename, re); subErr != nil {
			err = multierror.Append(err, subErr)
		}
	}
	return err
}

func (r *RegexReplaceExpressionTransform) applyRegexReplace(body *hclwrite.Body, filename string, re *regexp.Regexp) error {
	var err error
	for name, attr := range body.Attributes() {
		oldValue := strings.TrimSpace(string(attr.Expr().BuildTokens(nil).Bytes()))
		newValue := re.ReplaceAllString(oldValue, r.Replacement)
		if oldValue == newValue {
			continue
		}
		tokens, diag := hclsyntax.LexExpression([]byte(newValue), filename, hcl.InitialPos)
		if diag.HasErrors() {
			err = multierror.Append(err, diag)
			continue
		}
		body.SetAttributeRaw(name, writerTokens(tokens))
	}

	for _, block := range body.Blocks() {
		if subErr := r.applyRegexReplace(block.Body(), filename, re); subErr != nil {
			err = multierror.Append(err, subErr)
			continue
		}
	}
	return err
}
