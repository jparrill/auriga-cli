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

func TestConfiguredModels(t *testing.T) {
	tests := []struct {
		name      string
		value     any
		wantCount int
	}{
		{
			name:      "When models is an empty slice, it should return empty",
			value:     []string{},
			wantCount: 0,
		},
		{
			name:      "When models is a string slice with 3 items, it should return 3",
			value:     []string{"qwen3:32b", "devstral", "gemma3:27b"},
			wantCount: 3,
		},
		{
			name:      "When models is a single-item slice, it should return 1",
			value:     []string{"qwen3:32b"},
			wantCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			viper.Set("ollama.models", tt.value)
			models := ConfiguredModels()
			if len(models) != tt.wantCount {
				t.Errorf("got %d models, want %d: %v", len(models), tt.wantCount, models)
			}
		})
	}
}

func TestHasModel(t *testing.T) {
	tests := []struct {
		name string
		host string
		want bool
	}{
		{
			name: "When host is unreachable, it should return false",
			host: "http://localhost:99999",
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			viper.Set("ollama.host", tt.host)
			if HasModel("test") != tt.want {
				t.Errorf("got %v, want %v", !tt.want, tt.want)
			}
		})
	}
}

func TestDeleteModel(t *testing.T) {
	tests := []struct {
		name    string
		host    string
		wantErr bool
	}{
		{
			name:    "When host is unreachable, it should return an error",
			host:    "http://localhost:99999",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			viper.Set("ollama.host", tt.host)
			err := DeleteModel("nonexistent")
			if (err != nil) != tt.wantErr {
				t.Errorf("got err=%v, wantErr=%v", err, tt.wantErr)
			}
		})
	}
}
