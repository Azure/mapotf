package pkg

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gofrs/flock"
	"github.com/hashicorp/terraform-config-inspect/tfconfig"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestModuleSourceCache(t testing.TB) *moduleSourceDiskCache {
	t.Helper()
	return &moduleSourceDiskCache{
		dir:         t.TempDir(),
		inspector:   "fixture-inspector",
		ttl:         time.Hour,
		lockTimeout: 5 * time.Second,
		warning:     func(err error) { t.Log(err) },
	}
}

func moduleSourceFixture(t testing.TB) *tfconfig.Module {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "main.tf"), []byte(`
variable "required" {
  type        = string
  description = "Required input"
  sensitive   = true
}
variable "null" {
  type    = string
  default = null
}
variable "string" { default = "hello" }
variable "boolean" { default = false }
variable "integer" { default = 42 }
variable "fraction" { default = 1.125 }
variable "large" { default = 9007199254740993 }
variable "list" { default = ["a", 2, null, false, { nested = [3.5] }] }
variable "object" { default = { empty = {}, nested = { n = 5, b = true } } }
variable "empty_list" { default = [] }
variable "empty_map" { default = {} }
variable "empty_string" { default = "" }
variable "set" {
  type    = set(string)
  default = ["b", "a"]
}
output "id" {
  description = "Identifier"
  value       = "fixture"
}
output "secret" {
  sensitive = true
  value     = var.required
}
resource "fixture_resource" "untouched" {}
`), 0o600))
	mod, diags := tfconfig.LoadModule(dir)
	require.False(t, diags.HasErrors(), "%s", diags.Error())
	mod.Variables["string"].Deprecated = "Use another input"
	mod.Outputs["id"].Deprecated = "Use another output"
	mod.Outputs["id"].Type = "string"
	return mod
}

func testModuleSourceKey() moduleSourceCacheKey {
	key, _ := moduleSourceCacheIdentity("Azure/naming/azurerm", "0.4.2")
	return key
}

func TestModuleSourceCacheIdentity(t *testing.T) {
	key := testModuleSourceKey()
	for _, source := range []string{"Azure/naming/azurerm", "registry.terraform.io/Azure/naming/azurerm"} {
		got, ok := moduleSourceCacheIdentity(source, "0.4.2")
		require.True(t, ok)
		assert.Equal(t, key, got)
	}
	for _, source := range []string{
		"", ".", "./local", "../parent", `.\local`, "/local", `C:\local`,
		"git::https://example.com/repo.git?ref=v0.4.2", "https://example.com/module.zip",
		"app.terraform.io/org/module/azure", "private.example.com/org/module/azure",
		"https://user:secret@registry.terraform.io/Azure/naming/azurerm",
		"Azure/naming/azurerm//modules/child", " Azure/naming/azurerm",
		"Azure/naming/azurerm/", "Azure/naming/azurerm?token=secret", "Azure/../azurerm",
	} {
		_, ok := moduleSourceCacheIdentity(source, "0.4.2")
		assert.False(t, ok, source)
	}
	for _, version := range []string{
		"", "latest", "~> 0.4", ">= 0.4.2", "0.4", "= 0.4.2", "v0.4.2",
		"0.4.2, < 1.0", "0.4.2-beta.1", "0.4.2+build", "00.4.2", " 0.4.2", "0.4.2\n",
	} {
		_, ok := moduleSourceCacheIdentity("Azure/naming/azurerm", version)
		assert.False(t, ok, version)
	}
	for _, selected := range []TerraformModuleRef{
		{Source: "Azure/naming/azurerm", Version: "0.4.2"},
		{Source: key.source, Version: key.version},
	} {
		assert.True(t, moduleSourceSelectionMatches("Azure/naming/azurerm", "0.4.2", selected))
	}
	for _, selected := range []TerraformModuleRef{
		{}, {Source: key.source}, {Source: key.source, Version: "0.4.3"},
		{Source: "Azure/other/azurerm", Version: key.version},
		{Source: "private.example.com/Azure/naming/azurerm", Version: key.version},
	} {
		assert.False(t, moduleSourceSelectionMatches("Azure/naming/azurerm", "0.4.2", selected))
	}
}

