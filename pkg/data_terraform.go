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

	RequiredVersion   *string `attribute:"required_version"`
	RequiredProviders map[string]RequiredProvider
	Block             cty.Value `attribute:"block"`
}

func (d *TerraformData) Type() string {
	return "terraform"
}

// BaseValues exports provider metadata with string-typed nulls instead of golden's nil pointer values.
func (d *TerraformData) BaseValues() map[string]cty.Value {
	values := d.BaseBlock.BaseValues()
	providers := make(map[string]cty.Value, len(d.RequiredProviders))
	for name, provider := range d.RequiredProviders {
		attributes := map[string]cty.Value{
			"source":  cty.NullVal(cty.String),
			"version": cty.NullVal(cty.String),
		}
		if provider.Source != nil {
			attributes["source"] = cty.StringVal(*provider.Source)
		}
		if provider.Version != nil {
			attributes["version"] = cty.StringVal(*provider.Version)
		}
		providers[name] = cty.ObjectVal(attributes)
	}
	if len(providers) == 0 {
		values["required_providers"] = cty.MapValEmpty(cty.Object(map[string]cty.Type{
			"source":  cty.String,
			"version": cty.String,
		}))
	} else {
		values["required_providers"] = cty.MapVal(providers)
	}
	return values
}

func (d *TerraformData) ExecuteDuringPlan() error {
	d.Block = cty.NilVal
	d.RequiredProviders = nil
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
