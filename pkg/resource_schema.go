package pkg

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"

	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/hashicorp/terraform-exec/tfexec"
	tfjson "github.com/hashicorp/terraform-json"
	"github.com/zclconf/go-cty/cty"
)

type TerraformProviderSchemaRetriever interface {
	Get(providerSource, versionConstraint string) (*tfjson.ProviderSchema, error)
}

type providerSchemaPrefetcher interface {
	prefetch(requests []providerSchemaRequest) error
}

type TerraformCliProviderSchemaRetriever struct {
	ctx   context.Context
	state *providerSchemaRetrieverState
}

func NewTerraformCliProviderSchemaRetriever(ctx context.Context) TerraformProviderSchemaRetriever {
	if ctx == nil {
		ctx = context.Background()
	}
	return TerraformCliProviderSchemaRetriever{
		ctx:   ctx,
		state: newProviderSchemaRetrieverState(),
	}
}

func (t TerraformCliProviderSchemaRetriever) Get(providerSource, versionConstraint string) (*tfjson.ProviderSchema, error) {
	request := providerSchemaRequest{
		providerSource:    providerSource,
		versionConstraint: versionConstraint,
	}
	state := t.stateOrNew()
	if err := t.prefetchWithState(state, []providerSchemaRequest{request}); err != nil {
		return nil, err
	}

	state.mu.Lock()
	defer state.mu.Unlock()
	result, ok := state.cache[request.key()]
	if !ok {
		return nil, fmt.Errorf("provider schema retrieval completed without a result for source %q version %q", providerSource, versionConstraint)
	}
	return result.schema, result.err
}

type providerSchemaRequest struct {
	providerSource    string
	versionConstraint string
}

func (r providerSchemaRequest) key() providerSchemaCacheKey {
	return providerSchemaCacheKey{
		providerSource:    canonicalProviderSource(r.providerSource),
		versionConstraint: r.versionConstraint,
	}
}

type providerSchemaCacheKey struct {
	providerSource    string
	versionConstraint string
}

type providerSchemaResult struct {
	schema *tfjson.ProviderSchema
	err    error
}

type providerSchemaRetrieverState struct {
	mu            sync.Mutex
	cache         map[providerSchemaCacheKey]providerSchemaResult
	runnerFactory func(TerraformCliProviderSchemaRetriever) (providerSchemaRunner, error)
}

func newProviderSchemaRetrieverState() *providerSchemaRetrieverState {
	return &providerSchemaRetrieverState{
		cache: make(map[providerSchemaCacheKey]providerSchemaResult),
		runnerFactory: func(t TerraformCliProviderSchemaRetriever) (providerSchemaRunner, error) {
			execPath, err := t.getTerraformPath()
			if err != nil {
				return nil, err
			}
			return terraformExecProviderSchemaRunner{execPath: execPath}, nil
		},
	}
}

func (t TerraformCliProviderSchemaRetriever) stateOrNew() *providerSchemaRetrieverState {
	if t.state != nil {
		return t.state
	}
	return newProviderSchemaRetrieverState()
}

func (t TerraformCliProviderSchemaRetriever) prefetch(requests []providerSchemaRequest) error {
	return t.prefetchWithState(t.stateOrNew(), requests)
}

func (t TerraformCliProviderSchemaRetriever) prefetchWithState(state *providerSchemaRetrieverState, requests []providerSchemaRequest) error {
	state.mu.Lock()
	defer state.mu.Unlock()

	missing := deduplicateProviderSchemaRequests(requests, state.cache)
	if len(missing) == 0 {
		return nil
	}
	if ctxErr := t.context().Err(); ctxErr != nil {
		return ctxErr
	}

	runner, err := state.runnerFactory(t)
	if err != nil {
		if ctxErr := t.context().Err(); ctxErr != nil {
			return ctxErr
		}
		cacheProviderSchemaError(state.cache, missing, err)
		return nil
	}

	for _, batch := range partitionProviderSchemaRequests(missing) {
		if err := t.loadProviderSchemaBatch(state.cache, runner, batch); err != nil {
			return err
		}
	}
	return nil
}

