package pkg

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"testing"

	"github.com/Azure/golden"
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/hashicorp/hcl/v2/hclwrite"
	tfjson "github.com/hashicorp/terraform-json"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTerraformCliProviderSchemaRetriever_BatchesMultipleProviders(t *testing.T) {
	runner := &recordingProviderSchemaRunner{}
	retriever := newTestProviderSchemaRetriever(context.Background(), runner)
	requests := avmProviderSchemaRequests()

	require.NoError(t, retriever.prefetch(requests))
	for _, request := range requests {
		schema, err := retriever.Get(request.providerSource, request.versionConstraint)
		require.NoError(t, err)
		require.NotNil(t, schema)
	}

	require.Len(t, runner.calls, 1)
	assert.Equal(t, providerSchemaRequestKeys(requests), providerSchemaRequestKeys(runner.calls[0]))
}

func TestTerraformProviderConfig_UsesValidLocalNames(t *testing.T) {
	config := string(terraformProviderConfig(avmProviderSchemaRequests()))

	assert.Contains(t, config, "provider0 =")
	assert.Contains(t, config, "provider1 =")
	assert.Contains(t, config, "provider2 =")
	assert.NotContains(t, config, "provider_")
}

func TestTerraformCliProviderSchemaRetriever_DeduplicatesEquivalentRequests(t *testing.T) {
	runner := &recordingProviderSchemaRunner{}
	retriever := newTestProviderSchemaRetriever(context.Background(), runner)
	requests := []providerSchemaRequest{
		{providerSource: "Azure/azapi", versionConstraint: "~> 2.4"},
		{providerSource: "azure/azapi", versionConstraint: "~> 2.4"},
		{providerSource: "registry.terraform.io/azure/azapi", versionConstraint: "~> 2.4"},
	}

	require.NoError(t, retriever.prefetch(requests))
	for _, request := range requests {
		_, err := retriever.Get(request.providerSource, request.versionConstraint)
		require.NoError(t, err)
	}

	require.Len(t, runner.calls, 1)
	require.Len(t, runner.calls[0], 1)
}

func TestTerraformCliProviderSchemaRetriever_DeduplicatesConcurrentRequests(t *testing.T) {
	runner := &recordingProviderSchemaRunner{}
	retriever := newTestProviderSchemaRetriever(context.Background(), runner)
	request := providerSchemaRequest{providerSource: "hashicorp/random", versionConstraint: "~> 3.0"}

	var waitGroup sync.WaitGroup
	errCh := make(chan error, 8)
	for range 8 {
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			_, err := retriever.Get(request.providerSource, request.versionConstraint)
			errCh <- err
		}()
	}
	waitGroup.Wait()
	close(errCh)

	for err := range errCh {
		require.NoError(t, err)
	}
	require.Len(t, runner.calls, 1)
}

func TestTerraformCliProviderSchemaRetriever_SeparatesConflictingConstraints(t *testing.T) {
	runner := &recordingProviderSchemaRunner{}
	retriever := newTestProviderSchemaRetriever(context.Background(), runner)
	requests := []providerSchemaRequest{
		{providerSource: "hashicorp/azurerm", versionConstraint: "~> 3.0"},
		{providerSource: "hashicorp/azurerm", versionConstraint: "~> 4.0"},
		{providerSource: "hashicorp/random", versionConstraint: "~> 3.0"},
	}

	require.NoError(t, retriever.prefetch(requests))

	require.Len(t, runner.calls, 2)
	requestCount := 0
	for _, call := range runner.calls {
		requestCount += len(call)
		sources := make(map[string]struct{}, len(call))
		for _, request := range call {
			source := request.key().providerSource
			_, duplicate := sources[source]
			assert.False(t, duplicate)
			sources[source] = struct{}{}
		}
	}
	assert.Equal(t, len(requests), requestCount)
}

