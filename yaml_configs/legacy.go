package yaml_configs

import "sync"

// Package-level config used by the v1.1 API (LoadConfigWithOverrides + Get).

var (
	globalMu sync.RWMutex
	global   *Config
)

// LoadConfigWithSuffix loads path.yaml, then path.suffix.yaml, and installs the
// result as the package-level config read by Get. Missing files are skipped.
//
// Deprecated: Use LoadWithSuffix, which requires the base file and supports Env.
func LoadConfigWithSuffix(path string, suffix string) (*Config, error) {
	return LoadConfigWithOverrides(path+".yaml", path+"."+suffix+".yaml")
}

// LoadConfigWithOverrides loads files in order (later files override earlier
// ones) and installs the result as the package-level config read by Get.
// Missing files are skipped. Each call reloads; on error the previously
// installed config is kept.
//
// Deprecated: Use Load with File, OptionalFile and Env.
func LoadConfigWithOverrides(paths ...string) (*Config, error) {
	srcs := make([]Source, len(paths))
	for i, p := range paths {
		srcs[i] = OptionalFile(p)
	}
	cfg, err := Load(srcs...)
	if err != nil {
		return nil, err
	}
	SetGlobal(cfg)
	return cfg, nil
}

// SetGlobal installs cfg as the package-level config read by Get.
func SetGlobal(cfg *Config) {
	globalMu.Lock()
	global = cfg
	globalMu.Unlock()
}

// Get returns the package-level config value for key converted to T (see
// GetAs). It returns the zero value if no config is loaded, the key is unset,
// or the value cannot be converted; it never panics.
func Get[T any](key string) T {
	globalMu.RLock()
	cfg := global
	globalMu.RUnlock()
	v, _ := GetAs[T](cfg, key)
	return v
}
