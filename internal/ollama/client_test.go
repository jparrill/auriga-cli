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
	viper.Set("ollama.models", "")
	models := ConfiguredModels()
	if models != nil {
		t.Errorf("expected nil for empty, got %v", models)
	}
}

func TestConfiguredModels_SpaceSeparated(t *testing.T) {
	viper.Set("ollama.models", "model1 model2 model3")
	models := ConfiguredModels()
	if len(models) != 3 {
		t.Errorf("expected 3 models, got %d", len(models))
	}
	if models[0] != "model1" || models[2] != "model3" {
		t.Errorf("unexpected models: %v", models)
	}
}

func TestHasModel_Unreachable(t *testing.T) {
	viper.Set("ollama.host", "http://localhost:99999")
	if HasModel("test") {
		t.Error("expected false for unreachable host")
	}
}
