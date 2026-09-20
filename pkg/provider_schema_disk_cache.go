package pkg

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"

	tfjson "github.com/hashicorp/terraform-json"
)

const (
	// providerSchemaDiskCacheFormatVersion guards the on-disk entry layout.
	// Bump it whenever the envelope or the encoded schema shape changes so
	// stale entries written by an older mapotf are discarded rather than
	// decoded into a mismatched struct.
	providerSchemaDiskCacheFormatVersion = 1

	// providerSchemaDiskCacheDefaultTTL bounds how long a cached schema is
	// reused. A version constraint such as `~> 4.0` floats, so an entry that
	// never expired would pin ordering to whichever provider release happened
	// to be current when the cache was first populated.
	providerSchemaDiskCacheDefaultTTL = 7 * 24 * time.Hour

	envProviderSchemaCache    = "MAPOTF_PROVIDER_SCHEMA_CACHE"
	envProviderSchemaCacheDir = "MAPOTF_PROVIDER_SCHEMA_CACHE_DIR"
	envProviderSchemaCacheTTL = "MAPOTF_PROVIDER_SCHEMA_CACHE_TTL"
)

// providerSchemaDiskCache persists `terraform providers schema` results
// between mapotf processes.
//
// The in-memory cache in providerSchemaRetrieverState only survives a single
// invocation, so a caller that runs mapotf once per module pays the full
// `terraform init` + schema decode cost for every module even though the
// provider schemas are identical. This cache moves that cost to the first
// invocation only.
//
// Every operation is best-effort: any I/O or decode failure degrades to a
// cache miss and the normal Terraform path, because a broken cache must never
// fail a transform.
type providerSchemaDiskCache struct {
	dir string
	ttl time.Duration
	now func() time.Time
}

type providerSchemaDiskCacheEntry struct {
	FormatVersion     int                    `json:"format_version"`
	ProviderSource    string                 `json:"provider_source"`
	VersionConstraint string                 `json:"version_constraint"`
	CachedAt          time.Time              `json:"cached_at"`
	Schema            *tfjson.ProviderSchema `json:"schema"`
}

func newProviderSchemaDiskCacheFromEnv() *providerSchemaDiskCache {
	if !providerSchemaDiskCacheEnabled(os.Getenv(envProviderSchemaCache)) {
		return nil
	}

	dir := strings.TrimSpace(os.Getenv(envProviderSchemaCacheDir))
	if dir == "" {
		userCacheDir, err := os.UserCacheDir()
		if err != nil {
			return nil
		}
		dir = filepath.Join(userCacheDir, "mapotf", "provider-schema")
	}

	return &providerSchemaDiskCache{
		dir: dir,
		ttl: providerSchemaDiskCacheTTLFromEnv(os.Getenv(envProviderSchemaCacheTTL)),
	}
}

func providerSchemaDiskCacheEnabled(raw string) bool {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "0", "off", "false", "no", "disabled":
		return false
	default:
		return true
	}
}

// providerSchemaDiskCacheTTLFromEnv parses the TTL override. A non-positive
// duration disables expiry so a pipeline that pins exact provider versions can
// keep entries indefinitely; anything unparsable falls back to the default.
func providerSchemaDiskCacheTTLFromEnv(raw string) time.Duration {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return providerSchemaDiskCacheDefaultTTL
	}
	ttl, err := time.ParseDuration(trimmed)
	if err != nil {
		return providerSchemaDiskCacheDefaultTTL
	}
	if ttl <= 0 {
		return 0
	}
	return ttl
}

func (c *providerSchemaDiskCache) clock() time.Time {
	if c.now != nil {
		return c.now()
	}
	return time.Now()
}

func (c *providerSchemaDiskCache) entryPath(key providerSchemaCacheKey) string {
	sum := sha256.Sum256([]byte(key.providerSource + "\x00" + key.versionConstraint))
	return filepath.Join(c.dir, hex.EncodeToString(sum[:])+".json")
}

func (c *providerSchemaDiskCache) get(key providerSchemaCacheKey) (*tfjson.ProviderSchema, bool) {
	if c == nil {
		return nil, false
	}

	path := c.entryPath(key)
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, false
	}

	var entry providerSchemaDiskCacheEntry
	if err := json.Unmarshal(content, &entry); err != nil {
		c.discard(path)
		return nil, false
	}
	if entry.FormatVersion != providerSchemaDiskCacheFormatVersion || entry.Schema == nil {
		c.discard(path)
		return nil, false
	}
	// Guard against a hash collision or a hand-edited file resolving to the
	// wrong provider.
	if entry.ProviderSource != key.providerSource || entry.VersionConstraint != key.versionConstraint {
		return nil, false
	}
	if c.expired(entry.CachedAt) {
		c.discard(path)
		return nil, false
	}
	return entry.Schema, true
}

func (c *providerSchemaDiskCache) put(key providerSchemaCacheKey, schema *tfjson.ProviderSchema) {
	if c == nil || schema == nil {
		return
	}
	if err := os.MkdirAll(c.dir, 0o755); err != nil {
		return
	}

	content, err := json.Marshal(providerSchemaDiskCacheEntry{
		FormatVersion:     providerSchemaDiskCacheFormatVersion,
		ProviderSource:    key.providerSource,
		VersionConstraint: key.versionConstraint,
		CachedAt:          c.clock(),
		Schema:            schema,
	})
	if err != nil {
		return
	}

	// Write to a sibling temp file and rename so a concurrent mapotf process
	// never observes a partially written entry.
	tmp, err := os.CreateTemp(c.dir, "schema-*.tmp")
	if err != nil {
		return
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(content); err != nil {
		_ = tmp.Close()
		c.discard(tmpName)
		return
	}
	if err := tmp.Close(); err != nil {
		c.discard(tmpName)
		return
	}
	if err := os.Rename(tmpName, c.entryPath(key)); err != nil {
		c.discard(tmpName)
	}
}

func (c *providerSchemaDiskCache) expired(cachedAt time.Time) bool {
	if c.ttl <= 0 {
		return false
	}
	if cachedAt.IsZero() {
		return true
	}
	return c.clock().Sub(cachedAt) > c.ttl
}

func (c *providerSchemaDiskCache) discard(path string) {
	_ = os.Remove(path)
}
