package benchmark

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/viper"
)

func TestLoadRegistry_DefaultWhenMissing(t *testing.T) {
	viper.Set("benchmark.registry", filepath.Join(t.TempDir(), "no-such-registry.yaml"))

	entries, err := LoadRegistry()
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := entries["humaneval-go"]; !ok {
		t.Error("expected default humaneval-go entry")
	}
}

func TestLoadRegistry_FromFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "registry.yaml")
	content := `custom-suite:
  url: "https://example.com/suite.jsonl"
  description: "Test suite"
  language: go
  format: humaneval
`
	os.WriteFile(path, []byte(content), 0644)
	viper.Set("benchmark.registry", path)

	entries, err := LoadRegistry()
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Errorf("expected 1 entry, got %d", len(entries))
	}
	if entries["custom-suite"].URL != "https://example.com/suite.jsonl" {
		t.Errorf("unexpected URL: %s", entries["custom-suite"].URL)
	}
}

func TestSaveAndLoadRegistry(t *testing.T) {
	path := filepath.Join(t.TempDir(), "registry.yaml")
	viper.Set("benchmark.registry", path)

	entries := map[string]RegistryEntry{
		"test-suite": {
			URL:         "https://example.com/test.jsonl.gz",
			Description: "A test",
			Language:    "go",
			Format:      "humaneval",
			Compressed:  true,
			Runner:      "go",
		},
	}

	if err := SaveRegistry(entries); err != nil {
		t.Fatal(err)
	}

	loaded, err := LoadRegistry()
	if err != nil {
		t.Fatal(err)
	}
	if loaded["test-suite"].URL != entries["test-suite"].URL {
		t.Errorf("URL mismatch: got %q", loaded["test-suite"].URL)
	}
	if !loaded["test-suite"].Compressed {
		t.Error("expected compressed=true")
	}
}

func TestAddRegistryEntry(t *testing.T) {
	path := filepath.Join(t.TempDir(), "registry.yaml")
	viper.Set("benchmark.registry", path)

	err := AddRegistryEntry("new-suite", RegistryEntry{
		URL:         "https://example.com/new.jsonl",
		Description: "New suite",
		Language:    "python",
		Format:      "humaneval",
	})
	if err != nil {
		t.Fatal(err)
	}

	entries, err := LoadRegistry()
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := entries["new-suite"]; !ok {
		t.Error("new-suite not found after add")
	}
	if _, ok := entries["humaneval-go"]; !ok {
		t.Error("default humaneval-go should still exist")
	}
}

func TestRemoveRegistryEntry(t *testing.T) {
	path := filepath.Join(t.TempDir(), "registry.yaml")
	viper.Set("benchmark.registry", path)

	AddRegistryEntry("to-remove", RegistryEntry{URL: "https://example.com/rm.jsonl"})

	err := RemoveRegistryEntry("to-remove")
	if err != nil {
		t.Fatal(err)
	}

	entries, err := LoadRegistry()
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := entries["to-remove"]; ok {
		t.Error("to-remove should be gone")
	}
}

func TestLoadRegistry_EmptyFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "registry.yaml")
	os.WriteFile(path, []byte(""), 0644)
	viper.Set("benchmark.registry", path)

	entries, err := LoadRegistry()
	if err != nil {
		t.Fatal(err)
	}
	if entries == nil {
		t.Error("expected non-nil map for empty file")
	}
}

func TestDefaultRegistryIsCopy(t *testing.T) {
	viper.Set("benchmark.registry", filepath.Join(t.TempDir(), "no-file.yaml"))

	entries1, _ := LoadRegistry()
	entries1["mutated"] = RegistryEntry{URL: "test"}

	entries2, _ := LoadRegistry()
	if _, ok := entries2["mutated"]; ok {
		t.Error("default registry should not be shared between calls")
	}
}