func TestModuleSourceCacheRoundTrip(t *testing.T) {
	cache := newTestModuleSourceCache(t)
	mod := moduleSourceFixture(t)
	calls := 0
	fetch := func() (*tfconfig.Module, bool, error) {
		calls++
		return mod, true, nil
	}
	first, err := cache.getOrFetch(context.Background(), testModuleSourceKey(), fetch)
	require.NoError(t, err)
	require.Same(t, mod, first)
	secondCache := *cache
	second, err := secondCache.getOrFetch(context.Background(), testModuleSourceKey(), fetch)
	require.NoError(t, err)
	assert.Equal(t, 1, calls)
	assert.Equal(t, mod.Variables, second.Variables)
	assert.Equal(t, mod.Outputs, second.Outputs)
	assert.True(t, convertModuleVariables(mod.Variables).RawEquals(convertModuleVariables(second.Variables)))
	assert.True(t, convertModuleOutputs(mod.Outputs).RawEquals(convertModuleOutputs(second.Outputs)))
	required, optional := partitionModuleVariableNames(mod.Variables)
	cachedRequired, cachedOptional := partitionModuleVariableNames(second.Variables)
	assert.True(t, required.RawEquals(cachedRequired))
	assert.True(t, optional.RawEquals(cachedOptional))
	assert.True(t, second.Variables["required"].Required)
	assert.False(t, second.Variables["null"].Required)
	assert.Nil(t, second.Variables["null"].Default)
	assert.IsType(t, float64(0), second.Variables["integer"].Default)

	second.Variables["list"].Default.([]interface{})[0] = "mutated"
	second.Outputs["secret"].Sensitive = false
	third, err := cache.getOrFetch(context.Background(), testModuleSourceKey(), fetch)
	require.NoError(t, err)
	assert.Equal(t, mod.Variables, third.Variables)
	assert.Equal(t, mod.Outputs, third.Outputs)
	assert.Equal(t, 1, calls)
}

func TestModuleSourceCacheDistinctKeys(t *testing.T) {
	cache := newTestModuleSourceCache(t)
	mod := moduleSourceFixture(t)
	calls := 0
	fetch := func() (*tfconfig.Module, bool, error) { calls++; return mod, true, nil }
	for _, key := range []moduleSourceCacheKey{
		testModuleSourceKey(),
		{source: testModuleSourceKey().source, version: "0.4.3"},
		{source: "registry.terraform.io/Azure/other/azurerm", version: "0.4.2"},
	} {
		_, err := cache.getOrFetch(context.Background(), key, fetch)
		require.NoError(t, err)
	}
	assert.Equal(t, 3, calls)
	cache.inspector = "new-parser-version"
	_, err := cache.getOrFetch(context.Background(), testModuleSourceKey(), fetch)
	require.NoError(t, err)
	assert.Equal(t, 4, calls)
}

func TestModuleSourceCacheRefreshAndInvalidation(t *testing.T) {
	cache := newTestModuleSourceCache(t)
	now := time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)
	cache.now = func() time.Time { return now }
	mod := moduleSourceFixture(t)
	calls := 0
	fetch := func() (*tfconfig.Module, bool, error) { calls++; return mod, true, nil }
	get := func(want int) {
		t.Helper()
		_, err := cache.getOrFetch(context.Background(), testModuleSourceKey(), fetch)
		require.NoError(t, err)
		assert.Equal(t, want, calls)
	}
	get(1)
	now = now.Add(time.Minute)
	get(1)
	now = now.Add(time.Hour)
	get(2)
	cache.refresh = true
	get(3)
	cache.refresh = false
	get(3)
	cache.ttl = 0
	now = now.Add(10 * 365 * 24 * time.Hour)
	get(3)
	require.NoError(t, os.Remove(cache.entryPath(testModuleSourceKey())))
	get(4)
	assert.FileExists(t, cache.entryPath(testModuleSourceKey())+".lock")
}

