package pkg_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/Azure/mapotf/pkg"
	filesystem "github.com/Azure/mapotf/pkg/fs"
	"github.com/hashicorp/terraform-config-inspect/tfconfig"
	"github.com/prashantv/gostub"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stubModuleSourceFetcher struct {
	mod *tfconfig.Module
	err error
}

func (s *stubModuleSourceFetcher) Get(source, version string) (*tfconfig.Module, error) {
	return s.mod, s.err
}

// fakeAvmNamingModule mirrors the shape of a real AVM module reasonably
// closely: a couple of required inputs (no Default), a handful of optional
// inputs with diverse Default types (string, bool, map), a sensitive optional,
// and two outputs. Required.true is set explicitly because tfconfig populates
// it from Default-absence at load time (real fixture would derive it via
// LoadModule, but here we mimic the post-load state directly).
func fakeAvmNamingModule() *tfconfig.Module {
	return &tfconfig.Module{
		Variables: map[string]*tfconfig.Variable{
			// Required (no default).
			"name":     {Name: "name", Type: "string", Required: true, Description: "the name"},
			"location": {Name: "location", Type: "string", Required: true},
			// Optional, different default types.
			"tags":        {Name: "tags", Type: "map(string)", Required: false, Default: map[string]interface{}{}},
			"enable_diag": {Name: "enable_diag", Type: "bool", Required: false, Default: false},
			"prefix":      {Name: "prefix", Type: "string", Required: false, Default: ""},
			// Sensitive optional.
			"password": {Name: "password", Type: "string", Required: false, Default: "", Sensitive: true},
		},
		Outputs: map[string]*tfconfig.Output{
			"id":     {Name: "id", Description: "the id"},
			"secret": {Name: "secret", Sensitive: true},
		},
	}
}

func TestDataModuleSource_PartitionsRequiredAndOptional(t *testing.T) {
	mod := fakeAvmNamingModule()
	stub := gostub.Stub(&pkg.ModuleSourceFetcherFactory, func(ctx context.Context) pkg.TerraformModuleSourceFetcher {
		return &stubModuleSourceFetcher{mod: mod}
	}).Stub(&filesystem.Fs, fakeFs(map[string]string{
		"/main.tf": `resource "azurerm_resource_group" this {
}
`,
	}))
	defer stub.Reset()

	hclBlocks := newHclBlocks(t, `
data "module_source" "naming" {
  source  = "Azure/naming/azurerm"
  version = "~> 0.4"
}

transform "update_in_place" req {
  for_each             = { for i, n in data.module_source.naming.required_variables : format("%04d", i) => n }
  target_block_address = "resource.azurerm_resource_group.this"
  asstring {
    description = each.value
  }
}

transform "update_in_place" opt {
  for_each             = { for i, n in data.module_source.naming.optional_variables : format("%04d", i) => n }
  target_block_address = "resource.azurerm_resource_group.this"
  asstring {
    description = each.value
  }
}

transform "update_in_place" vars_keys {
  for_each             = { for i, n in sort(keys(data.module_source.naming.variables)) : format("%04d", i) => n }
  target_block_address = "resource.azurerm_resource_group.this"
  asstring {
    description = each.value
  }
}

transform "update_in_place" out_keys {
  for_each             = { for i, n in sort(keys(data.module_source.naming.outputs)) : format("%04d", i) => n }
  target_block_address = "resource.azurerm_resource_group.this"
  asstring {
    description = each.value
  }
}
`)
	cfg, err := pkg.NewMetaProgrammingTFConfig(&pkg.TerraformModuleRef{
		Dir:    ".",
		AbsDir: "/",
	}, nil, nil, nil, context.TODO())
	require.NoError(t, err)
	require.NoError(t, cfg.Init(hclBlocks))
	require.NoError(t, cfg.RunPrePlan())
	require.NoError(t, cfg.RunPlan())

	got := collectForEachStrings(t, cfg, "update_in_place", []string{
		"req", "opt", "vars_keys", "out_keys",
	}, "description")

	// Required variables: alphabetical, partitioned by tfconfig.Variable.Required.
	assert.Equal(t, []string{"location", "name"}, got["req"])
	// Optional variables: alphabetical, everything with Required:false.
	assert.Equal(t, []string{"enable_diag", "password", "prefix", "tags"}, got["opt"])
	// The full variables object exposes every variable name.
	assert.Equal(t, []string{"enable_diag", "location", "name", "password", "prefix", "tags"}, got["vars_keys"])
	// The full outputs object exposes every output name.
	assert.Equal(t, []string{"id", "secret"}, got["out_keys"])
}

