package ps

import (
	"os"
	"testing"

	"github.com/jparrill/auriga-cli/internal/ui"
)

func TestMain(m *testing.M) {
	ui.InitLogger(false)
	os.Exit(m.Run())
}

func TestExtractFlag(t *testing.T) {
	tests := []struct {
		name     string
		args     string
		flag     string
		expected string
	}{
		{"model flag", "/bin/llama-server -m /path/to/model.gguf --port 8090", "-m", "/path/to/model.gguf"},
		{"port flag", "/bin/llama-server -m model.gguf --port 8090", "--port", "8090"},
		{"mmproj flag", "/bin/llama-server -m model.gguf --mmproj /path/mmproj.gguf", "--mmproj", "/path/mmproj.gguf"},
		{"model in pi", "pi --model local", "--model", "local"},
		{"missing flag", "/bin/llama-server -m model.gguf", "--mmproj", ""},
		{"empty args", "", "--model", ""},
		{"flag at end no value", "/bin/llama-server --flash-attn", "--flash-attn", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractFlag(tt.args, tt.flag)
			if result != tt.expected {
				t.Errorf("extractFlag(%q, %q) = %q, want %q", tt.args, tt.flag, result, tt.expected)
			}
		})
	}
}

func TestResolveOllamaModelsDir_FromEnv(t *testing.T) {
	t.Setenv("OLLAMA_MODELS", "/tmp/test-ollama")
	dir := resolveOllamaModelsDir()
	if dir != "/tmp/test-ollama" {
		t.Errorf("expected /tmp/test-ollama, got %q", dir)
	}
}

func TestFormatGB(t *testing.T) {
	tests := []struct {
		bytes    int64
		expected string
	}{
		{0, "0.0 GB"},
		{1073741824, "1.0 GB"},
		{10737418240, "10.0 GB"},
	}
	for _, tt := range tests {
		result := formatGB(tt.bytes)
		if result != tt.expected {
			t.Errorf("formatGB(%d) = %q, want %q", tt.bytes, result, tt.expected)
		}
	}
}

func TestFormatBytesStr(t *testing.T) {
	result := formatBytesStr([]byte("1073741824\n"))
	if result != "1.0 GB" {
		t.Errorf("expected '1.0 GB', got %q", result)
	}
}

func TestGatherStatus(t *testing.T) {
	if os.Getenv("AURIGA_TEST_PS") == "" {
		t.Skip("skipping ps test (set AURIGA_TEST_PS=1 on Linux)")
	}
	procs := gatherStatus()
	if len(procs) != 3 {
		t.Errorf("expected 3 components, got %d", len(procs))
	}
}
