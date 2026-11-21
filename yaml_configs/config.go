// Package yaml_configs loads layered configuration from YAML files and
// environment variables.
//
//	cfg, err := yaml_configs.Load(
//		yaml_configs.File("config/env.yaml"),               // required
//		yaml_configs.OptionalFile("config/env.local.yaml"), // skipped if missing
//		yaml_configs.Env("MYAPP_"),                        // MYAPP_DATABASE_PORT overrides database.port
//		yaml_configs.Resolve(awsref.New(awsCfg)),           // "aws::ssm::/app/db-password" -> its value
//	)
//
// Sources are applied in order; later sources override earlier ones. YAML
// files are deep-merged: an override file only needs the keys it changes.
//
// Keys are case-insensitive and "." / "-" / "_" are interchangeable, so
// "database.port", "DATABASE_PORT" and "Database.Port" all name the same value.
// This is what lets an environment variable override a nested YAML key.
package yaml_configs

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

var (
	// ErrNotFound is returned when a key has no value (missing or YAML null).
	ErrNotFound = errors.New("config key not found")
	// ErrInvalidValue is returned when a value cannot be converted to the requested type.
	ErrInvalidValue = errors.New("invalid config value")
)

// Config is an immutable snapshot of loaded configuration. It is safe for
// concurrent use. A nil *Config behaves as an empty config.
type Config struct {
	entries map[string]entry
}

type entry struct {
	path  string // normalized dotted path ("DATABASE.PORT"); "" for env vars
	raw   string // key as written in the source, for error messages
	value any
}

// Source is one configuration layer passed to Load.
type Source interface {
	apply(ctx context.Context, entries map[string]entry) error
}

// File is a YAML file that must exist.
func File(path string) Source { return fileSource{path: path} }

// OptionalFile is a YAML file that is skipped if it does not exist.
func OptionalFile(path string) Source { return fileSource{path: path, optional: true} }

// Env overlays environment variables whose name starts with prefix (matched
// case-insensitively). The prefix is stripped, so with Env("MYAPP_") the
// variable MYAPP_DATABASE_PORT sets "database.port". Variables with an empty
// value are ignored. Env("") reads every variable; a prefix is recommended.
// The environment is read once, when Load runs.
func Env(prefix string) Source { return envSource{prefix: strings.ToUpper(prefix)} }

// Load applies sources in order and returns the resulting config.
func Load(sources ...Source) (*Config, error) {
	return LoadContext(context.Background(), sources...)
}

// LoadContext is Load with a context, used by Resolve sources for remote lookups.
func LoadContext(ctx context.Context, sources ...Source) (*Config, error) {
	entries := make(map[string]entry)
	for _, s := range sources {
		if err := s.apply(ctx, entries); err != nil {
			return nil, err
		}
	}
	return &Config{entries: entries}, nil
}

// LoadWithSuffix loads path.yaml (required), then path.suffix.yaml (optional),
// then any extra sources such as Env. Provide path without ".yaml".
func LoadWithSuffix(path, suffix string, extra ...Source) (*Config, error) {
	srcs := []Source{File(path + ".yaml"), OptionalFile(path + "." + suffix + ".yaml")}
	return Load(append(srcs, extra...)...)
}

type fileSource struct {
	path     string
	optional bool
}

func (s fileSource) apply(_ context.Context, entries map[string]entry) error {
	f, err := os.Open(s.path)
	if err != nil {
		if s.optional && errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("config file %s: %w", s.path, err)
	}
	defer f.Close()

	var doc any
	if err := yaml.NewDecoder(f).Decode(&doc); err != nil {
		if errors.Is(err, io.EOF) { // empty or comment-only file
			return nil
		}
		return fmt.Errorf("config file %s: %w", s.path, err)
	}
	if doc == nil {
		return nil
	}
	tree, ok := asMap(doc)
	if !ok {
		return fmt.Errorf("config file %s: top level must be a mapping, got %T", s.path, doc)
	}

	leaves := make(map[string]entry)
	if err := flatten(tree, "", "", leaves); err != nil {
		return fmt.Errorf("config file %s: %w", s.path, err)
	}
	for key, e := range leaves {
		// Deep-merge semantics: a value replaces anything nested below it
		// (scalar replaces map), and a nested value replaces a scalar at any
		// of its parents (map replaces scalar).
		for k, old := range entries {
			if old.path == "" {
				continue
			}
			if strings.HasPrefix(old.path, e.path+".") || strings.HasPrefix(e.path, old.path+".") {
				delete(entries, k)
			}
		}
		entries[key] = e
	}
	return nil
}