func deduplicateProviderSchemaRequests(requests []providerSchemaRequest, cache map[providerSchemaCacheKey]providerSchemaResult) []providerSchemaRequest {
	unique := make(map[providerSchemaCacheKey]providerSchemaRequest)
	for _, request := range requests {
		key := request.key()
		if _, ok := cache[key]; ok {
			continue
		}
		if _, ok := unique[key]; ok {
			continue
		}
		unique[key] = request
	}

	out := make([]providerSchemaRequest, 0, len(unique))
	for _, request := range unique {
		out = append(out, request)
	}
	sort.Slice(out, func(i, j int) bool {
		left, right := out[i].key(), out[j].key()
		if left.providerSource != right.providerSource {
			return left.providerSource < right.providerSource
		}
		return left.versionConstraint < right.versionConstraint
	})
	return out
}

func partitionProviderSchemaRequests(requests []providerSchemaRequest) [][]providerSchemaRequest {
	var batches [][]providerSchemaRequest
	for _, request := range requests {
		source := request.key().providerSource
		placed := false
		for i := range batches {
			conflict := false
			for _, existing := range batches[i] {
				if existing.key().providerSource == source {
					conflict = true
					break
				}
			}
			if conflict {
				continue
			}
			batches[i] = append(batches[i], request)
			placed = true
			break
		}
		if !placed {
			batches = append(batches, []providerSchemaRequest{request})
		}
	}
	return batches
}

func (t TerraformCliProviderSchemaRetriever) loadProviderSchemaBatch(cache map[providerSchemaCacheKey]providerSchemaResult, runner providerSchemaRunner, requests []providerSchemaRequest) error {
	schemas, err := t.retrieveProviderSchemas(runner, requests)
	if err == nil {
		cacheProviderSchemas(cache, requests, schemas.Schemas)
		return nil
	}
	if ctxErr := contextError(t.context(), err); ctxErr != nil {
		return ctxErr
	}

	var executionErr *providerSchemaExecutionError
	if len(requests) == 1 || !errors.As(err, &executionErr) {
		cacheProviderSchemaError(cache, requests, err)
		return nil
	}

	for _, request := range requests {
		schemas, err := t.retrieveProviderSchemas(runner, []providerSchemaRequest{request})
		if err != nil {
			if ctxErr := contextError(t.context(), err); ctxErr != nil {
				return ctxErr
			}
			cache[request.key()] = providerSchemaResult{err: err}
			continue
		}
		cacheProviderSchemas(cache, []providerSchemaRequest{request}, schemas.Schemas)
	}
	return nil
}

func (t TerraformCliProviderSchemaRetriever) retrieveProviderSchemas(runner providerSchemaRunner, requests []providerSchemaRequest) (*tfjson.ProviderSchemas, error) {
	tmpFolder, err := os.MkdirTemp("", "mapotf-provider-schema-*")
	if err != nil {
		return nil, fmt.Errorf("error creating temp TF code folder: %s", err)
	}
	defer func() {
		_ = os.RemoveAll(tmpFolder)
	}()

	if err := os.WriteFile(filepath.Join(tmpFolder, "main.tf"), terraformProviderConfig(requests), 0600); err != nil {
		return nil, fmt.Errorf("error writing temp TF code file: %s", err)
	}

	schema, err := runner.ProviderSchemas(t.context(), tmpFolder)
	if err != nil {
		return nil, &providerSchemaExecutionError{err: err}
	}
	if schema == nil {
		return nil, &providerSchemaExecutionError{err: errors.New("terraform providers schema returned no result")}
	}
	return schema, nil
}

func terraformProviderConfig(requests []providerSchemaRequest) []byte {
	file := hclwrite.NewEmptyFile()
	terraformBlock := hclwrite.NewBlock("terraform", nil)
	requiredProvidersBlock := hclwrite.NewBlock("required_providers", nil)
	for i, request := range requests {
		requiredProvidersBlock.Body().SetAttributeValue(fmt.Sprintf("provider%d", i), cty.ObjectVal(map[string]cty.Value{
			"source":  cty.StringVal(request.providerSource),
			"version": cty.StringVal(request.versionConstraint),
		}))
	}
	terraformBlock.Body().AppendBlock(requiredProvidersBlock)
	file.Body().AppendBlock(terraformBlock)
	return file.Bytes()
}