func TestModuleSourceCacheRejectsInvalidEntries(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*moduleSourceCacheEntry)
		raw    []byte
	}{
		{name: "truncated", raw: []byte(`{"format_version":`)},
		{name: "trailing", raw: []byte(`{} {}`)},
		{name: "null", raw: []byte(`null`)},
		{name: "format", mutate: func(e *moduleSourceCacheEntry) { e.FormatVersion++ }},
		{name: "source", mutate: func(e *moduleSourceCacheEntry) { e.Source = "other/module/azure" }},
		{name: "version", mutate: func(e *moduleSourceCacheEntry) { e.Version = "9.9.9" }},
		{name: "parser", mutate: func(e *moduleSourceCacheEntry) { e.Inspector = "other-parser" }},
		{name: "missing_timestamp", mutate: func(e *moduleSourceCacheEntry) { e.CachedAt = time.Time{} }},
		{name: "future_timestamp", mutate: func(e *moduleSourceCacheEntry) { e.CachedAt = time.Now().Add(time.Hour) }},
		{name: "checksum", mutate: func(e *moduleSourceCacheEntry) { e.SHA256 = "incorrect" }},
		{name: "missing_metadata", mutate: func(e *moduleSourceCacheEntry) { e.Metadata = nil }},
		{name: "missing_maps", mutate: func(e *moduleSourceCacheEntry) { replaceTestMetadata(e, `{}`) }},
		{name: "null_maps", mutate: func(e *moduleSourceCacheEntry) { replaceTestMetadata(e, `{"variables":null,"outputs":{}}`) }},
		{name: "null_variable", mutate: func(e *moduleSourceCacheEntry) { replaceTestMetadata(e, `{"variables":{"v":null},"outputs":{}}`) }},
		{name: "null_output", mutate: func(e *moduleSourceCacheEntry) { replaceTestMetadata(e, `{"variables":{},"outputs":{"o":null}}`) }},
		{name: "missing_variable_fields", mutate: func(e *moduleSourceCacheEntry) {
			replaceTestMetadata(e, `{"variables":{"v":{"name":"v"}},"outputs":{}}`)
		}},
		{name: "wrong_variable_name", mutate: func(e *moduleSourceCacheEntry) {
			replaceTestMetadata(e, `{"variables":{"v":{"name":"different"}},"outputs":{}}`)
		}},
		{name: "missing_output_fields", mutate: func(e *moduleSourceCacheEntry) {
			replaceTestMetadata(e, `{"variables":{},"outputs":{"o":{"name":"o"}}}`)
		}},
		{name: "unknown_field", mutate: func(e *moduleSourceCacheEntry) {
			replaceTestMetadata(e, `{"variables":{},"outputs":{},"unexpected":true}`)
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cache := newTestModuleSourceCache(t)
			mod := moduleSourceFixture(t)
			require.NoError(t, cache.write(testModuleSourceKey(), mod))
			content, err := os.ReadFile(cache.entryPath(testModuleSourceKey()))
			require.NoError(t, err)
			var entry moduleSourceCacheEntry
			require.NoError(t, json.Unmarshal(content, &entry))
			if tc.mutate != nil {
				tc.mutate(&entry)
				content, err = json.Marshal(entry)
				require.NoError(t, err)
			} else {
				content = tc.raw
			}
			require.NoError(t, os.WriteFile(cache.entryPath(testModuleSourceKey()), content, 0o600))
			var warnings []error
			cache.warning = func(err error) { warnings = append(warnings, err) }
			calls := 0
			fetch := func() (*tfconfig.Module, bool, error) { calls++; return mod, true, nil }
			for range 2 {
				got, err := cache.getOrFetch(context.Background(), testModuleSourceKey(), fetch)
				require.NoError(t, err)
				assert.Equal(t, mod.Variables, got.Variables)
			}
			assert.Equal(t, 1, calls, "the invalid entry must be replaced, not repeatedly reused or refetched")
			require.Len(t, warnings, 1)
		})
	}
}

func replaceTestMetadata(entry *moduleSourceCacheEntry, data string) {
	entry.Metadata = json.RawMessage(data)
	sum := sha256.Sum256(entry.Metadata)
	entry.SHA256 = hex.EncodeToString(sum[:])
}

