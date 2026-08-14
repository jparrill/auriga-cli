package show

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/spf13/viper"
)

func newTestServer(model string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/health":
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, `{"status":"ok"}`)
		case r.URL.Path == "/v1/chat/completions":
			var req map[string]any
			json.NewDecoder(r.Body).Decode(&req)

			resp := map[string]any{
				"model": model,
				"choices": []map[string]any{
					{"message": map[string]string{"role": "assistant", "content": "Mountains rise high"}},
				},
				"timings": map[string]any{
					"prompt_n":           15,
					"prompt_ms":          500.0,
					"prompt_per_second":  30.0,
					"predicted_n":        20,
					"predicted_ms":       2000.0,
					"predicted_per_second": 10.0,
				},
			}
			json.NewEncoder(w).Encode(resp)
		}
	}))
}

func TestIsPortHealthy_Healthy(t *testing.T) {
	srv := newTestServer("test-model")
	defer srv.Close()
	port := extractTestPort(t, srv)

	if !isPortHealthy(port) {
		t.Error("When server healthy, isPortHealthy should return true")
	}
}

func TestIsPortHealthy_NoServer(t *testing.T) {
	if isPortHealthy(59876) {
		t.Error("When no server, isPortHealthy should return false")
	}
}

func TestRunBench_NoThinking(t *testing.T) {
	srv := newTestServer("test-model")
	defer srv.Close()
	port := extractTestPort(t, srv)

	result := runBench(port, false)

	if result.Error != "" {
		t.Errorf("When server responds, runBench should not error, got: %s", result.Error)
	}
	if result.GenerationTokPerSec != 10.0 {
		t.Errorf("When server reports 10 tok/s, got %.1f", result.GenerationTokPerSec)
	}
	if result.PromptTokPerSec != 30.0 {
		t.Errorf("When server reports 30 tok/s prompt, got %.1f", result.PromptTokPerSec)
	}
}

func TestRunBench_ServerDown(t *testing.T) {
	result := runBench(59877, false)

	if result.Error == "" {
		t.Error("When server down, runBench should return error")
	}
}

func TestMeasureTTFT_ReturnsPositive(t *testing.T) {
	srv := newTestServer("test-model")
	defer srv.Close()
	port := extractTestPort(t, srv)

	ttft, model := measureTTFT(port)

	if ttft <= 0 {
		t.Error("When server responds, TTFT should be positive")
	}
	if model != "test-model" {
		t.Errorf("When server returns model, got %q, want test-model", model)
	}
}

func TestFmtTokS_Normal(t *testing.T) {
	got := fmtTokS(10.5, "")
	if got != "10.5 tok/s" {
		t.Errorf("fmtTokS(10.5) = %q, want '10.5 tok/s'", got)
	}
}

func TestFmtTokS_Error(t *testing.T) {
	got := fmtTokS(0, "connection refused")
	if got != "error" {
		t.Errorf("fmtTokS with error = %q, want 'error'", got)
	}
}

func TestResolveProfilePort_Dense(t *testing.T) {
	viper.Reset()
	defer viper.Reset()

	viper.Set("profiles.test-dense.model", "Qwen3.8-27B-Q8_0.gguf")
	viper.Set("profiles.test-dense.type", "dense")
	viper.Set("llama_server.dense_port", 8090)
	viper.Set("llama_server.host", "http://localhost:8090")

	port := resolveProfilePort("test-dense")
	if port != 8090 {
		t.Errorf("When dense profile, port should be 8090, got %d", port)
	}
}

func TestResolveProfilePort_MoE(t *testing.T) {
	viper.Reset()
	defer viper.Reset()

	viper.Set("profiles.test-moe.model", "Qwen3.6-35B-A3B-Q8_0.gguf")
	viper.Set("profiles.test-moe.type", "moe")
	viper.Set("llama_server.moe_port", 8091)

	port := resolveProfilePort("test-moe")
	if port != 8091 {
		t.Errorf("When MoE profile, port should be 8091, got %d", port)
	}
}

func TestResolveProfilePort_CustomOverride(t *testing.T) {
	viper.Reset()
	defer viper.Reset()

	viper.Set("profiles.custom.model", "model.gguf")
	viper.Set("profiles.custom.port", 9000)

	port := resolveProfilePort("custom")
	if port != 9000 {
		t.Errorf("When custom port, should return 9000, got %d", port)
	}
}

func TestCollectActivePorts_NoServers(t *testing.T) {
	viper.Reset()
	defer viper.Reset()

	viper.Set("llama_server.dense_port", 59870)
	viper.Set("llama_server.moe_port", 59871)
	viper.Set("llama_server.host", "http://localhost:59870")

	ports := collectActivePorts()
	if len(ports) != 0 {
		t.Errorf("When no servers, collectActivePorts should return empty, got %d", len(ports))
	}
}

func TestNewShowPerfCmd_Registration(t *testing.T) {
	cmd := NewShowCmd()
	found := false
	for _, sub := range cmd.Commands() {
		if sub.Name() == "perf" {
			found = true
			break
		}
	}
	if !found {
		t.Error("When listing show subcommands, perf should be registered")
	}
}

func TestBenchPort_FullCycle(t *testing.T) {
	srv := newTestServer("bench-model")
	defer srv.Close()
	port := extractTestPort(t, srv)

	result := benchPort(port, "test-profile")

	if result.Error != "" {
		t.Errorf("When server responds, benchPort should not error, got: %s", result.Error)
	}
	if result.Model != "bench-model" {
		t.Errorf("When server returns model, got %q, want bench-model", result.Model)
	}
	if result.TTFT <= 0 {
		t.Error("When server responds, TTFT should be positive")
	}
	if result.NoThink.GenerationTokPerSec != 10.0 {
		t.Errorf("When server reports 10 tok/s no-think gen, got %.1f", result.NoThink.GenerationTokPerSec)
	}
	if result.Think.GenerationTokPerSec != 10.0 {
		t.Errorf("When server reports 10 tok/s think gen, got %.1f", result.Think.GenerationTokPerSec)
	}
}

func extractTestPort(t *testing.T, srv *httptest.Server) int {
	t.Helper()
	addr := srv.Listener.Addr().String()
	parts := strings.Split(addr, ":")
	port, err := strconv.Atoi(parts[len(parts)-1])
	if err != nil {
		t.Fatalf("could not extract port from %s: %v", addr, err)
	}
	return port
}

func TestPrintPerfResults_NoError(t *testing.T) {
	results := []perfResult{
		{
			Port:    8090,
			Profile: "test",
			Model:   "model.gguf",
			TTFT:    150 * time.Millisecond,
			NoThink: benchResult{PromptTokPerSec: 30.0, GenerationTokPerSec: 10.0, PromptTokens: 15, GeneratedTokens: 20},
			Think:   benchResult{PromptTokPerSec: 25.0, GenerationTokPerSec: 7.0, PromptTokens: 15, GeneratedTokens: 40},
		},
	}
	printPerfResults(results)
}
