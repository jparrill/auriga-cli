package benchmark

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/jparrill/auriga-cli/internal/config"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

type RegistryEntry struct {
	URL         string `yaml:"url"`
	Description string `yaml:"description"`
	Language    string `yaml:"language"`
	Format      string `yaml:"format"`
	Compressed  bool   `yaml:"compressed,omitempty"`
	Runner      string `yaml:"runner,omitempty"`
}

var defaultRegistry = map[string]RegistryEntry{
	"humaneval-go": {
		URL:         "https://github.com/zai-org/CodeGeeX2/raw/main/benchmark/humanevalx/humanevalx_go.jsonl.gz",
		Description: "HumanEval-X Go — 164 Go coding problems with unit tests",
		Language:    "go",
		Format:      "humaneval",
		Compressed:  true,
		Runner:      "go",
	},
}

func RegistryPath() string {
	if v := viper.GetString("benchmark.registry"); v != "" {
		return config.ExpandHome(v)
	}
	return config.ExpandHome("~/.config/auriga/suite-registry.yaml")
}

func LoadRegistry() (map[string]RegistryEntry, error) {
	path := RegistryPath()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return copyDefaultRegistry(), nil
		}
		return nil, fmt.Errorf("read registry: %w", err)
	}

	var entries map[string]RegistryEntry
	if err := yaml.Unmarshal(data, &entries); err != nil {
		return nil, fmt.Errorf("parse registry: %w", err)
	}
	if entries == nil {
		entries = make(map[string]RegistryEntry)
	}
	return entries, nil
}

func SaveRegistry(entries map[string]RegistryEntry) error {
	path := RegistryPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create registry dir: %w", err)
	}

	data, err := yaml.Marshal(entries)
	if err != nil {
		return fmt.Errorf("marshal registry: %w", err)
	}
	return os.WriteFile(path, data, 0o644)
}

func AddRegistryEntry(name string, entry RegistryEntry) error {
	entries, err := LoadRegistry()
	if err != nil {
		return err
	}
	entries[name] = entry
	return SaveRegistry(entries)
}

func RemoveRegistryEntry(name string) error {
	entries, err := LoadRegistry()
	if err != nil {
		return err
	}
	delete(entries, name)
	return SaveRegistry(entries)
}

func copyDefaultRegistry() map[string]RegistryEntry {
	out := make(map[string]RegistryEntry, len(defaultRegistry))
	for k, v := range defaultRegistry {
		out[k] = v
	}
	return out
}
