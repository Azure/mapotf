package pkg

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime/debug"
	"strings"
	"time"

	"github.com/gofrs/flock"
	"github.com/hashicorp/terraform-config-inspect/tfconfig"
)

const (
	moduleSourceCacheFormatVersion = 1
	moduleSourceCacheDefaultTTL    = 7 * 24 * time.Hour
	moduleSourceCacheLockTimeout   = 2 * time.Minute
	moduleSourceCacheMaxEntrySize  = 16 << 20

	envModuleSourceCache            = "MAPOTF_MODULE_SOURCE_CACHE"
	envModuleSourceCacheDir         = "MAPOTF_MODULE_SOURCE_CACHE_DIR"
	envModuleSourceCacheTTL         = "MAPOTF_MODULE_SOURCE_CACHE_TTL"
	envModuleSourceCacheLockTimeout = "MAPOTF_MODULE_SOURCE_CACHE_LOCK_TIMEOUT"
)

var (
	publicModuleSourcePattern  = regexp.MustCompile(`^(registry\.terraform\.io/)?[A-Za-z0-9][A-Za-z0-9_-]*/[A-Za-z0-9][A-Za-z0-9_-]*/[a-z][a-z0-9]*$`)
	exactModuleVersionPattern  = regexp.MustCompile(`^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$`)
	moduleSourceInspector      = moduleSourceInspectorVersion()
	moduleSourceCLIConfigPaths = defaultModuleSourceCLIConfigPaths
)

type moduleSourceCacheKey struct {
	source  string
	version string
}

// Only data blocks use this adapter; the exported fetcher still returns a full module.
type moduleSourceMetadataFetcher struct {
	TerraformCliModuleSourceFetcher
}

func (t moduleSourceMetadataFetcher) Get(source, version, baseDir string) (*tfconfig.Module, error) {
	key, eligible := moduleSourceCacheIdentity(source, version)
	if !eligible {
		return t.TerraformCliModuleSourceFetcher.Get(source, version, baseDir)
	}
	cache := newModuleSourceDiskCacheFromEnv()
	return cache.getOrFetch(t.ctx, key, func() (*tfconfig.Module, bool, error) {
		return t.fetchRemoteModule(source, version)
	})
}

func moduleSourceCacheIdentity(source, version string) (moduleSourceCacheKey, bool) {
	if !publicModuleSourcePattern.MatchString(source) || !exactModuleVersionPattern.MatchString(version) {
		return moduleSourceCacheKey{}, false
	}
	return moduleSourceCacheKey{
		source:  "registry.terraform.io/" + strings.TrimPrefix(source, "registry.terraform.io/"),
		version: version,
	}, true
}

func moduleSourceSelectionMatches(source, version string, selected TerraformModuleRef) bool {
	requested, eligible := moduleSourceCacheIdentity(source, version)
	actual, selectedEligible := moduleSourceCacheIdentity(selected.Source, selected.Version)
	return eligible && selectedEligible && requested == actual
}

type moduleSourceDiskCache struct {
	dir         string
	inspector   string
	ttl         time.Duration
	lockTimeout time.Duration
	refresh     bool
	now         func() time.Time
	warning     func(error)
}

type moduleSourceCacheEntry struct {
	FormatVersion int             `json:"format_version"`
	Source        string          `json:"source"`
	Version       string          `json:"version"`
	Inspector     string          `json:"inspector"`
	CachedAt      time.Time       `json:"cached_at"`
	Metadata      json.RawMessage `json:"metadata"`
	SHA256        string          `json:"sha256"`
}

type moduleSourceMetadata struct {
	Variables map[string]*tfconfig.Variable `json:"variables"`
	Outputs   map[string]*tfconfig.Output   `json:"outputs"`
}

func newModuleSourceDiskCacheFromEnv() *moduleSourceDiskCache {
	mode := strings.ToLower(strings.TrimSpace(os.Getenv(envModuleSourceCache)))
	switch mode {
	case "0", "off", "false", "no", "disabled":
		return nil
	}
	if moduleSourceInspector == "" || !moduleSourceCacheEnvironmentSafe() {
		return nil
	}
	dir := strings.TrimSpace(os.Getenv(envModuleSourceCacheDir))
	if dir == "" {
		userDir, err := os.UserCacheDir()
		if err != nil {
			log.Printf("Warning: module source cache disabled: %v", err)
			return nil
		}
		dir = filepath.Join(userDir, "mapotf", "module-source")
	}
	return &moduleSourceDiskCache{
		dir:         dir,
		inspector:   moduleSourceInspector,
		ttl:         moduleSourceCacheDuration(envModuleSourceCacheTTL, moduleSourceCacheDefaultTTL, false),
		lockTimeout: moduleSourceCacheDuration(envModuleSourceCacheLockTimeout, moduleSourceCacheLockTimeout, true),
		refresh:     mode == "refresh",
	}
}

