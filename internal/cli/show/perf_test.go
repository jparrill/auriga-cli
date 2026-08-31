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

	"github.com/jparrill/auriga-cli/internal/perf"
	"github.com/spf13/viper"
)

func newTestServer(model string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/health":
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, `{"status":"ok"}`)
		case r.URL.Path == "/v1/models":
			resp := map[string]any{
				"data": []map[string]string{{"id": model}},
			}
			json.NewEncoder(w).Encode(resp)
		case r.URL.Path == "/v1/chat/completions":
			var req map[string]any
			json.NewDecoder(r.Body).Decode(&req)

			resp := map[string]any{
				"model": model,
				"choices": []map[string]any{
					{"message": map[string]string{"role": "assistant", "content": "Mountains rise high"}},
				},
				"timings": map[string]any{
					"prompt_n":            15,
					"prompt_ms":           500.0,
					"prompt_per_second":   30.0,
					"predicted_n":         20,
					"predicted_ms":        2000.0,
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

	result := perf.RunBench(port, false)

	if result.Error != "" {
		t.Errorf("When server responds, RunBench should not error, got: %s", result.Error)
	}
	if result.GenerationTokPerSec != 10.0 {
		t.Errorf("When server reports 10 tok/s, median should be 10.0, got %.1f", result.GenerationTokPerSec)
	}
	if result.GenMin != 10.0 || result.GenMax != 10.0 {
		t.Errorf("When all samples identical, min/max should equal median, got min=%.1f max=%.1f", result.GenMin, result.GenMax)
	}
}

func TestRunSingleBench_NoThinking(t *testing.T) {
	srv := newTestServer("test-model")
	defer srv.Close()
	port := extractTestPort(t, srv)

	result := perf.RunSingleBench(port, false)

	if result.Error != "" {
		t.Errorf("When server responds, RunSingleBench should not error, got: %s", result.Error)
	}
	if result.GenerationTokPerSec != 10.0 {
		t.Errorf("When server reports 10 tok/s, got %.1f", result.GenerationTokPerSec)
	}
	if result.PromptTokPerSec != 30.0 {
		t.Errorf("When server reports 30 tok/s prompt, got %.1f", result.PromptTokPerSec)
	}
}

func TestRunBench_ServerDown(t *testing.T) {
	result := perf.RunBench(59877, false)

	if result.Error == "" {
		t.Error("When server down, RunBench should return error")
	}
}

func TestMedian_Odd(t *testing.T) {
	got := perf.Median([]float64{1, 3, 5, 7, 9})
	if got != 5 {
		t.Errorf("median of [1,3,5,7,9] should be 5, got %.1f", got)
	}
}

func TestMedian_Even(t *testing.T) {
	got := perf.Median([]float64{1, 3, 5, 7})
	if got != 4 {
		t.Errorf("median of [1,3,5,7] should be 4, got %.1f", got)
	}
}

func TestMedian_Empty(t *testing.T) {
	got := perf.Median([]float64{})
	if got != 0 {
		t.Errorf("median of empty should be 0, got %.1f", got)
	}
}

func TestMeasureTTFT_ReturnsPositive(t *testing.T) {
	srv := newTestServer("test-model")
	defer srv.Close()
	port := extractTestPort(t, srv)

	ttft, model := perf.MeasureTTFT(port)

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
	if result.Binary == "" {
		t.Error("When benchPort runs, Binary should be set")
	}
	if result.TTFT <= 0 {
		t.Error("When server responds, TTFT should be positive")
	}
	if result.NoThink.GenerationTokPerSec != 10.0 {
		t.Errorf("When server reports 10 tok/s no-think gen, median got %.1f", result.NoThink.GenerationTokPerSec)
	}
	if result.Think.GenerationTokPerSec != 10.0 {
		t.Errorf("When server reports 10 tok/s think gen, median got %.1f", result.Think.GenerationTokPerSec)
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

func TestGetRunningModel_ReturnsModelID(t *testing.T) {
	srv := newTestServer("test-model.gguf")
	defer srv.Close()
	port := extractTestPort(t, srv)

	model := getRunningModel(port)
	if model != "test-model.gguf" {
		t.Errorf("When server returns model, getRunningModel should return it, got %q", model)
	}
}

func TestGetRunningModel_NoServer(t *testing.T) {
	model := getRunningModel(59878)
	if model != "" {
		t.Errorf("When no server, getRunningModel should return empty, got %q", model)
	}
}

func TestResolveProfileForPort_MatchesRunningModel(t *testing.T) {
	srv := newTestServer("correct-model.gguf")
	defer srv.Close()
	port := extractTestPort(t, srv)

	viper.Reset()
	defer viper.Reset()

	viper.Set("profiles.wrong-profile.model", "other-model.gguf")
	viper.Set("profiles.wrong-profile.port", port)
	viper.Set("profiles.correct-profile.model", "correct-model.gguf")
	viper.Set("profiles.correct-profile.port", port)
	viper.Set("llama_server.host", "http://localhost:8090")

	profile := resolveProfileForPort(port)
	if profile != "correct-profile" {
		t.Errorf("When multiple profiles on same port, should match running model, got %q", profile)
	}
}

func TestResolveProfileForPort_MatchesFullPathModel(t *testing.T) {
	srv := newTestServer("/home/user/models/gguf/Qwen3.6-35B-A3B-Q8_0.gguf")
	defer srv.Close()
	port := extractTestPort(t, srv)

	viper.Reset()
	defer viper.Reset()

	viper.Set("profiles.qwen-vision.model", "Qwen3.6-35B-A3B-Q8_0.gguf")
	viper.Set("profiles.qwen-vision.type", "moe")
	viper.Set("profiles.qwen-vision.port", port)
	viper.Set("llama_server.host", "http://localhost:8090")

	profile := resolveProfileForPort(port)
	if profile != "qwen-vision" {
		t.Errorf("When server returns full path, should match by basename, got %q", profile)
	}
}

func TestBenchPort_MoETypeFromRunningModel(t *testing.T) {
	srv := newTestServer("/home/user/models/gguf/Qwen3.6-35B-A3B-Q8_0.gguf")
	defer srv.Close()
	port := extractTestPort(t, srv)

	viper.Reset()
	defer viper.Reset()

	result := benchPort(port, fmt.Sprintf("port-%d", port))
	if result.ModelType != "moe" {
		t.Errorf("When running model has A3B pattern, type should be moe, got %q", result.ModelType)
	}
}

func TestResolveSpecType_DFlash(t *testing.T) {
	viper.Reset()
	defer viper.Reset()

	viper.Set("profiles.glimmer.dflash", "dflash-kquant.gguf")

	got := resolveSpecType("glimmer")
	if got != "dflash" {
		t.Errorf("When dflash set, spec should be dflash, got %q", got)
	}
}

func TestResolveSpecType_MTPFromFlags(t *testing.T) {
	viper.Reset()
	defer viper.Reset()

	viper.Set("profiles.qwen.flags", []string{"--spec-type", "draft-mtp", "--spec-draft-n-max", "2"})

	got := resolveSpecType("qwen")
	if got != "mtp" {
		t.Errorf("When draft-mtp in flags, spec should be mtp, got %q", got)
	}
}

func TestResolveSpecType_MTPFromDrafter(t *testing.T) {
	viper.Reset()
	defer viper.Reset()

	viper.Set("profiles.gemma.mtp_drafter", "mtp-gemma.gguf")

	got := resolveSpecType("gemma")
	if got != "mtp" {
		t.Errorf("When mtp_drafter set, spec should be mtp, got %q", got)
	}
}

func TestResolveSpecType_None(t *testing.T) {
	viper.Reset()
	defer viper.Reset()

	viper.Set("profiles.plain.model", "model.gguf")

	got := resolveSpecType("plain")
	if got != "-" {
		t.Errorf("When no spec config, should be -, got %q", got)
	}
}

func TestPrintPerfResults_NoError(t *testing.T) {
	results := []perfResult{
		{
			Port:    8090,
			Profile: "test",
			Binary:  "llama-server",
			Model:   "model.gguf",
			TTFT:    150 * time.Millisecond,
			NoThink: perf.BenchResult{PromptTokPerSec: 30.0, GenerationTokPerSec: 10.0, GenMin: 9.5, GenMax: 10.5, PromptTokens: 15, GeneratedTokens: 20},
			Think:   perf.BenchResult{PromptTokPerSec: 25.0, GenerationTokPerSec: 7.0, GenMin: 6.5, GenMax: 7.5, PromptTokens: 15, GeneratedTokens: 40},
		},
	}
	printPerfResults(results)
}

func TestFmtTokSRange_WithRange(t *testing.T) {
	b := perf.BenchResult{GenerationTokPerSec: 19.5, GenMin: 18.9, GenMax: 20.1}
	got := fmtTokSRange(b)
	if got != "19.5 (18.9-20.1)" {
		t.Errorf("fmtTokSRange with range = %q, want '19.5 (18.9-20.1)'", got)
	}
}

func TestFmtTokSRange_NoRange(t *testing.T) {
	b := perf.BenchResult{GenerationTokPerSec: 10.0, GenMin: 10.0, GenMax: 10.0}
	got := fmtTokSRange(b)
	if got != "10.0 tok/s" {
		t.Errorf("fmtTokSRange without range = %q, want '10.0 tok/s'", got)
	}
}

func TestFmtTokSRange_Error(t *testing.T) {
	b := perf.BenchResult{Error: "connection refused"}
	got := fmtTokSRange(b)
	if got != "error" {
		t.Errorf("fmtTokSRange with error = %q, want 'error'", got)
	}
}