func TestModuleSourceCacheDoesNotPersistFailuresOrPartialResults(t *testing.T) {
	cache := newTestModuleSourceCache(t)
	mod := moduleSourceFixture(t)
	failure := errors.New("download failed")
	_, err := cache.getOrFetch(context.Background(), testModuleSourceKey(), func() (*tfconfig.Module, bool, error) {
		return mod, true, failure
	})
	require.ErrorIs(t, err, failure)
	assert.NoFileExists(t, cache.entryPath(testModuleSourceKey()))
	for range 2 {
		got, err := cache.getOrFetch(context.Background(), testModuleSourceKey(), func() (*tfconfig.Module, bool, error) {
			return mod, false, nil
		})
		require.NoError(t, err)
		assert.Same(t, mod, got)
		assert.NoFileExists(t, cache.entryPath(testModuleSourceKey()))
	}
	_, err = cache.getOrFetch(context.Background(), testModuleSourceKey(), func() (*tfconfig.Module, bool, error) {
		return mod, true, nil
	})
	require.NoError(t, err)
	assert.FileExists(t, cache.entryPath(testModuleSourceKey()))
}

func TestModuleSourceCachePreservesUncacheableMetadata(t *testing.T) {
	for _, value := range []interface{}{int(42), int64(9007199254740993), []string{"a"}, map[string]int{"n": 1}} {
		t.Run(fmt.Sprintf("%T", value), func(t *testing.T) {
			cache := newTestModuleSourceCache(t)
			mod := moduleSourceFixture(t)
			mod.Variables["integer"].Default = value
			warnings := 0
			cache.warning = func(error) { warnings++ }
			for range 2 {
				got, err := cache.getOrFetch(context.Background(), testModuleSourceKey(), func() (*tfconfig.Module, bool, error) {
					return mod, true, nil
				})
				require.NoError(t, err)
				assert.Equal(t, value, got.Variables["integer"].Default)
			}
			assert.Equal(t, 2, warnings)
			assert.NoFileExists(t, cache.entryPath(testModuleSourceKey()))
		})
	}
	cache := newTestModuleSourceCache(t)
	for _, mod := range []*tfconfig.Module{
		nil, {}, {Variables: map[string]*tfconfig.Variable{}, Outputs: nil},
		{Variables: map[string]*tfconfig.Variable{"v": nil}, Outputs: map[string]*tfconfig.Output{}},
	} {
		assert.Error(t, cache.write(testModuleSourceKey(), mod))
	}
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "main.tf"), []byte(`variable "`), 0o600))
	mod, diags := tfconfig.LoadModule(dir)
	require.True(t, diags.HasErrors())
	assert.Error(t, cache.write(testModuleSourceKey(), mod))
}

func TestModuleSourceCacheEmptyModule(t *testing.T) {
	cache := newTestModuleSourceCache(t)
	mod := &tfconfig.Module{Variables: map[string]*tfconfig.Variable{}, Outputs: map[string]*tfconfig.Output{}}
	require.NoError(t, cache.write(testModuleSourceKey(), mod))
	got, err := cache.read(testModuleSourceKey())
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Empty(t, got.Variables)
	assert.Empty(t, got.Outputs)
}

func TestModuleSourceCacheIOFailures(t *testing.T) {
	for _, failure := range []string{"directory", "lock", "replacement"} {
		t.Run(failure, func(t *testing.T) {
			cache := newTestModuleSourceCache(t)
			mod := moduleSourceFixture(t)
			switch failure {
			case "directory":
				file := filepath.Join(cache.dir, "file")
				require.NoError(t, os.WriteFile(file, nil, 0o600))
				cache.dir = filepath.Join(file, "cache")
			case "lock":
				require.NoError(t, os.Mkdir(cache.entryPath(testModuleSourceKey())+".lock", 0o700))
			case "replacement":
				require.NoError(t, os.Mkdir(cache.entryPath(testModuleSourceKey()), 0o700))
			}
			warnings := 0
			cache.warning = func(error) { warnings++ }
			got, err := cache.getOrFetch(context.Background(), testModuleSourceKey(), func() (*tfconfig.Module, bool, error) {
				return mod, true, nil
			})
			require.NoError(t, err)
			assert.Same(t, mod, got)
			assert.Positive(t, warnings)
			if failure == "replacement" {
				files, err := filepath.Glob(filepath.Join(cache.dir, "module-*.tmp"))
				require.NoError(t, err)
				assert.Empty(t, files)
			}
		})
	}
}

