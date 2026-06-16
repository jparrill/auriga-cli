package config

import (
	"os"
	"strings"
	"testing"
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
		{"DefaultPiBin", DefaultPiBin},
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
		DefaultGGUFDir, DefaultMMProjDir, DefaultLlamaServerBin, DefaultPiBin,
	}
	for _, p := range paths {
		if !strings.HasPrefix(p, "~/") {
			t.Errorf("path %q should start with ~/", p)
		}
	}
}
