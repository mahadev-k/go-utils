package yaml_configs

import (
	"fmt"
	"os"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"
)

var (
	configDoOnce sync.Once
	config       *Config
)

type Config struct {
	configMap     map[string]any
	configFlatMap map[string]any
}

// LoadConfigWithSuffix loads a config file with a suffix, and overrides the config with the suffix file
// file path is path.suffix.yaml
// provide path without .yaml
func LoadConfigWithSuffix(path string, suffix string) (*Config, error) {
	return LoadConfigWithOverrides(
		fmt.Sprintf("%s.yaml", path),
		fmt.Sprintf("%s.%s.yaml", path, suffix),
	)
}

// LoadConfigWithOverrides loads configs in order, with later files overriding earlier ones
func LoadConfigWithOverrides(paths ...string) (*Config, error) {
	var loadErr error

	configDoOnce.Do(func() {
		config = &Config{
			configMap:     make(map[string]any),
			configFlatMap: make(map[string]any),
		}

		// Load each config file in order
		for _, path := range paths {
			if err := loadAndMerge(path, config); err != nil {
				loadErr = err
				return
			}
		}
	})

	if loadErr != nil {
		return nil, loadErr
	}
	return config, nil
}

func loadAndMerge(path string, cfg *Config) error {
	yamlFile, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			// Skip if file doesn't exist
			fmt.Printf("File %s does not exist, skipping\n", path)
			return nil
		}
		return err
	}
	defer yamlFile.Close()

	var newConfig map[string]any
	if err := yaml.NewDecoder(yamlFile).Decode(&newConfig); err != nil {
		return err
	}

	// Merge new config into existing
	mergeMap(cfg.configMap, newConfig)

	// Rebuild flat map
	cfg.configFlatMap = make(map[string]any)
	flattenConfig(cfg.configMap, "", cfg.configFlatMap)

	return nil
}

// mergeMap recursively merges src into dst
func mergeMap(dst, src map[string]any) {
	for key, srcVal := range src {
		if dstVal, exists := dst[key]; exists {
			// If both are maps, merge recursively
			if dstMap, ok := dstVal.(map[string]any); ok {
				if srcMap, ok := srcVal.(map[string]any); ok {
					mergeMap(dstMap, srcMap)
					continue
				}
			}
		}
		// Otherwise override the value
		dst[key] = srcVal
	}
}

func flattenConfig(configMap map[string]any, prefix string, flatMap map[string]any) {
	for key, value := range configMap {
		var newKey string
		if prefix == "" {
			newKey = key
		} else {
			newKey = fmt.Sprintf("%s.%s", prefix, key)
		}
		if nestedMap, ok := value.(map[string]any); ok {
			flattenConfig(nestedMap, newKey, flatMap)
		} else {

			flatMap[newKey] = value
			flatMap[strings.ToUpper(newKey)] = value
			flatMap[strings.ToLower(newKey)] = value
			flatMap[strings.ReplaceAll(newKey, ".", "_")] = value
		}
	}
}

func (c *Config) Get(key string) any {
	return c.configFlatMap[key]
}

func Get[T any](key string) T {
	value, ok := config.configFlatMap[key]
	if !ok {
		return *new(T)
	}
	return value.(T)
}