func TestModuleSourceCacheConcurrentPopulation(t *testing.T) {
	cache := newTestModuleSourceCache(t)
	mod := moduleSourceFixture(t)
	var calls atomic.Int32
	start := make(chan struct{})
	results := make(chan error, 12)
	for range cap(results) {
		go func() {
			<-start
			instance := *cache
			got, err := instance.getOrFetch(context.Background(), testModuleSourceKey(), func() (*tfconfig.Module, bool, error) {
				calls.Add(1)
				time.Sleep(30 * time.Millisecond)
				return mod, true, nil
			})
			if err == nil && (got == nil || len(got.Variables) != len(mod.Variables)) {
				err = errors.New("incomplete result")
			}
			results <- err
		}()
	}
	close(start)
	for range cap(results) {
		require.NoError(t, <-results)
	}
	assert.EqualValues(t, 1, calls.Load())
}

func TestModuleSourceCacheCancellationAndLockTimeout(t *testing.T) {
	cache := newTestModuleSourceCache(t)
	lock := flock.New(cache.entryPath(testModuleSourceKey()) + ".lock")
	require.NoError(t, lock.Lock())
	t.Cleanup(func() { require.NoError(t, lock.Close()) })
	fetch := func() (*tfconfig.Module, bool, error) {
		t.Fatal("must not duplicate population while another caller holds the lock")
		return nil, false, nil
	}
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := cache.getOrFetch(canceled, testModuleSourceKey(), fetch)
	require.ErrorIs(t, err, context.Canceled)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	start := time.Now()
	_, err = cache.getOrFetch(ctx, testModuleSourceKey(), fetch)
	require.ErrorIs(t, err, context.DeadlineExceeded)
	assert.Less(t, time.Since(start), time.Second)

	cache.lockTimeout = 30 * time.Millisecond
	start = time.Now()
	_, err = cache.getOrFetch(context.Background(), testModuleSourceKey(), fetch)
	require.ErrorIs(t, err, context.DeadlineExceeded)
	assert.ErrorContains(t, err, envModuleSourceCacheLockTimeout)
	assert.Less(t, time.Since(start), time.Second)

	require.NoError(t, lock.Close())
	ctx, cancel = context.WithCancel(context.Background())
	mod := moduleSourceFixture(t)
	_, err = cache.getOrFetch(ctx, testModuleSourceKey(), func() (*tfconfig.Module, bool, error) {
		cancel()
		return mod, true, nil
	})
	require.ErrorIs(t, err, context.Canceled)
	assert.NoFileExists(t, cache.entryPath(testModuleSourceKey()))
}

func TestModuleSourceCacheProcessHelper(t *testing.T) {
	mode := os.Getenv("MAPOTF_TEST_CACHE_PROCESS")
	if mode == "" {
		t.Skip("subprocess helper")
	}
	cache := &moduleSourceDiskCache{
		dir: os.Getenv("MAPOTF_TEST_CACHE_DIR"), inspector: "fixture-inspector",
		ttl: time.Hour, lockTimeout: 10 * time.Second,
	}
	if mode == "abandon" {
		lock := flock.New(cache.entryPath(testModuleSourceKey()) + ".lock")
		require.NoError(t, lock.Lock())
		require.NoError(t, os.WriteFile(filepath.Join(cache.dir, "ready"), nil, 0o600))
		time.Sleep(time.Minute)
		t.Fatal("parent did not terminate lock owner")
	}
	_, err := cache.getOrFetch(context.Background(), testModuleSourceKey(), func() (*tfconfig.Module, bool, error) {
		marker, err := os.CreateTemp(cache.dir, "fetch-")
		if err != nil {
			return nil, false, err
		}
		if err := marker.Close(); err != nil {
			return nil, false, err
		}
		time.Sleep(100 * time.Millisecond)
		mod, diags := tfconfig.LoadModule(os.Getenv("MAPOTF_TEST_CACHE_FIXTURE"))
		if diags.HasErrors() {
			return nil, false, diags
		}
		return mod, true, nil
	})
	require.NoError(t, err)
}