func flatten(m map[string]any, path, raw string, out map[string]entry) error {
	for k, v := range m {
		p, r := normalizeSegment(k), k
		if path != "" {
			p, r = path+"."+p, raw+"."+k
		}
		if nested, ok := asMap(v); ok {
			if err := flatten(nested, p, r, out); err != nil {
				return err
			}
			continue
		}
		key := strings.ReplaceAll(p, ".", "_")
		if prev, dup := out[key]; dup {
			// Sort so the message does not depend on map iteration order.
			a, b := prev.raw, r
			if a > b {
				a, b = b, a
			}
			return fmt.Errorf("keys %q and %q refer to the same setting %s", a, b, key)
		}
		out[key] = entry{path: p, raw: r, value: v}
	}
	return nil
}

func asMap(v any) (map[string]any, bool) {
	switch m := v.(type) {
	case map[string]any:
		return m, true
	case map[any]any:
		out := make(map[string]any, len(m))
		for k, v := range m {
			out[fmt.Sprint(k)] = v
		}
		return out, true
	}
	return nil, false
}

type envSource struct{ prefix string }

func (s envSource) apply(_ context.Context, entries map[string]entry) error {
	for _, kv := range os.Environ() {
		name, value, ok := strings.Cut(kv, "=")
		if !ok || value == "" || !strings.HasPrefix(strings.ToUpper(name), s.prefix) {
			continue
		}
		key := normalizeKey(name[len(s.prefix):])
		if key == "" {
			continue
		}
		entries[key] = entry{raw: name, value: value}
	}
	return nil
}

func normalizeSegment(s string) string {
	return strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(s), "-", "_"))
}

func normalizeKey(key string) string {
	return strings.ReplaceAll(normalizeSegment(key), ".", "_")
}

// Lookup returns the raw value for key and whether it is set. YAML null counts as unset.
func (c *Config) Lookup(key string) (any, bool) {
	if c == nil {
		return nil, false
	}
	e, ok := c.entries[normalizeKey(key)]
	if !ok || e.value == nil {
		return nil, false
	}
	return e.value, true
}

// Get returns the raw value for key, or nil if unset.
func (c *Config) Get(key string) any {
	v, _ := c.Lookup(key)
	return v
}

// Has reports whether key is set.
func (c *Config) Has(key string) bool {
	_, ok := c.Lookup(key)
	return ok
}

// Keys returns all set keys in normalized form (e.g. "DATABASE_PORT"), sorted.
func (c *Config) Keys() []string {
	if c == nil {
		return nil
	}
	keys := make([]string, 0, len(c.entries))
	for k, e := range c.entries {
		if e.value != nil {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	return keys
}

// GetAs returns the value for key converted to T. Supported conversions:
// string, bool, int, int64, float64, time.Duration and []string (from a YAML
// list or a comma-separated string). Strings — e.g. from environment
// variables — are parsed. Other types must match the decoded YAML value
// exactly. Errors wrap ErrNotFound or ErrInvalidValue.
func GetAs[T any](c *Config, key string) (T, error) {
	var zero T
	v, ok := c.Lookup(key)
	if !ok {
		return zero, fmt.Errorf("%w: %s", ErrNotFound, key)
	}
	out, err := convert[T](v)
	if err != nil {
		return zero, fmt.Errorf("config key %s: %w", key, err)
	}
	return out, nil
}

// GetOr returns the value for key converted to T, or def if the key is unset
// or cannot be converted.
func GetOr[T any](c *Config, key string, def T) T {
	v, err := GetAs[T](c, key)
	if err != nil {
		return def
	}
	return v
}