func TestDataModuleSource_StringJSONShape(t *testing.T) {
	mod := &tfconfig.Module{
		Variables: map[string]*tfconfig.Variable{
			"only_required": {Name: "only_required", Type: "string", Required: true, Description: "d"},
			"only_optional": {Name: "only_optional", Type: "string", Required: false, Default: "hello"},
		},
		Outputs: map[string]*tfconfig.Output{
			"o": {Name: "o", Description: "out"},
		},
	}
	stub := gostub.Stub(&pkg.ModuleSourceFetcherFactory, func(ctx context.Context) pkg.TerraformModuleSourceFetcher {
		return &stubModuleSourceFetcher{mod: mod}
	})
	defer stub.Reset()

	d := &pkg.ModuleSourceData{
		BaseData:  &pkg.BaseData{},
		Source:    "Azure/naming/azurerm",
		Version:   "~> 0.4",
	}
	require.NoError(t, d.ExecuteDuringPlan())

	var got map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(d.String()), &got))

	assert.Equal(t, "Azure/naming/azurerm", got["source"])
	assert.Equal(t, "~> 0.4", got["version"])
	require.Contains(t, got, "variables")
	require.Contains(t, got, "outputs")
	require.Contains(t, got, "required_variables")
	require.Contains(t, got, "optional_variables")

	req, _ := got["required_variables"].([]interface{})
	assert.Equal(t, []interface{}{"only_required"}, req)
	opt, _ := got["optional_variables"].([]interface{})
	assert.Equal(t, []interface{}{"only_optional"}, opt)

	// outputs object must have no "required" field (outputs are always emitted).
	outs, _ := got["outputs"].(map[string]interface{})
	require.NotNil(t, outs)
	out, _ := outs["o"].(map[string]interface{})
	require.NotNil(t, out)
	_, hasRequired := out["required"]
	assert.False(t, hasRequired, "outputs object must not expose a 'required' field")
	assert.Equal(t, "out", out["description"])
	assert.Equal(t, false, out["sensitive"])

	// Variables object exposes per-variable required/type/description/sensitive/default.
	vars, _ := got["variables"].(map[string]interface{})
	require.NotNil(t, vars)
	req1, _ := vars["only_required"].(map[string]interface{})
	require.NotNil(t, req1)
	assert.Equal(t, true, req1["required"])
	assert.Equal(t, "string", req1["type"])
	assert.Equal(t, "d", req1["description"])
	assert.Nil(t, req1["default"])

	opt1, _ := vars["only_optional"].(map[string]interface{})
	require.NotNil(t, opt1)
	assert.Equal(t, false, opt1["required"])
	assert.Equal(t, "hello", opt1["default"])
}

func TestDataModuleSource_EmptyModule(t *testing.T) {
	mod := &tfconfig.Module{
		Variables: map[string]*tfconfig.Variable{},
		Outputs:   map[string]*tfconfig.Output{},
	}
	stub := gostub.Stub(&pkg.ModuleSourceFetcherFactory, func(ctx context.Context) pkg.TerraformModuleSourceFetcher {
		return &stubModuleSourceFetcher{mod: mod}
	})
	defer stub.Reset()

	d := &pkg.ModuleSourceData{
		BaseData: &pkg.BaseData{},
		Source:   "some/empty/module",
	}
	require.NoError(t, d.ExecuteDuringPlan())

	// Empty module must produce empty (not panicked, not null) collections so
	// downstream `concat(required_variables, optional_variables)` keeps working.
	assert.True(t, d.RequiredVariables.LengthInt() == 0)
	assert.True(t, d.OptionalVariables.LengthInt() == 0)
}
