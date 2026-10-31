package pkg_test

import (
	"context"
	"strings"
	"testing"

	"github.com/Azure/golden"
	"github.com/Azure/mapotf/pkg"
	filesystem "github.com/Azure/mapotf/pkg/fs"
	"github.com/prashantv/gostub"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zclconf/go-cty/cty"
)

func TestDataCheckReflection(t *testing.T) {
	const source = `check "endpoints" {
  # var.location is not a dependency.
  assert {
    condition     = length(var.subnet_endpoints) > 0
    error_message = "var.location is not needed"
  }
}

check "locations" {
  assert {
    condition     = var.location != null && contains([for v in var.regions : lower(v)], lower(var["location"]))
    error_message = "Location ${var.location} is invalid; literal $${var.location}"
  }
  assert {
    condition     = local.selected == var.region_map[var.selected]
    error_message = "second assertion"
  }
  data "http" "probe" {
    url = local.endpoint
    lifecycle {
      postcondition {
        condition     = self.status_code == 200 && var.locations[0] != ""
        error_message = "probe failed"
      }
    }
  }
}
`
	stub := gostub.Stub(&filesystem.Fs, fakeFs(map[string]string{"/main.tf": source}))
	defer stub.Reset()

	cfg, err := pkg.NewMetaProgrammingTFConfig(&pkg.TerraformModuleRef{Dir: "/", AbsDir: "/"}, nil, nil, nil, context.TODO())
	require.NoError(t, err)
	data := &pkg.DataCheck{BaseBlock: golden.NewBaseBlock(cfg, nil), BaseData: &pkg.BaseData{}}
	require.NoError(t, data.ExecuteDuringPlan())
	result := data.Result.AsValueMap()
	require.Len(t, result, 2)

	unrelated := result["endpoints"]
	assert.Equal(t, "check.endpoints", unrelated.GetAttr("mptf").GetAttr("block_address").AsString())
	unrelatedRefs := unrelated.GetAttr("mptf").GetAttr("references").AsValueSlice()
	require.Len(t, unrelatedRefs, 1)
	assert.Equal(t, "var.subnet_endpoints", unrelatedRefs[0].GetAttr("traversal").AsString())
	unrelatedAssert := unrelated.GetAttr("assert").AsValueSlice()
	require.Len(t, unrelatedAssert, 1)
	assert.Equal(t, "var.location is not needed", unrelatedAssert[0].GetAttr("error_message").AsString())

	location := result["locations"]
	assertions := location.GetAttr("assert").AsValueSlice()
	require.Len(t, assertions, 2)
	assert.Equal(t, `var.location != null && contains([for v in var.regions : lower(v)], lower(var["location"]))`, assertions[0].GetAttr("condition").AsString())
	assert.Equal(t, `"Location ${var.location} is invalid; literal $${var.location}"`, assertions[0].GetAttr("error_message").AsString())
	assert.Equal(t, `local.selected == var.region_map[var.selected]`, assertions[1].GetAttr("condition").AsString())
	assert.Equal(t, "second assertion", assertions[1].GetAttr("error_message").AsString())
	postcondition := location.GetAttr("data").AsValueSlice()[0].GetAttr("lifecycle").AsValueSlice()[0].GetAttr("postcondition").AsValueSlice()[0]
	assert.Equal(t, `self.status_code == 200 && var.locations[0] != ""`, postcondition.GetAttr("condition").AsString())
	assert.Equal(t, "probe failed", postcondition.GetAttr("error_message").AsString())

	refs := location.GetAttr("mptf").GetAttr("references").AsValueSlice()
	var traversals []string
	for _, ref := range refs {
		traversals = append(traversals, ref.GetAttr("traversal").AsString())
		assert.Equal(t, "main.tf", ref.GetAttr("range").GetAttr("file_name").AsString())
		line, _ := ref.GetAttr("range").GetAttr("start_line").AsBigFloat().Int64()
		column, _ := ref.GetAttr("range").GetAttr("start_column").AsBigFloat().Int64()
		sourceLine := strings.Split(source, "\n")[line-1]
		assert.True(t, strings.HasPrefix(sourceLine[column-1:], ref.GetAttr("traversal").AsString()))
	}
	assert.Equal(t, []string{
		"var.location", "var.regions", `var["location"]`, "var.location",
		"local.selected", "var.region_map", "var.selected", "local.endpoint",
		"self.status_code", "var.locations[0]",
	}, traversals)
	indexed := refs[2]
	assert.Equal(t, "var", indexed.GetAttr("root").AsString())
	indexStep := indexed.GetAttr("steps").AsValueSlice()[0]
	assert.Equal(t, "index", indexStep.GetAttr("kind").AsString())
	assert.Equal(t, "location", indexStep.GetAttr("value").AsString())
	assert.Equal(t, "string", indexStep.GetAttr("value_type").AsString())
	numberStep := refs[len(refs)-1].GetAttr("steps").AsValueSlice()[1]
	assert.Equal(t, "0", numberStep.GetAttr("value").AsString())
	assert.Equal(t, "number", numberStep.GetAttr("value_type").AsString())
	require.NotNil(t, cfg.RootBlock("check.locations"))
	assert.Contains(t, data.String(), `"references"`)
}

