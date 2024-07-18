package pkg

import (
	"fmt"
	"github.com/Azure/golden"
	"github.com/hashicorp/hcl/v2"
)

type RequiredProvider struct {
	Source  *string `attribute:"source"`
	Version *string `attribute:"version"`
}

var _ Data = &TerraformData{}

type TerraformData struct {
	*BaseData
	*golden.BaseBlock

	RequiredVersion   *string                     `attribute:"required_version"`
	RequiredProviders map[string]RequiredProvider `attribute:"required_providers"`
}

func (d *TerraformData) Type() string {
	return "terraform"
}

func (d *TerraformData) ExecuteDuringPlan() error {
	tb := d.BaseBlock.Config().(*MetaProgrammingTFConfig).TerraformBlock()
	if tb == nil {
		return nil
	}
	requiredTerraformVersion, ok := tb.Attributes["required_version"]
	if ok {
		v, diag := requiredTerraformVersion.Expr.Value(&hcl.EvalContext{})
		if diag.HasErrors() {
			return fmt.Errorf("error while evaluating terraform block's `required_version`: %+v", diag)
		}
		s := v.AsString()
		d.RequiredVersion = &s
	}
	rp, ok := tb.NestedBlocks["required_providers"]
	if !ok || len(rp) == 0 {
		return nil
	}
	d.RequiredProviders = make(map[string]RequiredProvider)
	for s, p := range rp[0].Body.Attributes {
		providerConfig, diag := p.Expr.Value(&hcl.EvalContext{})
		if diag.HasErrors() {
			return fmt.Errorf("error while evaluating terraform block's `required_providers.%s`: %+v", s, diag)
		}
		p := RequiredProvider{}
		it := providerConfig.ElementIterator()
		for it.Next() {
			k, _ := it.Element()
			if k.AsString() == "source" {
				source := providerConfig.GetAttr("source").AsString()
				p.Source = &source
			}
			if k.AsString() == "version" {
				version := providerConfig.GetAttr("version").AsString()
				p.Version = &version
			}
		}
		d.RequiredProviders[s] = p
	}
	return nil
}
