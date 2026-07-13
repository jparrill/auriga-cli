package ollama

import (
	"testing"

	"github.com/spf13/viper"
)

func TestHost(t *testing.T) {
	viper.Set("ollama.host", "http://test:11434")
	h := Host()
	if h != "http://test:11434" {
		t.Errorf("expected http://test:11434, got %s", h)
	}
}

func TestConfiguredModels_Empty(t *testing.T) {
	viper.Set("ollama.models", []string{})
	models := ConfiguredModels()
	if len(models) != 0 {
		t.Errorf("expected empty for no models, got %v", models)
	}
}

func TestConfiguredModels_StringSlice(t *testing.T) {
	viper.Set("ollama.models", []string{"qwen3:32b", "devstral", "gemma3:27b"})
	models := ConfiguredModels()
	if len(models) != 3 {
		t.Errorf("expected 3 models, got %d", len(models))
	}
	if models[0] != "qwen3:32b" || models[2] != "gemma3:27b" {
		t.Errorf("unexpected models: %v", models)
	}
}

func TestHasModel_Unreachable(t *testing.T) {
	viper.Set("ollama.host", "http://localhost:99999")
	if HasModel("test") {
		t.Error("expected false for unreachable host")
	}
}