func TestDataCheckPlanFiltersRealLocationReferences(t *testing.T) {
	stub := gostub.Stub(&filesystem.Fs, fakeFs(map[string]string{
		"/main.tf": `check "unrelated" {
  assert {
    condition     = length(var.subnet_endpoints) > 0
    error_message = "var.location"
  }
}
check "location" {
  assert {
    condition     = var.location != null
    error_message = "location is required"
  }
}
check "indexed_location" {
  assert {
    condition     = var["location"] != null
    error_message = "location is required"
  }
}
`,
		"/main.mptf.hcl": `data "check" "all" {}
transform "remove_block" "location_checks" {
  for_each = {
    for name, check in data.check.all.result : name => check
    if length([for ref in check.mptf.references : ref if ref.root == "var" && try(ref.steps[0].value == "location", false)]) > 0
  }
  target_block_address = each.value.mptf.block_address
}
`,
	}))
	defer stub.Reset()

	blocks, err := pkg.LoadMPTFHclBlocks(false, "/")
	require.NoError(t, err)
	cfg, err := pkg.NewMetaProgrammingTFConfig(&pkg.TerraformModuleRef{Dir: "/", AbsDir: "/"}, nil, blocks, nil, context.TODO())
	require.NoError(t, err)
	plan, err := pkg.RunMetaProgrammingTFPlan(cfg)
	require.NoError(t, err)
	require.Len(t, plan.Transforms, 2)
	require.NoError(t, plan.Apply())
	actual, err := afero.ReadFile(filesystem.Fs, "/main.tf")
	require.NoError(t, err)
	assert.Contains(t, string(actual), `check "unrelated"`)
	assert.NotContains(t, string(actual), `check "location"`)
	assert.NotContains(t, string(actual), `check "indexed_location"`)
}

func TestDataCheckReferencesSplatConditionalAndHeredoc(t *testing.T) {
	source := `check "expressions" {
  assert {
    condition = var.enabled ? length(var.regions[*].name) > 0 : contains(var.items, local.default) && var["loc\u0061tion"] != null
    error_message = <<-EOT
    Location: ${var.location}; literal $${var.location}
    EOT
  }
}
`
	stub := gostub.Stub(&filesystem.Fs, fakeFs(map[string]string{"/main.tf": source}))
	defer stub.Reset()
	cfg, err := pkg.NewMetaProgrammingTFConfig(&pkg.TerraformModuleRef{Dir: "/", AbsDir: "/"}, nil, nil, nil, context.TODO())
	require.NoError(t, err)
	data := &pkg.DataCheck{BaseBlock: golden.NewBaseBlock(cfg, nil), BaseData: &pkg.BaseData{}}
	require.NoError(t, data.ExecuteDuringPlan())
	refs := data.Result.GetAttr("expressions").GetAttr("mptf").GetAttr("references").AsValueSlice()
	var traversals []string
	for _, ref := range refs {
		traversals = append(traversals, ref.GetAttr("traversal").AsString())
	}
	assert.Equal(t, []string{"var.enabled", "var.regions", "var.items", "local.default", `var["loc\u0061tion"]`, "var.location"}, traversals)
	assert.Equal(t, "location", refs[4].GetAttr("steps").AsValueSlice()[0].GetAttr("value").AsString())
	message := data.Result.GetAttr("expressions").GetAttr("assert").AsValueSlice()[0].GetAttr("error_message").AsString()
	assert.Contains(t, message, "${var.location}")
	assert.Contains(t, message, "$${var.location}")
}

func TestDataCheckRejectsMissingLabel(t *testing.T) {
	stub := gostub.Stub(&filesystem.Fs, fakeFs(map[string]string{"/main.tf": `check {
  assert {
    condition = true
    error_message = "invalid"
  }
}`}))
	defer stub.Reset()
	cfg, err := pkg.NewMetaProgrammingTFConfig(&pkg.TerraformModuleRef{Dir: "/", AbsDir: "/"}, nil, nil, nil, context.TODO())
	require.NoError(t, err)
	data := &pkg.DataCheck{BaseBlock: golden.NewBaseBlock(cfg, nil), BaseData: &pkg.BaseData{}}
	require.ErrorContains(t, data.ExecuteDuringPlan(), "must have exactly one label")
}

func TestDataCheckEmptyAndLiteralReferences(t *testing.T) {
	for _, tc := range []struct {
		name   string
		source string
		count  int
	}{
		{name: "no checks", source: `variable "location" { type = string }`, count: 0},
		{name: "literal only", source: `check "literal" {
  assert {
    condition = true
    error_message = "var.location"
  }
}`, count: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			stub := gostub.Stub(&filesystem.Fs, fakeFs(map[string]string{"/main.tf": tc.source}))
			defer stub.Reset()
			cfg, err := pkg.NewMetaProgrammingTFConfig(&pkg.TerraformModuleRef{Dir: "/", AbsDir: "/"}, nil, nil, nil, context.TODO())
			require.NoError(t, err)
			data := &pkg.DataCheck{BaseBlock: golden.NewBaseBlock(cfg, nil), BaseData: &pkg.BaseData{}}
			require.NoError(t, data.ExecuteDuringPlan())
			assert.Len(t, data.Result.AsValueMap(), tc.count)
			if tc.count != 0 {
				refs := data.Result.GetAttr("literal").GetAttr("mptf").GetAttr("references")
				assert.True(t, refs.RawEquals(cty.ListValEmpty(refs.Type().ElementType())))
			}
		})
	}
}
