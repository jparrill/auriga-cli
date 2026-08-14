package show

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/jparrill/auriga-cli/internal/llamaserver"
	"github.com/jparrill/auriga-cli/internal/ui"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func newShowPerfCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "perf [profile-name]",
		Short: "Quick performance test for running llama-server instances",
		Long: `Send test prompts to each running llama-server instance and report:
  - TTFT: Time to first token (prompt processing latency)
  - No-think: Generation speed with thinking disabled
  - Think: Generation speed with thinking enabled

If a profile name is given, only test that profile's port.

Examples:
  auriga show perf              # Test all running instances
  auriga show perf qwen3.8-27b  # Test specific profile`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 1 {
				return runPerfForProfile(args[0])
			}
			return runPerfAll()
		},
	}
}

type perfResult struct {
	Port      int
	Profile   string
	ModelType string
	Model     string
	TTFT      time.Duration
	NoThink   benchResult
	Think     benchResult
	Error     string
}

type benchResult struct {
	PromptTokPerSec     float64
	GenerationTokPerSec float64
	PromptTokens        int
	GeneratedTokens     int
	TotalMs             float64
	Error               string
}

type chatResponse struct {
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

func runPerfAll() error {
	ports := collectActivePorts()
	if len(ports) == 0 {
		ui.Warn("No running llama-server instances found")
		return nil
	}

	var results []perfResult
	for _, p := range ports {
		r := benchPort(p.port, p.profile)
		results = append(results, r)
	}

	printPerfResults(results)
	return nil
}

func runPerfForProfile(name string) error {
	profileKey := fmt.Sprintf("profiles.%s", name)
	model := viper.GetString(profileKey + ".model")
	if model == "" {
		return fmt.Errorf("profile %q not found", name)
	}

	port := resolveProfilePort(name)
	r := benchPort(port, name)
	printPerfResults([]perfResult{r})
	return nil
}

type portInfo struct {
	port    int
	profile string
}

func collectActivePorts() []portInfo {
	var active []portInfo
	seen := map[int]bool{}

	for _, port := range []int{llamaserver.DensePort(), llamaserver.MoePort()} {
		if seen[port] {
			continue
		}
		seen[port] = true
		if isPortHealthy(port) {
			profile := resolveProfileForPort(port)
			active = append(active, portInfo{port: port, profile: profile})
		}
	}

	profiles := viper.GetStringMap("profiles")
	for name := range profiles {
		port := resolveProfilePort(name)
		if seen[port] {
			continue
		}
		seen[port] = true
		if isPortHealthy(port) {
			active = append(active, portInfo{port: port, profile: name})
		}
	}

	return active
}

func resolveProfilePort(name string) int {
	profileKey := fmt.Sprintf("profiles.%s", name)
	if p := viper.GetInt(profileKey + ".port"); p > 0 {
		return p
	}
	t := viper.GetString(profileKey + ".type")
	if t == "" {
		model := viper.GetString(profileKey + ".model")
		t = detectModelTypeShow(model)
	}
	if t == "moe" {
		return llamaserver.MoePort()
	}
	return llamaserver.DensePort()
}

func resolveProfileForPort(port int) string {
	runningModel := getRunningModel(port)
	profiles := viper.GetStringMap("profiles")
	for name := range profiles {
		if resolveProfilePort(name) == port {
			if runningModel == "" {
				return name
			}
			if viper.GetString(fmt.Sprintf("profiles.%s.model", name)) == runningModel {
				return name
			}
		}
	}
	return fmt.Sprintf("port-%d", port)
}

func getRunningModel(port int) string {
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(fmt.Sprintf("http://localhost:%d/v1/models", port))
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return ""
	}
	var result struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if json.Unmarshal(body, &result) == nil && len(result.Data) > 0 {
		return result.Data[0].ID
	}
	return ""
}