func TestModuleSourceCacheAcrossProcesses(t *testing.T) {
	for _, mode := range []string{"concurrent", "abandon"} {
		t.Run(mode, func(t *testing.T) {
			cache := newTestModuleSourceCache(t)
			mod := moduleSourceFixture(t)
			exe, err := os.Executable()
			require.NoError(t, err)
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()
			start := func() (*exec.Cmd, *bytes.Buffer) {
				cmd := exec.CommandContext(ctx, exe, "-test.run=^TestModuleSourceCacheProcessHelper$")
				cmd.Env = append(os.Environ(),
					"MAPOTF_TEST_CACHE_PROCESS="+mode, "MAPOTF_TEST_CACHE_DIR="+cache.dir,
					"MAPOTF_TEST_CACHE_FIXTURE="+mod.Path)
				var output bytes.Buffer
				cmd.Stdout, cmd.Stderr = &output, &output
				require.NoError(t, cmd.Start())
				t.Cleanup(func() {
					if cmd.ProcessState == nil {
						_ = cmd.Process.Kill()
						_ = cmd.Wait()
					}
				})
				return cmd, &output
			}
			if mode == "abandon" {
				cmd, _ := start()
				require.Eventually(t, func() bool {
					_, err := os.Stat(filepath.Join(cache.dir, "ready"))
					return err == nil
				}, 10*time.Second, 10*time.Millisecond)
				require.NoError(t, cmd.Process.Kill())
				require.Error(t, cmd.Wait())
				_, err := cache.getOrFetch(ctx, testModuleSourceKey(), func() (*tfconfig.Module, bool, error) {
					return mod, true, nil
				})
				require.NoError(t, err, "process death must release the OS lock without deleting the lock file")
				assert.FileExists(t, cache.entryPath(testModuleSourceKey())+".lock")
				return
			}
			var commands []*exec.Cmd
			var outputs []*bytes.Buffer
			for range 4 {
				cmd, output := start()
				commands = append(commands, cmd)
				outputs = append(outputs, output)
			}
			for i, cmd := range commands {
				require.NoError(t, cmd.Wait(), outputs[i].String())
			}
			files, err := filepath.Glob(filepath.Join(cache.dir, "fetch-*"))
			require.NoError(t, err)
			assert.Len(t, files, 1)
		})
	}
}

func isolateModuleSourceCacheEnvironment(t *testing.T) {
	t.Helper()
	for _, name := range []string{
		envModuleSourceCache, envModuleSourceCacheDir, envModuleSourceCacheTTL, envModuleSourceCacheLockTimeout,
		"TF_CLI_CONFIG_FILE", "TERRAFORM_CONFIG", "TF_CLI_ARGS", "TF_CLI_ARGS_get", "TF_DATA_DIR",
	} {
		t.Setenv(name, "")
	}
	for _, env := range os.Environ() {
		name, _, _ := strings.Cut(env, "=")
		if strings.HasPrefix(strings.ToUpper(name), "TF_TOKEN_") {
			t.Setenv(name, "")
		}
	}
	t.Setenv(envModuleSourceCacheDir, t.TempDir())
	oldInspector, oldPaths := moduleSourceInspector, moduleSourceCLIConfigPaths
	configDir := t.TempDir()
	moduleSourceInspector = "fixture-inspector"
	moduleSourceCLIConfigPaths = func() (string, string, error) {
		return filepath.Join(configDir, "terraform.rc"), filepath.Join(configDir, "terraform.d"), nil
	}
	t.Cleanup(func() {
		moduleSourceInspector, moduleSourceCLIConfigPaths = oldInspector, oldPaths
	})
}

func TestModuleSourceCacheConfiguration(t *testing.T) {
	isolateModuleSourceCacheEnvironment(t)
	cache := newModuleSourceDiskCacheFromEnv()
	require.NotNil(t, cache)
	assert.Equal(t, os.Getenv(envModuleSourceCacheDir), cache.dir)
	assert.Equal(t, moduleSourceCacheDefaultTTL, cache.ttl)
	assert.Equal(t, moduleSourceCacheLockTimeout, cache.lockTimeout)
	for _, mode := range []string{"0", "off", "false", "no", "disabled", " OFF "} {
		t.Setenv(envModuleSourceCache, mode)
		assert.Nil(t, newModuleSourceDiskCacheFromEnv(), mode)
	}
	t.Setenv(envModuleSourceCache, "refresh")
	t.Setenv(envModuleSourceCacheTTL, "30m")
	t.Setenv(envModuleSourceCacheLockTimeout, "750ms")
	cache = newModuleSourceDiskCacheFromEnv()
	require.NotNil(t, cache)
	assert.True(t, cache.refresh)
	assert.Equal(t, 30*time.Minute, cache.ttl)
	assert.Equal(t, 750*time.Millisecond, cache.lockTimeout)
	for _, value := range []string{"0", "-1h"} {
		t.Setenv(envModuleSourceCacheTTL, value)
		assert.LessOrEqual(t, newModuleSourceDiskCacheFromEnv().ttl, time.Duration(0))
	}
	t.Setenv(envModuleSourceCacheTTL, "invalid")
	t.Setenv(envModuleSourceCacheLockTimeout, "0")
	cache = newModuleSourceDiskCacheFromEnv()
	assert.Equal(t, moduleSourceCacheDefaultTTL, cache.ttl)
	assert.Equal(t, moduleSourceCacheLockTimeout, cache.lockTimeout)

	t.Setenv(envModuleSourceCacheDir, "")
	cache = newModuleSourceDiskCacheFromEnv()
	require.NotNil(t, cache)
	userDir, err := os.UserCacheDir()
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(userDir, "mapotf", "module-source"), cache.dir)
	moduleSourceInspector = ""
	assert.Nil(t, newModuleSourceDiskCacheFromEnv())
}

