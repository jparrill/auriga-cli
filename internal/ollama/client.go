package ollama

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/jparrill/auriga-cli/internal/config"
	"github.com/spf13/viper"
)

type Model struct {
	Name       string `json:"name"`
	Size       int64  `json:"size"`
	ModifiedAt string `json:"modified_at"`
}

type tagsResponse struct {
	Models []Model `json:"models"`
}

type generateRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
	Stream bool   `json:"stream"`
	Options map[string]interface{} `json:"options,omitempty"`
}

type generateResponse struct {
	Response string `json:"response"`
}

func Host() string {
	return viper.GetString("ollama.host")
}

func ConfiguredModels() []string {
	raw := viper.GetString("ollama.models")
	if raw == "" {
		return nil
	}
	return strings.Fields(raw)
}

func ListModels() ([]Model, error) {
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(Host() + "/api/tags")
	if err != nil {
		return nil, fmt.Errorf("cannot reach Ollama at %s: %w", Host(), err)
	}
	defer resp.Body.Close()

	var tags tagsResponse
	if err := json.NewDecoder(resp.Body).Decode(&tags); err != nil {
		return nil, fmt.Errorf("invalid response: %w", err)
	}
	return tags.Models, nil
}

func HasModel(name string) bool {
	models, err := ListModels()
	if err != nil {
		return false
	}
	for _, m := range models {
		if m.Name == name {
			return true
		}
	}
	return false
}

func Generate(model, prompt string, maxTokens int, timeout time.Duration) (string, error) {
	_ = config.DryRun

	payload := generateRequest{
		Model:  model,
		Prompt: prompt,
		Stream: false,
		Options: map[string]interface{}{
			"num_predict": maxTokens,
			"temperature": 0.3,
		},
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	client := &http.Client{Timeout: timeout}
	resp, err := client.Post(Host()+"/api/generate", "application/json", strings.NewReader(string(data)))
	if err != nil {
		return "", fmt.Errorf("Ollama call failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("Ollama returned %d", resp.StatusCode)
	}

	var genResp generateResponse
	if err := json.NewDecoder(resp.Body).Decode(&genResp); err != nil {
		return "", fmt.Errorf("invalid response: %w", err)
	}
	return genResp.Response, nil
}
