package pkg

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	tfjson "github.com/hashicorp/terraform-json"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestProviderSchemaDiskCache(t *testing.T) *providerSchemaDiskCache {
	t.Helper()
	return &providerSchemaDiskCache{
		dir: t.TempDir(),
		ttl: providerSchemaDiskCacheDefaultTTL,
	}
}

func testProviderSchemaCacheKey() providerSchemaCacheKey {
	return providerSchemaRequest{
		providerSource:    "Azure/azapi",
		versionConstraint: "~> 2.12",
	}.key()
}

// TestProviderSchemaDiskCache_RoundTrip pins the core contract: a schema
// written by one mapotf process is readable by the next.
func TestProviderSchemaDiskCache_RoundTrip(t *testing.T) {
	cache := newTestProviderSchemaDiskCache(t)
	key := testProviderSchemaCacheKey()

	_, ok := cache.get(key)
	require.False(t, ok, "expected a miss before anything is written")

	cache.put(key, providerSchemaFixture())

	schema, ok := cache.get(key)
	require.True(t, ok)
	require.NotNil(t, schema)
	require.Contains(t, schema.ResourceSchemas, "azapi_resource")
}

// TestProviderSchemaDiskCache_SurvivesNewCacheInstance simulates a second
// mapotf process pointed at the same directory.
func TestProviderSchemaDiskCache_SurvivesNewCacheInstance(t *testing.T) {
	dir := t.TempDir()
	key := testProviderSchemaCacheKey()

	first := &providerSchemaDiskCache{dir: dir, ttl: providerSchemaDiskCacheDefaultTTL}
	first.put(key, providerSchemaFixture())

	second := &providerSchemaDiskCache{dir: dir, ttl: providerSchemaDiskCacheDefaultTTL}
	schema, ok := second.get(key)
	require.True(t, ok)
	require.NotNil(t, schema)
}

func TestProviderSchemaDiskCache_DistinctKeysDoNotCollide(t *testing.T) {
	cache := newTestProviderSchemaDiskCache(t)
	azapi := providerSchemaRequest{providerSource: "Azure/azapi", versionConstraint: "~> 2.12"}.key()
	random := providerSchemaRequest{providerSource: "hashicorp/random", versionConstraint: "~> 3.0"}.key()

	cache.put(azapi, providerSchemaFixture())

	_, ok := cache.get(random)
	assert.False(t, ok, "a different provider must not read another provider's entry")

	// A changed version constraint is a different cache key.
	bumped := providerSchemaRequest{providerSource: "Azure/azapi", versionConstraint: "~> 3.0"}.key()
	_, ok = cache.get(bumped)
	assert.False(t, ok, "a changed version constraint must miss")
}

// TestProviderSchemaDiskCache_EquivalentSourcesShareEntry pins that source
// casing and the implicit registry host normalise to one entry, matching the
// in-memory dedupe behaviour.
func TestProviderSchemaDiskCache_EquivalentSourcesShareEntry(t *testing.T) {
	cache := newTestProviderSchemaDiskCache(t)
	cache.put(providerSchemaRequest{providerSource: "Azure/azapi", versionConstraint: "~> 2.12"}.key(), providerSchemaFixture())

	for _, source := range []string{"azure/azapi", "registry.terraform.io/azure/azapi", "Azure/azapi"} {
		_, ok := cache.get(providerSchemaRequest{providerSource: source, versionConstraint: "~> 2.12"}.key())
		assert.Truef(t, ok, "expected %q to resolve to the same cache entry", source)
	}
}

func TestProviderSchemaDiskCache_ExpiredEntryIsDiscarded(t *testing.T) {
	cache := newTestProviderSchemaDiskCache(t)
	cache.ttl = time.Hour
	key := testProviderSchemaCacheKey()

	cache.put(key, providerSchemaFixture())
	cache.now = func() time.Time { return time.Now().Add(2 * time.Hour) }

	_, ok := cache.get(key)
	assert.False(t, ok, "entry older than the TTL must miss")
	assert.NoFileExists(t, cache.entryPath(key), "an expired entry should be removed")
}

func TestProviderSchemaDiskCache_ZeroTTLNeverExpires(t *testing.T) {
	cache := newTestProviderSchemaDiskCache(t)
	cache.ttl = 0
	key := testProviderSchemaCacheKey()

	cache.put(key, providerSchemaFixture())
	cache.now = func() time.Time { return time.Now().Add(10000 * time.Hour) }

	_, ok := cache.get(key)
	assert.True(t, ok, "a non-positive TTL disables expiry")
}