func TestModuleSourceCacheBypassesCLIConfiguration(t *testing.T) {
	for _, name := range []string{
		"TF_CLI_CONFIG_FILE", "TERRAFORM_CONFIG", "TF_CLI_ARGS", "TF_CLI_ARGS_get", "TF_DATA_DIR",
		"TF_TOKEN_registry_terraform_io", "TF_TOKEN_REGISTRY_TERRAFORM_IO",
		"TF_TOKEN_registry.terraform.io", "TF_TOKEN_other_example_com",
	} {
		t.Run(name, func(t *testing.T) {
			isolateModuleSourceCacheEnvironment(t)
			t.Setenv(name, "do-not-store-this-value")
			assert.Nil(t, newModuleSourceDiskCacheFromEnv())
			files, err := os.ReadDir(os.Getenv(envModuleSourceCacheDir))
			require.NoError(t, err)
			assert.Empty(t, files)
		})
	}
	for _, file := range []string{"terraform.rc", "credentials.tfrc.json", "custom.tfrc", "custom.tfrc.json"} {
		t.Run(file, func(t *testing.T) {
			isolateModuleSourceCacheEnvironment(t)
			configFile, configDir, err := moduleSourceCLIConfigPaths()
			require.NoError(t, err)
			require.NoError(t, os.MkdirAll(configDir, 0o700))
			path := configFile
			if file != "terraform.rc" {
				path = filepath.Join(configDir, file)
			}
			require.NoError(t, os.WriteFile(path, []byte("private configuration must not be read"), 0o600))
			assert.Nil(t, newModuleSourceDiskCacheFromEnv())
		})
	}
	t.Run("plugins_directory_is_not_configuration", func(t *testing.T) {
		isolateModuleSourceCacheEnvironment(t)
		_, configDir, err := moduleSourceCLIConfigPaths()
		require.NoError(t, err)
		require.NoError(t, os.MkdirAll(filepath.Join(configDir, "plugins"), 0o700))
		assert.NotNil(t, newModuleSourceDiskCacheFromEnv())
	})
}

func BenchmarkModuleSourceCache(b *testing.B) {
	for _, mode := range []string{"disabled", "cold", "warm"} {
		b.Run(mode, func(b *testing.B) {
			cache := newTestModuleSourceCache(b)
			fixture := moduleSourceFixture(b)
			calls := 0
			fetch := func() (*tfconfig.Module, bool, error) {
				calls++
				time.Sleep(20 * time.Millisecond)
				mod, diags := tfconfig.LoadModule(fixture.Path)
				if diags.HasErrors() {
					return nil, false, diags
				}
				return mod, true, nil
			}
			if mode == "warm" {
				_, err := cache.getOrFetch(context.Background(), testModuleSourceKey(), fetch)
				require.NoError(b, err)
			}
			if mode == "disabled" {
				cache = nil
			}
			calls = 0
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				key := testModuleSourceKey()
				if mode == "cold" {
					key.version = fmt.Sprintf("1.0.%d", i)
				}
				_, err := cache.getOrFetch(context.Background(), key, fetch)
				if err != nil {
					b.Fatal(err)
				}
			}
			b.StopTimer()
			b.ReportMetric(float64(calls)/float64(b.N), "fetches/op")
		})
	}
}
