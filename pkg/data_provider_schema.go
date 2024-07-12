package pkg

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/Azure/golden"
	"github.com/hashicorp/go-multierror"
	tfjson "github.com/hashicorp/terraform-json"
	"github.com/zclconf/go-cty/cty"
	"github.com/zclconf/go-cty/cty/function/stdlib"
)

var _ Data = &ProviderSchemaData{}
var SchemaRetrieverFactory = func(ctx context.Context) TerraformProviderSchemaRetriever {
	return NewTerraformCliProviderSchemaRetriever(ctx)
}

type ProviderSchemaData struct {
	*BaseData
	*golden.BaseBlock

	Source    string    `hcl:"provider_source"`
	Version   string    `hcl:"provider_version"`
	Resources cty.Value `attribute:"resources"`
}

func (r *ProviderSchemaData) Type() string {
	return "provider_schema"
}

func (r *ProviderSchemaData) ExecuteDuringPlan() error {
	schemas, err := SchemaRetrieverFactory(r.Context()).Get(r.Source, r.Version)
	if err != nil {
		return fmt.Errorf("cannot read `terraform prviders schema` for source %s with version %s: %+v", r.Source, r.Version, err)
	}
	r.Resources, err = r.Convert(schemas.ResourceSchemas)
	return err
}

func (r *ProviderSchemaData) Convert(schemas map[string]*tfjson.Schema) (cty.Value, error) {
	resourcesMap := make(map[string]cty.Value)
	var convertErr error

	for resourceName, schema := range schemas {
		attributesMap, err := r.convertAttributeSchemas(schema.Block.Attributes)
		if err != nil {
			convertErr = multierror.Append(err, fmt.Errorf("cannot convert attribute schemas for resource %s: %+v", resourceName, err))
			continue
		}
		nestedBlocksMap, err := r.convertNestedBlockSchemas(schema.Block.NestedBlocks)
		if err != nil {
			convertErr = multierror.Append(err, fmt.Errorf("cannot convert nested block schemas for resource %s: %+v", resourceName, err))
			continue
		}
		resourcesMap[resourceName] = cty.ObjectVal(map[string]cty.Value{
			"version": cty.NumberUIntVal(schema.Version),
			"block": cty.ObjectVal(map[string]cty.Value{
				"attributes":  cty.ObjectVal(attributesMap),
				"block_types": cty.ObjectVal(nestedBlocksMap),
				"description": cty.StringVal(schema.Block.Description),
			}),
		})
	}
	if convertErr != nil {
		return cty.Value{}, convertErr
	}

	return cty.ObjectVal(resourcesMap), nil
}

func (r *ProviderSchemaData) convertAttributeSchemas(attrs map[string]*tfjson.SchemaAttribute) (map[string]cty.Value, error) {
	attributesMap := make(map[string]cty.Value)

	for attrName, attr := range attrs {
		marshal, err := json.Marshal(attr)
		if err != nil {
			return nil, fmt.Errorf("cannot marshal attribute schema for %s: %+v", attrName, err)
		}
		attrObj, err := stdlib.JSONDecode(cty.StringVal(string(marshal)))
		if err != nil {
			return nil, fmt.Errorf("cannot decode attribute schema for %s: %+v", attrName, err)
		}
		attributesMap[attrName] = attrObj
	}
	return attributesMap, nil
}

func (r *ProviderSchemaData) convertNestedBlockSchemas(blocks map[string]*tfjson.SchemaBlockType) (map[string]cty.Value, error) {
	nestedBlocksMap := make(map[string]cty.Value)

	for blockName, block := range blocks {
		marshal, err := json.Marshal(block)
		if err != nil {
			return nil, fmt.Errorf("cannot marshal block schema for %s: %+v", blockName, err)
		}
		nestedBlocksMap[blockName], err = stdlib.JSONDecode(cty.StringVal(string(marshal)))
		if err != nil {
			return nil, fmt.Errorf("cannot decode block schema for %s: %+v", blockName, err)
		}
	}
	return nestedBlocksMap, nil
}
