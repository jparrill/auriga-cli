package perf

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
)

func newBenchServer(model string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/health":
			w.WriteHeader(http.StatusOK)
		case r.URL.Path == "/v1/chat/completions":
			resp := map[string]any{
				"model": model,
				"choices": []map[string]any{
					{"message": map[string]string{"role": "assistant", "content": "test response"}},
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

func extractPort(t *testing.T, srv *httptest.Server) int {
	t.Helper()
	addr := srv.Listener.Addr().String()
	parts := strings.Split(addr, ":")
	port, err := strconv.Atoi(parts[len(parts)-1])
	if err != nil {
		t.Fatalf("could not extract port from %s: %v", addr, err)
	}
	return port
}

func TestRunSingleBench_Success(t *testing.T) {
	srv := newBenchServer("test-model")
	defer srv.Close()
	port := extractPort(t, srv)

	result := RunSingleBench(port, false)

	if result.Error != "" {
		t.Errorf("When server responds, RunSingleBench should not error, got: %s", result.Error)
	}
	if result.GenerationTokPerSec != 10.0 {
		t.Errorf("expected 10 tok/s gen, got %.1f", result.GenerationTokPerSec)
	}
	if result.PromptTokPerSec != 30.0 {
		t.Errorf("expected 30 tok/s prompt, got %.1f", result.PromptTokPerSec)
	}
	if result.PromptTokens != 15 {
		t.Errorf("expected 15 prompt tokens, got %d", result.PromptTokens)
	}
	if result.GeneratedTokens != 20 {
		t.Errorf("expected 20 generated tokens, got %d", result.GeneratedTokens)
	}
}

func TestRunSingleBench_Thinking(t *testing.T) {
	srv := newBenchServer("test-model")
	defer srv.Close()
	port := extractPort(t, srv)

	result := RunSingleBench(port, true)

	if result.Error != "" {
		t.Errorf("When thinking enabled, RunSingleBench should not error, got: %s", result.Error)
	}
}

func TestRunSingleBench_ServerDown(t *testing.T) {
	result := RunSingleBench(59899, false)

	if result.Error == "" {
		t.Error("When server down, RunSingleBench should return error")
	}
}

func TestRunBench_MedianCalculation(t *testing.T) {
	srv := newBenchServer("test-model")
	defer srv.Close()
	port := extractPort(t, srv)

	result := RunBench(port, false)

	if result.Error != "" {
		t.Errorf("RunBench should not error, got: %s", result.Error)
	}
	if result.GenerationTokPerSec != 10.0 {
		t.Errorf("When all samples identical, median should be 10.0, got %.1f", result.GenerationTokPerSec)
	}
	if result.GenMin != 10.0 || result.GenMax != 10.0 {
		t.Errorf("When all samples identical, min/max should equal median, got min=%.1f max=%.1f", result.GenMin, result.GenMax)
	}
	if result.PromptTokPerSec != 30.0 {
		t.Errorf("When all samples identical, prompt median should be 30.0, got %.1f", result.PromptTokPerSec)
	}
}

func TestRunBenchN_CustomIterations(t *testing.T) {
	srv := newBenchServer("test-model")
	defer srv.Close()
	port := extractPort(t, srv)

	result := RunBenchN(port, false, 3)

	if result.Error != "" {
		t.Errorf("RunBenchN should not error, got: %s", result.Error)
	}
	if result.GenerationTokPerSec != 10.0 {
		t.Errorf("expected median 10.0, got %.1f", result.GenerationTokPerSec)
	}
}

func TestRunBench_ServerDown(t *testing.T) {
	result := RunBench(59898, false)

	if result.Error == "" {
		t.Error("When server down, RunBench should return error")
	}
}

func TestMeasureTTFT_Success(t *testing.T) {
	srv := newBenchServer("test-model")
	defer srv.Close()
	port := extractPort(t, srv)

	ttft, model := MeasureTTFT(port)

	if ttft <= 0 {
		t.Error("When server responds, TTFT should be positive")
	}
	if model != "test-model" {
		t.Errorf("expected model 'test-model', got %q", model)
	}
}

func TestMeasureTTFT_ServerDown(t *testing.T) {
	ttft, model := MeasureTTFT(59897)

	if ttft != 0 {
		t.Errorf("When server down, TTFT should be 0, got %v", ttft)
	}
	if model != "" {
		t.Errorf("When server down, model should be empty, got %q", model)
	}
}

func TestMedian_Odd(t *testing.T) {
	got := Median([]float64{1, 3, 5, 7, 9})
	if got != 5 {
		t.Errorf("median of [1,3,5,7,9] should be 5, got %.1f", got)
	}
}

func TestMedian_Even(t *testing.T) {
	got := Median([]float64{1, 3, 5, 7})
	if got != 4 {
		t.Errorf("median of [1,3,5,7] should be 4, got %.1f", got)
	}
}

func TestMedian_Single(t *testing.T) {
	got := Median([]float64{42})
	if got != 42 {
		t.Errorf("median of [42] should be 42, got %.1f", got)
	}
}

func TestMedian_Empty(t *testing.T) {
	got := Median([]float64{})
	if got != 0 {
		t.Errorf("median of empty should be 0, got %.1f", got)
	}
}

func TestRunSingleBench_InvalidJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "not json")
	}))
	defer srv.Close()
	port := extractPort(t, srv)

	result := RunSingleBench(port, false)
	if result.Error == "" {
		t.Error("When server returns invalid JSON, should return error")
	}
}
