package ps

import (
	"os"
	"testing"

	"github.com/jparrill/auriga-cli/internal/ui"
	"github.com/spf13/viper"
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
	t.Setenv("OLLAMA_MODELS_DIR", "/tmp/test-ollama")
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
	if len(procs) < 3 {
		t.Errorf("expected at least 3 components, got %d", len(procs))
	}
}

func TestDetectType(t *testing.T) {
	tests := []struct {
		name      string
		modelName string
		want      string
	}{
		{"When model has A3B, it should detect MoE", "Qwen3.6-35B-A3B-Q8_0.gguf", "moe"},
		{"When model has A4B, it should detect MoE", "gemma-4-26B-A4B-it-UD-Q4_K_M.gguf", "moe"},
		{"When model is dense, it should detect dense", "Qwen3.6-27B-UD-Q8_K_XL.gguf", "dense"},
		{"When model is DeepSeek, it should detect dense", "DeepSeek-R1-Distill-Qwen-32B-Q8_0.gguf", "dense"},
		{"When empty, it should default dense", "", "dense"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := detectType(tt.modelName)
			if got != tt.want {
				t.Errorf("detectType(%q) = %q, want %q", tt.modelName, got, tt.want)
			}
		})
	}
}

func TestResolveProfile_Found(t *testing.T) {
	viper.Reset()
	defer viper.Reset()

	viper.Set("profiles.gemma4-26b.model", "gemma-4-26B-A4B-it-UD-Q4_K_M.gguf")
	viper.Set("profiles.gemma4-26b.type", "moe")

	profile, modelType := resolveProfile("gemma-4-26B-A4B-it-UD-Q4_K_M.gguf")
	if profile != "gemma4-26b" {
		t.Errorf("When model matches profile, should return profile name, got %q", profile)
	}
	if modelType != "moe" {
		t.Errorf("When profile has type, should return it, got %q", modelType)
	}
}

func TestResolveProfile_NotFound(t *testing.T) {
	viper.Reset()
	defer viper.Reset()

	profile, modelType := resolveProfile("unknown-model.gguf")
	if profile != "-" {
		t.Errorf("When model not in profiles, should return -, got %q", profile)
	}
	if modelType != "dense" {
		t.Errorf("When model not in profiles and dense, should detect dense, got %q", modelType)
	}
}

func TestResolveProfile_AutoDetectType(t *testing.T) {
	viper.Reset()
	defer viper.Reset()

	viper.Set("profiles.gemma4-test.model", "gemma-4-26B-A4B-it.gguf")

	profile, modelType := resolveProfile("gemma-4-26B-A4B-it.gguf")
	if profile != "gemma4-test" {
		t.Errorf("When model matches, should return profile name, got %q", profile)
	}
	if modelType != "moe" {
		t.Errorf("When type not set and model is MoE, should auto-detect, got %q", modelType)
	}
}

func TestResolveProfile_EmptyModel(t *testing.T) {
	profile, modelType := resolveProfile("")
	if profile != "-" || modelType != "-" {
		t.Errorf("When empty model, should return -, got %q %q", profile, modelType)
	}
}

func TestDetectManagement_NoPort(t *testing.T) {
	got := detectManagement("-")
	if got != "process" {
		t.Errorf("When no port, management should be process, got %q", got)
	}
}

func TestCheckHealth_InvalidPort(t *testing.T) {
	viper.Reset()
	defer viper.Reset()

	viper.Set("llama_server.host", "http://localhost:8090")

	got := checkHealth("59999")
	if got != "unreachable" {
		t.Errorf("When port not listening, health should be unreachable, got %q", got)
	}
}
