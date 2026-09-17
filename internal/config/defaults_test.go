package config

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestExpandHome(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"tilde path", "~/foo/bar", home + "/foo/bar"},
		{"absolute path", "/usr/local/bin", "/usr/local/bin"},
		{"relative path", "foo/bar", "foo/bar"},
		{"empty string", "", ""},
		{"tilde only", "~", "~"},
		{"tilde slash", "~/", home},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExpandHome(tt.input)
			if got != tt.expected {
				t.Errorf("ExpandHome(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestDefaultsNotEmpty(t *testing.T) {
	defaults := []struct {
		name  string
		value string
	}{
		{"DefaultGGUFDir", DefaultGGUFDir},
		{"DefaultMMProjDir", DefaultMMProjDir},
		{"DefaultLlamaServerBin", DefaultLlamaServerBin},
		{"DefaultOllamaHost", DefaultOllamaHost},
		{"DefaultQuant", DefaultQuant},
	}

	for _, d := range defaults {
		t.Run(d.name, func(t *testing.T) {
			if d.value == "" {
				t.Errorf("%s is empty", d.name)
			}
		})
	}
}

func TestDefaultsContainTilde(t *testing.T) {
	paths := []string{
		DefaultGGUFDir, DefaultMMProjDir, DefaultLlamaServerBin,
	}
	for _, p := range paths {
		if !strings.HasPrefix(p, "~/") {
			t.Errorf("path %q should start with ~/", p)
		}
	}
}

func TestConfigExample_UsesPublicConfigurationKeys(t *testing.T) {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("could not locate test file")
	}

	data, err := os.ReadFile(filepath.Join(filepath.Dir(filename), "..", "..", "config.yaml.example"))
	if err != nil {
		t.Fatal(err)
	}

	var config map[string]any
	if err := yaml.Unmarshal(data, &config); err != nil {
		t.Fatalf("example config should be valid YAML: %v", err)
	}

	llamaServer, ok := config["llama_server"].(map[string]any)
	if !ok {
		t.Fatal("example config should define llama_server mapping")
	}
	for _, key := range []string{"slot_1_port", "slot_2_port"} {
		if _, ok := llamaServer[key]; !ok {
			t.Errorf("example config should define llama_server.%s", key)
		}
	}
	for _, key := range []string{"dense_port", "moe_port"} {
		if _, ok := llamaServer[key]; ok {
			t.Errorf("example config should not define obsolete llama_server.%s", key)
		}
	}
	if _, ok := config["pi"]; ok {
		t.Error("example config should not define obsolete pi section")
	}
}