func TestProviderSchemaDiskCache_CorruptEntryIsDiscarded(t *testing.T) {
	cache := newTestProviderSchemaDiskCache(t)
	key := testProviderSchemaCacheKey()

	require.NoError(t, os.MkdirAll(cache.dir, 0o755))
	require.NoError(t, os.WriteFile(cache.entryPath(key), []byte("{not json"), 0o600))

	_, ok := cache.get(key)
	assert.False(t, ok)
	assert.NoFileExists(t, cache.entryPath(key), "a corrupt entry should be removed so it is rebuilt")
}

func TestProviderSchemaDiskCache_StaleFormatVersionIsDiscarded(t *testing.T) {
	cache := newTestProviderSchemaDiskCache(t)
	key := testProviderSchemaCacheKey()

	content, err := json.Marshal(providerSchemaDiskCacheEntry{
		FormatVersion:     providerSchemaDiskCacheFormatVersion + 1,
		ProviderSource:    key.providerSource,
		VersionConstraint: key.versionConstraint,
		CachedAt:          time.Now(),
		Schema:            providerSchemaFixture(),
	})
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(cache.dir, 0o755))
	require.NoError(t, os.WriteFile(cache.entryPath(key), content, 0o600))

	_, ok := cache.get(key)
	assert.False(t, ok, "an entry written by a different cache format must be ignored")
}

// TestProviderSchemaDiskCache_MismatchedEntryIsIgnored covers the collision
// guard: a file whose recorded identity disagrees with the key must not be
// served, and must not be deleted either since it belongs to another key.
func TestProviderSchemaDiskCache_MismatchedEntryIsIgnored(t *testing.T) {
	cache := newTestProviderSchemaDiskCache(t)
	key := testProviderSchemaCacheKey()

	content, err := json.Marshal(providerSchemaDiskCacheEntry{
		FormatVersion:     providerSchemaDiskCacheFormatVersion,
		ProviderSource:    "registry.terraform.io/hashicorp/random",
		VersionConstraint: "~> 3.0",
		CachedAt:          time.Now(),
		Schema:            providerSchemaFixture(),
	})
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(cache.dir, 0o755))
	require.NoError(t, os.WriteFile(cache.entryPath(key), content, 0o600))

	_, ok := cache.get(key)
	assert.False(t, ok)
	assert.FileExists(t, cache.entryPath(key))
}

func TestProviderSchemaDiskCache_NilReceiverIsSafe(t *testing.T) {
	var cache *providerSchemaDiskCache
	key := testProviderSchemaCacheKey()

	require.NotPanics(t, func() {
		cache.put(key, providerSchemaFixture())
		_, ok := cache.get(key)
		assert.False(t, ok)
	})
}

func TestProviderSchemaDiskCache_PutIgnoresNilSchema(t *testing.T) {
	cache := newTestProviderSchemaDiskCache(t)
	key := testProviderSchemaCacheKey()

	cache.put(key, nil)

	_, ok := cache.get(key)
	assert.False(t, ok)
}

// TestProviderSchemaDiskCache_UnwritableDirIsNotFatal pins the best-effort
// contract: a cache that cannot be written must degrade to a miss, never error.
func TestProviderSchemaDiskCache_UnwritableDirIsNotFatal(t *testing.T) {
	file := filepath.Join(t.TempDir(), "not-a-dir")
	require.NoError(t, os.WriteFile(file, []byte("x"), 0o600))

	cache := &providerSchemaDiskCache{dir: filepath.Join(file, "cache"), ttl: time.Hour}
	key := testProviderSchemaCacheKey()

	require.NotPanics(t, func() { cache.put(key, providerSchemaFixture()) })
	_, ok := cache.get(key)
	assert.False(t, ok)
}

func TestProviderSchemaDiskCacheEnabled(t *testing.T) {
	for _, raw := range []string{"0", "off", "false", "no", "disabled", "OFF", " off "} {
		assert.Falsef(t, providerSchemaDiskCacheEnabled(raw), "expected %q to disable the cache", raw)
	}
	for _, raw := range []string{"", "1", "on", "true", "anything"} {
		assert.Truef(t, providerSchemaDiskCacheEnabled(raw), "expected %q to leave the cache enabled", raw)
	}
}

func TestProviderSchemaDiskCacheTTLFromEnv(t *testing.T) {
	assert.Equal(t, providerSchemaDiskCacheDefaultTTL, providerSchemaDiskCacheTTLFromEnv(""))
	assert.Equal(t, providerSchemaDiskCacheDefaultTTL, providerSchemaDiskCacheTTLFromEnv("not-a-duration"))
	assert.Equal(t, 90*time.Minute, providerSchemaDiskCacheTTLFromEnv("90m"))
	assert.Equal(t, time.Duration(0), providerSchemaDiskCacheTTLFromEnv("0"))
	assert.Equal(t, time.Duration(0), providerSchemaDiskCacheTTLFromEnv("-5m"))
}