func TestTerraformCliProviderSchemaRetriever_IsolatesBatchFailures(t *testing.T) {
	runner := &recordingProviderSchemaRunner{
		run: func(_ context.Context, requests []providerSchemaRequest) (*tfjson.ProviderSchemas, error) {
			if len(requests) > 1 {
				return nil, errors.New("combined init failed")
			}
			if canonicalProviderSource(requests[0].providerSource) == "registry.terraform.io/example/broken" {
				return nil, errors.New("invalid provider source")
			}
			return providerSchemasFor(requests), nil
		},
	}
	retriever := newTestProviderSchemaRetriever(context.Background(), runner)
	valid := providerSchemaRequest{providerSource: "hashicorp/random", versionConstraint: "~> 3.0"}
	invalid := providerSchemaRequest{providerSource: "example/broken", versionConstraint: "~> 1.0"}

	require.NoError(t, retriever.prefetch([]providerSchemaRequest{valid, invalid}))

	schema, err := retriever.Get(valid.providerSource, valid.versionConstraint)
	require.NoError(t, err)
	require.NotNil(t, schema)
	schema, err = retriever.Get(invalid.providerSource, invalid.versionConstraint)
	require.ErrorContains(t, err, "invalid provider source")
	assert.Nil(t, schema)
	assert.Len(t, runner.calls, 3)
}

func TestTerraformCliProviderSchemaRetriever_StopsOnCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	runner := &recordingProviderSchemaRunner{}
	retriever := newTestProviderSchemaRetriever(ctx, runner)

	err := retriever.prefetch(avmProviderSchemaRequests())

	require.ErrorIs(t, err, context.Canceled)
	assert.Empty(t, runner.calls)
}

func TestTerraformCliProviderSchemaRetriever_DoesNotRetryCanceledBatch(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	runner := &recordingProviderSchemaRunner{
		run: func(_ context.Context, _ []providerSchemaRequest) (*tfjson.ProviderSchemas, error) {
			cancel()
			return nil, context.Canceled
		},
	}
	retriever := newTestProviderSchemaRetriever(ctx, runner)

	err := retriever.prefetch(avmProviderSchemaRequests())

	require.ErrorIs(t, err, context.Canceled)
	assert.Len(t, runner.calls, 1)
}

func TestMetaProgrammingTFConfig_RunPlanPrefetchesProviderSchemas(t *testing.T) {
	moduleDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(moduleDir, "main.tf"), []byte(`resource "example_resource" "this" {}`), 0600))
	config, err := NewMetaProgrammingTFConfig(&TerraformModuleRef{
		Dir:    moduleDir,
		AbsDir: moduleDir,
	}, nil, providerSchemaHCLBlocks(t), nil, context.Background())
	require.NoError(t, err)

	runner := &recordingProviderSchemaRunner{}
	retriever := newTestProviderSchemaRetriever(context.Background(), runner)
	config.providerSchemaRetrieverOnce.Do(func() {
		config.providerSchemaRetriever = retriever
	})

	require.NoError(t, config.RunPlan())
	require.Len(t, runner.calls, 1)
	assert.Equal(t, providerSchemaRequestKeys(avmProviderSchemaRequests()), providerSchemaRequestKeys(runner.calls[0]))
}

func BenchmarkTerraformCliProviderSchemaRetriever_ThreeProviders(b *testing.B) {
	requests := avmProviderSchemaRequests()

	b.Run("individual", func(b *testing.B) {
		b.ReportAllocs()
		cycles := 0
		for i := 0; i < b.N; i++ {
			runner := &recordingProviderSchemaRunner{}
			retriever := newTestProviderSchemaRetriever(context.Background(), runner)
			for _, request := range requests {
				if _, err := retriever.Get(request.providerSource, request.versionConstraint); err != nil {
					b.Fatal(err)
				}
			}
			cycles += len(runner.calls)
		}
		b.ReportMetric(float64(cycles)/float64(b.N), "terraform_cycles/op")
	})

	b.Run("batched", func(b *testing.B) {
		b.ReportAllocs()
		cycles := 0
		for i := 0; i < b.N; i++ {
			runner := &recordingProviderSchemaRunner{}
			retriever := newTestProviderSchemaRetriever(context.Background(), runner)
			if err := retriever.prefetch(requests); err != nil {
				b.Fatal(err)
			}
			for _, request := range requests {
				if _, err := retriever.Get(request.providerSource, request.versionConstraint); err != nil {
					b.Fatal(err)
				}
			}
			cycles += len(runner.calls)
		}
		b.ReportMetric(float64(cycles)/float64(b.N), "terraform_cycles/op")
	})
}

type recordingProviderSchemaRunner struct {
	calls [][]providerSchemaRequest
	run   func(context.Context, []providerSchemaRequest) (*tfjson.ProviderSchemas, error)
}