func isPortHealthy(port int) bool {
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(fmt.Sprintf("http://localhost:%d/health", port))
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

func benchPort(port int, profile string) perfResult {
	pType := viper.GetString(fmt.Sprintf("profiles.%s.type", profile))
	if pType == "" {
		pType = detectModelTypeShow(viper.GetString(fmt.Sprintf("profiles.%s.model", profile)))
	}
	result := perfResult{Port: port, Profile: profile, ModelType: pType}

	ui.Info(fmt.Sprintf("Testing %s (port %d)...", profile, port))

	// TTFT: measure time to first token via streaming
	ttft, model := measureTTFT(port)
	result.TTFT = ttft
	result.Model = model

	// No-think test
	ui.Info("  without thinking...")
	result.NoThink = runBench(port, false)

	// Think test
	ui.Info("  with thinking...")
	result.Think = runBench(port, true)

	return result
}

func measureTTFT(port int) (time.Duration, string) {
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

	// Get model name from non-streaming request
	payload["stream"] = false
	body, _ = json.Marshal(payload)
	req2, _ := http.NewRequestWithContext(context.Background(), "POST", host+"/v1/chat/completions", bytes.NewReader(body))
	req2.Header.Set("Content-Type", "application/json")
	resp2, err := client.Do(req2)
	model := ""
	if err == nil {
		defer resp2.Body.Close()
		respBody, _ := io.ReadAll(resp2.Body)
		var cr chatResponse
		if json.Unmarshal(respBody, &cr) == nil {
			model = cr.Model
		}
	}

	return ttft, model
}

func runBench(port int, thinking bool) benchResult {
	host := fmt.Sprintf("http://localhost:%d", port)

	payload := map[string]any{
		"messages":   []map[string]string{{"role": "user", "content": "Write a haiku about mountains."}},
		"max_tokens": 80,
	}
	if !thinking {
		payload["chat_template_kwargs"] = map[string]bool{"enable_thinking": false}
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return benchResult{Error: err.Error()}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "POST", host+"/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return benchResult{Error: err.Error()}
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	start := time.Now()
	resp, err := client.Do(req)
	wallTime := time.Since(start)
	if err != nil {
		return benchResult{Error: err.Error()}
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return benchResult{Error: err.Error()}
	}

	var cr chatResponse
	if err := json.Unmarshal(respBody, &cr); err != nil {
		return benchResult{Error: fmt.Sprintf("parse error: %s", err)}
	}

	return benchResult{
		PromptTokPerSec:     cr.Timings.PromptPerSecond,
		GenerationTokPerSec: cr.Timings.PredictedPerSec,
		PromptTokens:        cr.Timings.PromptN,
		GeneratedTokens:     cr.Timings.PredictedN,
		TotalMs:             float64(wallTime.Milliseconds()),
	}
}

func printPerfResults(results []perfResult) {
	fmt.Println()
	tbl := ui.NewTable("Performance",
		"PROFILE", "TYPE", "PORT", "MODEL",
		"TTFT",
		"NO-THINK PROMPT", "NO-THINK GEN",
		"THINK PROMPT", "THINK GEN",
	)

	for _, r := range results {
		if r.Error != "" {
			tbl.AddRow(r.Profile, "-", fmt.Sprintf("%d", r.Port), "-", "-", "-", "-", "-", "-")
			continue
		}

		ttft := fmt.Sprintf("%dms", r.TTFT.Milliseconds())

		noThinkPrompt := fmtTokS(r.NoThink.PromptTokPerSec, r.NoThink.Error)
		noThinkGen := fmtTokS(r.NoThink.GenerationTokPerSec, r.NoThink.Error)
		thinkPrompt := fmtTokS(r.Think.PromptTokPerSec, r.Think.Error)
		thinkGen := fmtTokS(r.Think.GenerationTokPerSec, r.Think.Error)

		tbl.AddRow(
			r.Profile,
			r.ModelType,
			fmt.Sprintf("%d", r.Port),
			r.Model,
			ttft,
			noThinkPrompt, noThinkGen,
			thinkPrompt, thinkGen,
		)
	}
	tbl.Print()
}

func fmtTokS(tokPerSec float64, err string) string {
	if err != "" {
		return "error"
	}
	return fmt.Sprintf("%.1f tok/s", tokPerSec)
}
