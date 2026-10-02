package pkg

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/Azure/golden"
	filesystem "github.com/Azure/mapotf/pkg/fs"
	"github.com/Azure/mapotf/pkg/terraform"
	"github.com/zclconf/go-cty/cty"
	ctyjson "github.com/zclconf/go-cty/cty/json"
)

var _ Data = &TestFileData{}

type TestFileData struct {
	*BaseData
	*golden.BaseBlock

	Result cty.Value `attribute:"result"`
}

func (d *TestFileData) Type() string {
	return "test_file"
}

func (d *TestFileData) ExecuteDuringPlan() error {
	cfg := d.Config().(*MetaProgrammingTFConfig)
	if cfg.module.TestFile == "" {
		return fmt.Errorf("data test_file requires test-file mode")
	}
	variables := cty.NullVal(cty.DynamicPseudoType)
	runs := make(map[string]cty.Value)
	mocks := make(map[string]cty.Value)
	providers := make(map[string]cty.Value)
	modules := make(map[string]cty.Value)
	for _, block := range cfg.module.TestBlocks {
		switch block.Type {
		case "variables":
			variables = block.EvalContext()
		case "run":
			name := block.Labels[0]
			runs[name] = block.EvalContext()
			target, err := testRunModule(block, cfg.ModuleDir())
			if err != nil {
				return err
			}
			modules[name] = target
		case "mock_provider":
			mocks[strings.TrimPrefix(block.Address, "mock_provider.")] = block.EvalContext()
		case "provider":
			providers[strings.TrimPrefix(block.Address, "provider.")] = block.EvalContext()
		}
	}
	d.Result = cty.ObjectVal(map[string]cty.Value{
		"filename":       cty.StringVal(cfg.module.TestFile),
		"module_dir":     cty.StringVal(cfg.ModuleDir()),
		"variables":      variables,
		"runs":           cty.ObjectVal(runs),
		"mock_providers": cty.ObjectVal(mocks),
		"providers":      cty.ObjectVal(providers),
		"run_modules":    cty.ObjectVal(modules),
	})
	return nil
}

func testRunModule(run *terraform.RootBlock, moduleDir string) (cty.Value, error) {
	kind := "root"
	source := cty.NullVal(cty.String)
	dir := cty.StringVal(moduleDir)
	modules := run.NestedBlocks["module"]
	if len(modules) > 0 {
		attribute := modules[0].Attributes["source"]
		value, diag := attribute.Expr.Value(nil)
		if diag.HasErrors() || value.IsNull() || !value.IsKnown() || value.Type() != cty.String {
			return cty.NilVal, fmt.Errorf("%s: module source must be a known string", run.Range())
		}
		source = value
		kind = "remote"
		dir = cty.NullVal(cty.String)
		if isLocalSource(source.AsString()) {
			kind = "local"
			path := source.AsString()
			if !filepath.IsAbs(path) {
				path = filepath.Join(moduleDir, path)
			}
			info, err := filesystem.Fs.Stat(path)
			if err != nil {
				return cty.NilVal, fmt.Errorf("%s: cannot inspect local run module %q: %w", run.Range(), path, err)
			}
			if !info.IsDir() {
				return cty.NilVal, fmt.Errorf("%s: local run module %q is not a directory", run.Range(), path)
			}
			dir = cty.StringVal(filepath.Clean(path))
		}
	}
	return cty.ObjectVal(map[string]cty.Value{
		"kind": cty.StringVal(kind), "source": source, "dir": dir,
	}), nil
}

func (d *TestFileData) String() string {
	result, err := ctyjson.Marshal(d.Result, d.Result.Type())
	if err != nil {
		panic(err.Error())
	}
	return string(result)
}