func moduleSourceCacheDuration(name string, fallback time.Duration, positive bool) time.Duration {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return fallback
	}
	value, err := time.ParseDuration(raw)
	if err != nil || (positive && value <= 0) {
		log.Printf("Warning: invalid %s; using %s", name, fallback)
		return fallback
	}
	return value
}

func moduleSourceInspectorVersion() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return ""
	}
	for _, dep := range info.Deps {
		if dep.Path == "github.com/hashicorp/terraform-config-inspect" {
			if dep.Replace != nil {
				dep = dep.Replace
			}
			if dep.Version != "" && dep.Sum != "" {
				return dep.Path + "@" + dep.Version + ":" + dep.Sum
			}
		}
	}
	return ""
}

// A public-looking address can be redirected or authenticated by Terraform's CLI config.
// Bypass these contexts without reading, persisting or fingerprinting credentials.
func moduleSourceCacheEnvironmentSafe() bool {
	for _, name := range []string{"TF_CLI_CONFIG_FILE", "TERRAFORM_CONFIG", "TF_CLI_ARGS", "TF_CLI_ARGS_get", "TF_DATA_DIR"} {
		if os.Getenv(name) != "" {
			return false
		}
	}
	for _, env := range os.Environ() {
		name, value, _ := strings.Cut(env, "=")
		if strings.HasPrefix(strings.ToUpper(name), "TF_TOKEN_") && value != "" {
			return false
		}
	}
	configFile, configDir, err := moduleSourceCLIConfigPaths()
	if err != nil {
		log.Printf("Warning: module source cache disabled: %v", err)
		return false
	}
	if _, err := os.Stat(configFile); !errors.Is(err, os.ErrNotExist) {
		if err != nil {
			log.Printf("Warning: module source cache disabled: cannot inspect CLI configuration: %v", err)
		}
		return false
	}
	entries, err := os.ReadDir(configDir)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		log.Printf("Warning: module source cache disabled: cannot inspect CLI configuration directory: %v", err)
		return false
	}
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".tfrc") || strings.HasSuffix(entry.Name(), ".tfrc.json") {
			return false
		}
	}
	return true
}