func cacheProviderSchemas(cache map[providerSchemaCacheKey]providerSchemaResult, requests []providerSchemaRequest, schemas map[string]*tfjson.ProviderSchema) {
	for _, request := range requests {
		schema, err := lookupProviderSchema(schemas, request.providerSource, request.versionConstraint)
		cache[request.key()] = providerSchemaResult{
			schema: schema,
			err:    err,
		}
	}
}

func cacheProviderSchemaError(cache map[providerSchemaCacheKey]providerSchemaResult, requests []providerSchemaRequest, err error) {
	for _, request := range requests {
		cache[request.key()] = providerSchemaResult{err: err}
	}
}

func contextError(ctx context.Context, err error) error {
	if ctxErr := ctx.Err(); ctxErr != nil {
		return ctxErr
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return err
	}
	return nil
}

func (t TerraformCliProviderSchemaRetriever) context() context.Context {
	if t.ctx == nil {
		return context.Background()
	}
	return t.ctx
}

type providerSchemaExecutionError struct {
	err error
}

func (e *providerSchemaExecutionError) Error() string {
	return e.err.Error()
}

func (e *providerSchemaExecutionError) Unwrap() error {
	return e.err
}

type providerSchemaRunner interface {
	ProviderSchemas(ctx context.Context, workingDir string) (*tfjson.ProviderSchemas, error)
}

type terraformExecProviderSchemaRunner struct {
	execPath string
}

func (r terraformExecProviderSchemaRunner) ProviderSchemas(ctx context.Context, workingDir string) (*tfjson.ProviderSchemas, error) {
	tf, err := tfexec.NewTerraform(workingDir, r.execPath)
	if err != nil {
		return nil, fmt.Errorf("error running NewTerraform: %w", err)
	}
	if err := tf.Init(ctx, tfexec.Upgrade(true)); err != nil {
		return nil, fmt.Errorf("error running Init: %w", err)
	}
	schema, err := tf.ProvidersSchema(ctx)
	if err != nil {
		return nil, fmt.Errorf("error running providers: %w", err)
	}
	return schema, nil
}

// lookupProviderSchema resolves a provider schema by source within the map
// produced by `terraform providers schema -json`. Terraform normalises
// provider source namespaces to lowercase in that output (registry
// namespaces are case-insensitive identifiers per the registry protocol),
// so the lookup lowercases `providerSource` before searching. On miss it
// returns a real error rather than `(nil, nil)` so config-layer callers
// surface the failure instead of dereferencing nil downstream.
func lookupProviderSchema(schemas map[string]*tfjson.ProviderSchema, providerSource, versionConstraint string) (*tfjson.ProviderSchema, error) {
	lowered := strings.ToLower(providerSource)
	src := canonicalProviderSource(providerSource)
	if r, ok := schemas[src]; ok && r != nil {
		return r, nil
	}
	// Fall back to a direct lookup on the lowercased source for providers
	// whose schema key already includes a non-default hostname or is
	// otherwise not prefixed with `registry.terraform.io/`.
	if r, ok := schemas[lowered]; ok && r != nil {
		return r, nil
	}
	return nil, fmt.Errorf("provider schema %q not found; ensure `terraform init` succeeds for source %q version %q", src, providerSource, versionConstraint)
}

func canonicalProviderSource(providerSource string) string {
	lowered := strings.ToLower(providerSource)
	if strings.Count(lowered, "/") == 1 {
		return fmt.Sprintf("registry.terraform.io/%s", lowered)
	}
	return lowered
}

func (t TerraformCliProviderSchemaRetriever) getTerraformPath() (string, error) {
	var cmd *exec.Cmd

	if t.isWindows() {
		cmd = exec.Command("where", "terraform")
	} else {
		cmd = exec.Command("which", "terraform")
	}

	out, err := cmd.Output()
	if err != nil {
		return "", err
	}

	path := strings.TrimSpace(string(out))
	return path, nil
}

func (t TerraformCliProviderSchemaRetriever) isWindows() bool {
	return runtime.GOOS == "windows"
}
