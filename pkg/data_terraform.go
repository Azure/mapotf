package pkg

import (
	"fmt"
	"github.com/Azure/golden"
	"github.com/hashicorp/hcl/v2"
	"github.com/zclconf/go-cty/cty"
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
	Block             cty.Value                   `attribute:"block"`
}

func (d *TerraformData) Type() string {
	return "terraform"
}

func (d *TerraformData) ExecuteDuringPlan() error {
	d.Block = cty.NilVal
	tb := d.BaseBlock.Config().(*MetaProgrammingTFConfig).TerraformBlock()
	if tb == nil {
		return nil
	}
	d.Block = tb.EvalContext()
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
	for providerName, providerAttribute := range rp[0].Body.Attributes {
		providerEntries, diag := hcl.ExprMap(providerAttribute.Expr)
		if diag.HasErrors() {
			return fmt.Errorf("error while evaluating terraform block's `required_providers.%s`: %+v", providerName, diag)
		}
		provider := RequiredProvider{}
		for _, entry := range providerEntries {
			key, diag := entry.Key.Value(&hcl.EvalContext{})
			if diag.HasErrors() {
				return fmt.Errorf("error while evaluating terraform block's `required_providers.%s` key: %+v", providerName, diag)
			}
			attributeName := key.AsString()
			if attributeName != "source" && attributeName != "version" {
				continue
			}
			value, diag := entry.Value.Value(&hcl.EvalContext{})
			if diag.HasErrors() {
				return fmt.Errorf("error while evaluating terraform block's `required_providers.%s.%s`: %+v", providerName, attributeName, diag)
			}
			attributeValue := value.AsString()
			switch attributeName {
			case "source":
				provider.Source = &attributeValue
			case "version":
				provider.Version = &attributeValue
			}
		}
		d.RequiredProviders[providerName] = provider
	}
	return nil
}