func (c *moduleSourceDiskCache) getOrFetch(ctx context.Context, key moduleSourceCacheKey, fetch func() (*tfconfig.Module, bool, error)) (*tfconfig.Module, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	uncached := func() (*tfconfig.Module, error) {
		mod, _, err := fetch()
		return mod, err
	}
	if c == nil {
		return uncached()
	}
	if err := os.MkdirAll(c.dir, 0o700); err != nil {
		c.warn(fmt.Errorf("cannot create directory; fetching without cache: %w", err))
		return uncached()
	}
	lock := flock.New(c.entryPath(key)+".lock", flock.SetPermissions(0o600))
	defer func() {
		if err := lock.Close(); err != nil {
			c.warn(fmt.Errorf("cannot close population lock: %w", err))
		}
	}()
	lockCtx, cancel := context.WithTimeout(ctx, c.lockTimeout)
	defer cancel()
	locked, err := lock.TryLockContext(lockCtx, 25*time.Millisecond)
	if err != nil || !locked {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if errors.Is(err, context.DeadlineExceeded) || !locked && err == nil {
			return nil, fmt.Errorf("timed out waiting for module source cache population (adjust %s or disable %s): %w", envModuleSourceCacheLockTimeout, envModuleSourceCache, context.DeadlineExceeded)
		}
		c.warn(fmt.Errorf("cannot lock entry; fetching without cache: %w", err))
		return uncached()
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if !c.refresh {
		mod, err := c.read(key)
		if err == nil && mod != nil {
			return mod, ctx.Err()
		}
		if err != nil {
			c.warn(fmt.Errorf("ignoring unusable entry and fetching again: %w", err))
		}
	}
	mod, cacheable, err := fetch()
	if err != nil {
		return mod, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if cacheable {
		if err := c.write(key, mod); err != nil {
			c.warn(fmt.Errorf("cannot store inspection; using fetched metadata: %w", err))
		}
	}
	return mod, nil
}

func (c *moduleSourceDiskCache) clock() time.Time {
	if c.now != nil {
		return c.now()
	}
	return time.Now()
}

func (c *moduleSourceDiskCache) warn(err error) {
	if c.warning != nil {
		c.warning(err)
		return
	}
	log.Printf("Warning: module source cache: %v", err)
}

func (c *moduleSourceDiskCache) entryPath(key moduleSourceCacheKey) string {
	identity := fmt.Sprintf("%d\x00%s\x00%s\x00%s", moduleSourceCacheFormatVersion, c.inspector, key.source, key.version)
	sum := sha256.Sum256([]byte(identity))
	return filepath.Join(c.dir, hex.EncodeToString(sum[:])+".json")
}

func (c *moduleSourceDiskCache) read(key moduleSourceCacheKey) (*tfconfig.Module, error) {
	file, err := os.Open(c.entryPath(key))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	content, readErr := io.ReadAll(io.LimitReader(file, moduleSourceCacheMaxEntrySize+1))
	if err := errors.Join(readErr, file.Close()); err != nil {
		return nil, err
	}
	if len(content) > moduleSourceCacheMaxEntrySize {
		return nil, errors.New("entry exceeds size limit")
	}
	var entry moduleSourceCacheEntry
	if err := decodeModuleSourceCacheJSON(content, &entry); err != nil {
		return nil, err
	}
	if entry.FormatVersion != moduleSourceCacheFormatVersion ||
		entry.Source != key.source || entry.Version != key.version || entry.Inspector != c.inspector {
		return nil, errors.New("entry format or identity does not match")
	}
	now := c.clock()
	if entry.CachedAt.IsZero() || entry.CachedAt.After(now) {
		return nil, errors.New("entry timestamp is invalid")
	}
	if c.ttl > 0 && now.Sub(entry.CachedAt) >= c.ttl {
		return nil, nil
	}
	sum := sha256.Sum256(entry.Metadata)
	if entry.SHA256 != hex.EncodeToString(sum[:]) {
		return nil, errors.New("metadata checksum does not match")
	}
	metadata, err := decodeModuleSourceMetadata(entry.Metadata)
	if err != nil {
		return nil, err
	}
	return &tfconfig.Module{Variables: metadata.Variables, Outputs: metadata.Outputs}, nil
}

func decodeModuleSourceCacheJSON(data []byte, target interface{}) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(new(interface{})); err != io.EOF {
		return errors.New("unexpected trailing cache data")
	}
	return nil
}

func decodeModuleSourceMetadata(data []byte) (*moduleSourceMetadata, error) {
	var metadata moduleSourceMetadata
	if err := decodeModuleSourceCacheJSON(data, &metadata); err != nil {
		return nil, err
	}
	if metadata.Variables == nil || metadata.Outputs == nil {
		return nil, errors.New("missing variables or outputs")
	}
	for name, variable := range metadata.Variables {
		if variable == nil || variable.Name != name || name == "" || (variable.Required && variable.Default != nil) {
			return nil, errors.New("invalid variable metadata")
		}
	}
	for name, output := range metadata.Outputs {
		if output == nil || output.Name != name || name == "" {
			return nil, errors.New("invalid output metadata")
		}
	}
	canonical, err := json.Marshal(&metadata)
	if err != nil {
		return nil, err
	}
	if !bytes.Equal(canonical, data) {
		return nil, errors.New("incomplete or noncanonical metadata")
	}
	return &metadata, nil
}

func (c *moduleSourceDiskCache) write(key moduleSourceCacheKey, mod *tfconfig.Module) error {
	if mod == nil || mod.Diagnostics.HasErrors() {
		return errors.New("inspection is incomplete")
	}
	metadata := &moduleSourceMetadata{Variables: mod.Variables, Outputs: mod.Outputs}
	data, err := json.Marshal(metadata)
	if err != nil {
		return err
	}
	restored, err := decodeModuleSourceMetadata(data)
	if err != nil {
		return err
	}
	// Legacy HCL can return native numeric/collection types that JSON cannot preserve.
	if !reflect.DeepEqual(metadata, restored) {
		return errors.New("metadata does not round-trip without changing default types")
	}
	sum := sha256.Sum256(data)
	content, err := json.Marshal(moduleSourceCacheEntry{
		FormatVersion: moduleSourceCacheFormatVersion,
		Source:        key.source,
		Version:       key.version,
		Inspector:     c.inspector,
		CachedAt:      c.clock(),
		Metadata:      data,
		SHA256:        hex.EncodeToString(sum[:]),
	})
	if err != nil {
		return err
	}
	if len(content) > moduleSourceCacheMaxEntrySize {
		return errors.New("entry exceeds size limit")
	}
	tmp, err := os.CreateTemp(c.dir, "module-*.tmp")
	if err != nil {
		return err
	}
	defer func() {
		if err := os.Remove(tmp.Name()); err != nil && !errors.Is(err, os.ErrNotExist) {
			c.warn(fmt.Errorf("cannot remove temporary entry: %w", err))
		}
	}()
	_, writeErr := tmp.Write(content)
	if writeErr == nil {
		writeErr = tmp.Sync()
	}
	if err := errors.Join(writeErr, tmp.Close()); err != nil {
		return err
	}
	// Readers also hold the per-key OS lock, including on Windows where Rename
	// alone does not guarantee atomic replacement. Never unlink the lock file.
	return os.Rename(tmp.Name(), c.entryPath(key))
}