func TestNewProviderSchemaDiskCacheFromEnv(t *testing.T) {
	t.Run("disabled returns nil", func(t *testing.T) {
		t.Setenv(envProviderSchemaCache, "off")
		assert.Nil(t, newProviderSchemaDiskCacheFromEnv())
	})

	t.Run("honours explicit dir and ttl", func(t *testing.T) {
		dir := t.TempDir()
		t.Setenv(envProviderSchemaCache, "")
		t.Setenv(envProviderSchemaCacheDir, dir)
		t.Setenv(envProviderSchemaCacheTTL, "30m")

		cache := newProviderSchemaDiskCacheFromEnv()
		require.NotNil(t, cache)
		assert.Equal(t, dir, cache.dir)
		assert.Equal(t, 30*time.Minute, cache.ttl)
	})

	t.Run("defaults to the user cache dir", func(t *testing.T) {
		t.Setenv(envProviderSchemaCache, "")
		t.Setenv(envProviderSchemaCacheDir, "")
		t.Setenv(envProviderSchemaCacheTTL, "")

		cache := newProviderSchemaDiskCacheFromEnv()
		if cache == nil {
			t.Skip("no user cache dir available on this platform")
		}
		assert.Contains(t, cache.dir, "mapotf")
		assert.Equal(t, providerSchemaDiskCacheDefaultTTL, cache.ttl)
	})
}

// TestProviderSchemaRetriever_DiskCacheAvoidsSecondTerraformRun is the
// behaviour this cache exists for: a second mapotf invocation against the same
// providers must not shell out to Terraform again.
func TestProviderSchemaRetriever_DiskCacheAvoidsSecondTerraformRun(t *testing.T) {
	dir := t.TempDir()
	requests := avmProviderSchemaRequests()

	firstRunner := &recordingProviderSchemaRunner{}
	first := newTestProviderSchemaRetrieverWithDiskCache(t, firstRunner, dir)
	require.NoError(t, first.prefetch(requests))
	require.Len(t, firstRunner.calls, 1, "the cold run must invoke Terraform once")

	secondRunner := &recordingProviderSchemaRunner{}
	second := newTestProviderSchemaRetrieverWithDiskCache(t, secondRunner, dir)
	require.NoError(t, second.prefetch(requests))
	assert.Empty(t, secondRunner.calls, "the warm run must be served entirely from disk")

	for _, request := range requests {
		schema, err := second.Get(request.providerSource, request.versionConstraint)
		require.NoError(t, err)
		require.NotNil(t, schema)
	}
}

// TestProviderSchemaRetriever_DiskCacheDoesNotPersistFailures ensures a failed
// lookup is never replayed from disk on a later run.
func TestProviderSchemaRetriever_DiskCacheDoesNotPersistFailures(t *testing.T) {
	dir := t.TempDir()
	request := providerSchemaRequest{providerSource: "hashicorp/random", versionConstraint: "~> 3.0"}

	failing := &recordingProviderSchemaRunner{
		run: func(context.Context, []providerSchemaRequest) (*tfjson.ProviderSchemas, error) {
			return nil, errors.New("registry unavailable")
		},
	}
	first := newTestProviderSchemaRetrieverWithDiskCache(t, failing, dir)
	_, err := first.Get(request.providerSource, request.versionConstraint)
	require.Error(t, err)

	healthy := &recordingProviderSchemaRunner{}
	second := newTestProviderSchemaRetrieverWithDiskCache(t, healthy, dir)
	schema, err := second.Get(request.providerSource, request.versionConstraint)
	require.NoError(t, err, "a previous failure must not be cached to disk")
	require.NotNil(t, schema)
	require.Len(t, healthy.calls, 1)
}

func newTestProviderSchemaRetrieverWithDiskCache(t *testing.T, runner providerSchemaRunner, dir string) TerraformCliProviderSchemaRetriever {
	t.Helper()
	retriever := newTestProviderSchemaRetriever(context.Background(), runner)
	retriever.state.diskCache = &providerSchemaDiskCache{dir: dir, ttl: providerSchemaDiskCacheDefaultTTL}
	return retriever
}

func providerSchemaFixture() *tfjson.ProviderSchema {
	return &tfjson.ProviderSchema{
		ResourceSchemas: map[string]*tfjson.Schema{
			"azapi_resource": {
				Block: &tfjson.SchemaBlock{
					Attributes: map[string]*tfjson.SchemaAttribute{
						"name":      {Required: true},
						"parent_id": {Optional: true},
					},
				},
			},
		},
		DataSourceSchemas: map[string]*tfjson.Schema{},
	}
}
