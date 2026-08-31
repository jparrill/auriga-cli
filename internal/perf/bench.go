package perf

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"time"
)

const (
	Iterations = 5
	MaxTokens  = 256
)

var Prompt = `Analyze this Go code for bugs, performance issues, and improvements. Be thorough:

func processItems(items []Item) ([]Result, error) {
    var results []Result
    for i := 0; i < len(items); i++ {
        item := items[i]
        if item.Type == "" {
            continue
        }
        data, err := fetchData(item.ID)
        if err != nil {
            log.Printf("failed to fetch %s: %v", item.ID, err)
            continue
        }
        result, err := transform(data, item.Config)
        if err != nil {
            return nil, fmt.Errorf("transform failed for %s: %w", item.ID, err)
        }
        if result.Score > threshold {
            results = append(results, result)
        }
    }
    if len(results) == 0 {
        return nil, fmt.Errorf("no results met threshold")
    }
    return results, nil
}`

type BenchResult struct {
	PromptTokPerSec     float64
	GenerationTokPerSec float64
	GenMin              float64
	GenMax              float64
	PromptTokens        int
	GeneratedTokens     int
	Error               string
}

type ChatResponse struct {
	Model   string `json:"model"`
	Choices []struct {
		Message struct {
			Content          string `json:"content"`
			ReasoningContent string `json:"reasoning_content"`
		} `json:"message"`
	} `json:"choices"`
	Timings struct {
		PromptN         int     `json:"prompt_n"`
		PromptMs        float64 `json:"prompt_ms"`
		PromptPerSecond float64 `json:"prompt_per_second"`
		PredictedN      int     `json:"predicted_n"`
		PredictedMs     float64 `json:"predicted_ms"`
		PredictedPerSec float64 `json:"predicted_per_second"`
	} `json:"timings"`
}

func RunBench(port int, thinking bool) BenchResult {
	return RunBenchN(port, thinking, Iterations)
}

func RunBenchN(port int, thinking bool, iterations int) BenchResult {
	var genSamples, promptSamples []float64
	var last BenchResult

	for i := 0; i < iterations; i++ {
		r := RunSingleBench(port, thinking)
		if r.Error != "" {
			return r
		}
		genSamples = append(genSamples, r.GenerationTokPerSec)
		promptSamples = append(promptSamples, r.PromptTokPerSec)
		last = r
	}

	sort.Float64s(genSamples)
	sort.Float64s(promptSamples)

	return BenchResult{
		PromptTokPerSec:     Median(promptSamples),
		GenerationTokPerSec: Median(genSamples),
		GenMin:              genSamples[0],
		GenMax:              genSamples[len(genSamples)-1],
		PromptTokens:        last.PromptTokens,
		GeneratedTokens:     last.GeneratedTokens,
	}
}

func RunSingleBench(port int, thinking bool) BenchResult {
	host := fmt.Sprintf("http://localhost:%d", port)

	payload := map[string]any{
		"messages":   []map[string]string{{"role": "user", "content": Prompt}},
		"max_tokens": MaxTokens,
	}
	if !thinking {
		payload["chat_template_kwargs"] = map[string]bool{"enable_thinking": false}
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return BenchResult{Error: err.Error()}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "POST", host+"/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return BenchResult{Error: err.Error()}
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return BenchResult{Error: err.Error()}
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return BenchResult{Error: err.Error()}
	}

	var cr ChatResponse
	if err := json.Unmarshal(respBody, &cr); err != nil {
		return BenchResult{Error: fmt.Sprintf("parse error: %s", err)}
	}

	return BenchResult{
		PromptTokPerSec:     cr.Timings.PromptPerSecond,
		GenerationTokPerSec: cr.Timings.PredictedPerSec,
		PromptTokens:        cr.Timings.PromptN,
		GeneratedTokens:     cr.Timings.PredictedN,
	}
}

func MeasureTTFT(port int) (time.Duration, string) {
	host := fmt.Sprintf("http://localhost:%d", port)

	payload := map[string]any{
		"messages":             []map[string]string{{"role": "user", "content": "Hi"}},
		"max_tokens":           5,
		"stream":               true,
		"chat_template_kwargs": map[string]bool{"enable_thinking": false},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return 0, ""
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "POST", host+"/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return 0, ""
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	start := time.Now()
	resp, err := client.Do(req)
	if err != nil {
		return 0, ""
	}
	ttft := time.Since(start)
	resp.Body.Close()

	payload["stream"] = false
	body, _ = json.Marshal(payload)
	req2, _ := http.NewRequestWithContext(context.Background(), "POST", host+"/v1/chat/completions", bytes.NewReader(body))
	req2.Header.Set("Content-Type", "application/json")
	resp2, err := client.Do(req2)
	model := ""
	if err == nil {
		defer resp2.Body.Close()
		respBody, _ := io.ReadAll(resp2.Body)
		var cr ChatResponse
		if json.Unmarshal(respBody, &cr) == nil {
			model = cr.Model
		}
	}

	return ttft, model
}

func Median(sorted []float64) float64 {
	n := len(sorted)
	if n == 0 {
		return 0
	}
	if n%2 == 0 {
		return (sorted[n/2-1] + sorted[n/2]) / 2
	}
	return sorted[n/2]
}
