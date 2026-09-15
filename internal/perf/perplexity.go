package perf

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	osexec "os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"time"

	"github.com/jparrill/auriga-cli/internal/config"
	"github.com/jparrill/auriga-cli/internal/exec"
	"github.com/spf13/viper"
)

var (
	DefaultPerplexityBin = "~/infra/bin/llama-perplexity"
	DefaultDatasetPath   = "~/infra/ai/models/datasets/wikitext-2-raw/wiki.test.raw"
	DefaultCachePath     = "~/.config/auriga/perplexity-cache.json"
)

var pplRegex = regexp.MustCompile(`Final estimate: PPL = ([0-9.]+) \+/- ([0-9.]+)`)

type PerplexityResult struct {
	Score   float64   `json:"score"`
	StdDev  float64   `json:"std_dev"`
	Model   string    `json:"model"`
	Dataset string    `json:"dataset"`
	RunAt   time.Time `json:"run_at"`
}

func PerplexityBinPath() string {
	if v := viper.GetString("llama_server.perplexity.bin"); v != "" {
		return config.ExpandHome(v)
	}
	return config.ExpandHome(DefaultPerplexityBin)
}

func DatasetPath() string {
	if v := viper.GetString("llama_server.perplexity.dataset"); v != "" {
		return config.ExpandHome(v)
	}
	return config.ExpandHome(DefaultDatasetPath)
}

func CachePath() string {
	if v := viper.GetString("llama_server.perplexity.cache"); v != "" {
		return config.ExpandHome(v)
	}
	return config.ExpandHome(DefaultCachePath)
}

func PerplexityBinExists() bool {
	_, err := osexec.LookPath(PerplexityBinPath())
	if err == nil {
		return true
	}
	_, err = os.Stat(PerplexityBinPath())
	return err == nil
}

func DatasetExists() bool {
	_, err := os.Stat(DatasetPath())
	return err == nil
}

func LoadPerplexityCache() (map[string]PerplexityResult, error) {
	path := CachePath()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return make(map[string]PerplexityResult), nil
		}
		return nil, fmt.Errorf("read cache: %w", err)
	}

	var cache map[string]PerplexityResult
	if err := json.Unmarshal(data, &cache); err != nil {
		return nil, fmt.Errorf("parse cache: %w", err)
	}
	return cache, nil
}

func SavePerplexityCache(cache map[string]PerplexityResult) error {
	path := CachePath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create cache dir: %w", err)
	}

	data, err := json.MarshalIndent(cache, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal cache: %w", err)
	}
	return os.WriteFile(path, data, 0o644)
}

func GetCachedPerplexity(model string) (PerplexityResult, bool) {
	cache, err := LoadPerplexityCache()
	if err != nil {
		return PerplexityResult{}, false
	}
	key := filepath.Base(model)
	r, ok := cache[key]
	return r, ok
}

func RunPerplexity(modelPath string) (PerplexityResult, error) {
	binPath := PerplexityBinPath()
	if !PerplexityBinExists() {
		return PerplexityResult{}, fmt.Errorf("llama-perplexity binary not found at %s", binPath)
	}

	datasetPath := DatasetPath()
	if !DatasetExists() {
		return PerplexityResult{}, fmt.Errorf("wikitext-2 dataset not found at %s", datasetPath)
	}

	expandedModel := config.ExpandHome(modelPath)
	if _, err := os.Stat(expandedModel); err != nil {
		return PerplexityResult{}, fmt.Errorf("model not found at %s", expandedModel)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	args := []string{
		"-m", expandedModel,
		"-f", datasetPath,
	}

	out, err := exec.RunCapture(ctx, binPath, args, exec.RunOpts{})
	if err != nil {
		return PerplexityResult{}, fmt.Errorf("llama-perplexity failed: %w", err)
	}

	result, err := ParsePerplexityOutput(out)
	if err != nil {
		return PerplexityResult{}, err
	}

	result.Model = filepath.Base(expandedModel)
	result.Dataset = "wikitext-2"
	result.RunAt = time.Now()

	cache, _ := LoadPerplexityCache()
	if cache == nil {
		cache = make(map[string]PerplexityResult)
	}
	cache[result.Model] = result
	if saveErr := SavePerplexityCache(cache); saveErr != nil {
		return result, fmt.Errorf("perplexity measured (%.4f) but cache save failed: %w", result.Score, saveErr)
	}

	return result, nil
}

func ParsePerplexityOutput(output string) (PerplexityResult, error) {
	matches := pplRegex.FindStringSubmatch(output)
	if len(matches) < 3 {
		return PerplexityResult{}, fmt.Errorf("could not parse perplexity output: no 'Final estimate: PPL = X +/- Y' found")
	}

	score, err := strconv.ParseFloat(matches[1], 64)
	if err != nil {
		return PerplexityResult{}, fmt.Errorf("parse perplexity score: %w", err)
	}

	stddev, err := strconv.ParseFloat(matches[2], 64)
	if err != nil {
		return PerplexityResult{}, fmt.Errorf("parse perplexity stddev: %w", err)
	}

	return PerplexityResult{Score: score, StdDev: stddev}, nil
}