func (r *recordingProviderSchemaRunner) ProviderSchemas(ctx context.Context, workingDir string) (*tfjson.ProviderSchemas, error) {
	requests, err := readProviderSchemaRequests(filepath.Join(workingDir, "main.tf"))
	if err != nil {
		return nil, err
	}
	r.calls = append(r.calls, requests)
	if r.run != nil {
		return r.run(ctx, requests)
	}
	return providerSchemasFor(requests), nil
}

func newTestProviderSchemaRetriever(ctx context.Context, runner providerSchemaRunner) TerraformCliProviderSchemaRetriever {
	state := newProviderSchemaRetrieverState()
	state.runnerFactory = func(TerraformCliProviderSchemaRetriever) (providerSchemaRunner, error) {
		return runner, nil
	}
	return TerraformCliProviderSchemaRetriever{
		ctx:   ctx,
		state: state,
	}
}

func readProviderSchemaRequests(path string) ([]providerSchemaRequest, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	file, diagnostics := hclsyntax.ParseConfig(content, path, hcl.InitialPos)
	if diagnostics.HasErrors() {
		return nil, diagnostics
	}

	var requests []providerSchemaRequest
	for _, block := range file.Body.(*hclsyntax.Body).Blocks {
		if block.Type != "terraform" {
			continue
		}
		for _, nestedBlock := range block.Body.Blocks {
			if nestedBlock.Type != "required_providers" {
				continue
			}
			for _, attribute := range nestedBlock.Body.Attributes {
				value, diagnostics := attribute.Expr.Value(nil)
				if diagnostics.HasErrors() {
					return nil, diagnostics
				}
				requests = append(requests, providerSchemaRequest{
					providerSource:    value.GetAttr("source").AsString(),
					versionConstraint: value.GetAttr("version").AsString(),
				})
			}
		}
	}
	sort.Slice(requests, func(i, j int) bool {
		left, right := requests[i].key(), requests[j].key()
		if left.providerSource != right.providerSource {
			return left.providerSource < right.providerSource
		}
		return left.versionConstraint < right.versionConstraint
	})
	if len(requests) == 0 {
		return nil, fmt.Errorf("no required providers found in %s", path)
	}
	return requests, nil
}

func providerSchemasFor(requests []providerSchemaRequest) *tfjson.ProviderSchemas {
	schemas := make(map[string]*tfjson.ProviderSchema, len(requests))
	for _, request := range requests {
		schemas[canonicalProviderSource(request.providerSource)] = &tfjson.ProviderSchema{
			ResourceSchemas:   map[string]*tfjson.Schema{},
			DataSourceSchemas: map[string]*tfjson.Schema{},
		}
	}
	return &tfjson.ProviderSchemas{Schemas: schemas}
}

func providerSchemaRequestKeys(requests []providerSchemaRequest) []providerSchemaCacheKey {
	keys := make([]providerSchemaCacheKey, len(requests))
	for i, request := range requests {
		keys[i] = request.key()
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].providerSource != keys[j].providerSource {
			return keys[i].providerSource < keys[j].providerSource
		}
		return keys[i].versionConstraint < keys[j].versionConstraint
	})
	return keys
}

func avmProviderSchemaRequests() []providerSchemaRequest {
	return []providerSchemaRequest{
		{providerSource: "hashicorp/azurerm", versionConstraint: "~> 4.0"},
		{providerSource: "Azure/azapi", versionConstraint: "~> 2.4"},
		{providerSource: "hashicorp/random", versionConstraint: "~> 3.0"},
	}
}

func providerSchemaHCLBlocks(t *testing.T) []*golden.HclBlock {
	t.Helper()
	content := []byte(`
data "provider_schema" "azurerm" {
  provider_source  = "hashicorp/azurerm"
  provider_version = "~> 4.0"
}

data "provider_schema" "azapi" {
  provider_source  = "Azure/azapi"
  provider_version = "~> 2.4"
}

data "provider_schema" "random" {
  provider_source  = "hashicorp/random"
  provider_version = "~> 3.0"
}
`)
	readFile, diagnostics := hclsyntax.ParseConfig(content, "provider_schema_test.mptf.hcl", hcl.InitialPos)
	require.False(t, diagnostics.HasErrors(), diagnostics.Error())
	writeFile, diagnostics := hclwrite.ParseConfig(content, "provider_schema_test.mptf.hcl", hcl.InitialPos)
	require.False(t, diagnostics.HasErrors(), diagnostics.Error())
	return golden.AsHclBlocks(readFile.Body.(*hclsyntax.Body).Blocks, writeFile.Body().Blocks())
}
