package pkg

import (
	"context"
	"fmt"
	"sort"

	"github.com/Azure/golden"
	"github.com/hashicorp/terraform-config-inspect/tfconfig"
	"github.com/zclconf/go-cty/cty"
	ctyjson "github.com/zclconf/go-cty/cty/json"
)

var _ Data = &ModuleSourceData{}

// ModuleSourceFetcherFactory is overridable in tests; the default returns a
// CLI-backed fetcher that runs `terraform get` in a temp directory.
var ModuleSourceFetcherFactory = func(ctx context.Context) TerraformModuleSourceFetcher {
	return NewTerraformCliModuleSourceFetcher(ctx)
}

// ModuleSourceData fetches the variables and outputs of a Terraform module
// (by `source` + optional `version`) and exposes them as cty values that
// downstream transforms can compose against. The pre-sorted
// `required_variables` / `optional_variables` lists are intended to be fed
// into `reorder_attributes.body_attributes` so a `module.<name>` block's
// inputs end up in required-then-optional alphabetical order.
//
// `source` accepts the same values you'd write in a `module { source = "..." }`
// block: registry shortcuts (`Azure/naming/azurerm`), git URLs, and local
// paths (`./submod`, `../../`, absolute paths). Local paths are resolved
// against `base_dir`, which defaults to the target Terraform module's
// directory so callers normally don't need to set it.
type ModuleSourceData struct {
	*BaseData
	*golden.BaseBlock

	Source            string    `hcl:"source"`
	Version           string    `hcl:"version,optional"`
	BaseDir           string    `hcl:"base_dir,optional"`
	Variables         cty.Value `attribute:"variables"`
	Outputs           cty.Value `attribute:"outputs"`
	RequiredVariables cty.Value `attribute:"required_variables"`
	OptionalVariables cty.Value `attribute:"optional_variables"`
}

func (d *ModuleSourceData) Type() string {
	return "module_source"
}

func (d *ModuleSourceData) ExecuteDuringPlan() error {
	baseDir := d.BaseDir
	if baseDir == "" && d.BaseBlock != nil {
		if cfg, ok := d.Config().(*MetaProgrammingTFConfig); ok {
			baseDir = cfg.ModuleDir()
		}
	}
	mod, err := ModuleSourceFetcherFactory(d.Context()).Get(d.Source, d.Version, baseDir)
	if err != nil {
		return fmt.Errorf("cannot fetch module source %q version %q: %w", d.Source, d.Version, err)
	}
	d.Variables = convertModuleVariables(mod.Variables)
	d.Outputs = convertModuleOutputs(mod.Outputs)
	d.RequiredVariables, d.OptionalVariables = partitionModuleVariableNames(mod.Variables)
	return nil
}

func convertModuleVariables(vars map[string]*tfconfig.Variable) cty.Value {
	if len(vars) == 0 {
		return cty.EmptyObjectVal
	}
	out := make(map[string]cty.Value, len(vars))
	for name, v := range vars {
		var def cty.Value
		if v.Default == nil {
			def = cty.NullVal(cty.String)
		} else {
			// tfconfig.Variable.Default is interface{} approximating the
			// configured default. Render it as a string so it's safe to
			// expose as a string-typed cty value regardless of original type.
			def = cty.StringVal(fmt.Sprintf("%v", v.Default))
		}
		out[name] = cty.ObjectVal(map[string]cty.Value{
			"required":    cty.BoolVal(v.Required),
			"type":        cty.StringVal(v.Type),
			"description": cty.StringVal(v.Description),
			"sensitive":   cty.BoolVal(v.Sensitive),
			"default":     def,
		})
	}
	return cty.ObjectVal(out)
}

func convertModuleOutputs(outs map[string]*tfconfig.Output) cty.Value {
	if len(outs) == 0 {
		return cty.EmptyObjectVal
	}
	out := make(map[string]cty.Value, len(outs))
	for name, o := range outs {
		out[name] = cty.ObjectVal(map[string]cty.Value{
			"description": cty.StringVal(o.Description),
			"sensitive":   cty.BoolVal(o.Sensitive),
		})
	}
	return cty.ObjectVal(out)
}

func partitionModuleVariableNames(vars map[string]*tfconfig.Variable) (required, optional cty.Value) {
	var req, opt []string
	for name, v := range vars {
		if v.Required {
			req = append(req, name)
		} else {
			opt = append(opt, name)
		}
	}
	sort.Strings(req)
	sort.Strings(opt)
	return stringListOrEmpty(req), stringListOrEmpty(opt)
}

func stringListOrEmpty(items []string) cty.Value {
	if len(items) == 0 {
		return cty.ListValEmpty(cty.String)
	}
	vals := make([]cty.Value, len(items))
	for i, s := range items {
		vals[i] = cty.StringVal(s)
	}
	return cty.ListVal(vals)
}

func (d *ModuleSourceData) String() string {
	data := cty.ObjectVal(map[string]cty.Value{
		"source":             cty.StringVal(d.Source),
		"version":            cty.StringVal(d.Version),
		"base_dir":           cty.StringVal(d.BaseDir),
		"variables":          d.Variables,
		"outputs":            d.Outputs,
		"required_variables": d.RequiredVariables,
		"optional_variables": d.OptionalVariables,
	})
	r, err := ctyjson.Marshal(data, data.Type())
	if err != nil {
		panic(err.Error())
	}
	return string(r)
}
