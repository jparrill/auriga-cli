package perf

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestParsePerplexityOutput_Valid(t *testing.T) {
	output := `perplexity : calculating perplexity over 70 chunks
24.13 seconds per pass - ETA 28.15 minutes
[1]8.0756,[2]7.4543,[3]7.6925,[4]7.4525
Final estimate: PPL = 6.7794 +/- 0.0500`

	result, err := ParsePerplexityOutput(output)
	if err != nil {
		t.Fatalf("When output valid, parse should succeed, got: %v", err)
	}
	if result.Score != 6.7794 {
		t.Errorf("When PPL is 6.7794, score should match, got %.4f", result.Score)
	}
	if result.StdDev != 0.05 {
		t.Errorf("When stddev is 0.0500, should match, got %.4f", result.StdDev)
	}
}

func TestParsePerplexityOutput_NoMatch(t *testing.T) {
	output := "some random output without perplexity data"

	_, err := ParsePerplexityOutput(output)
	if err == nil {
		t.Error("When output has no PPL line, parse should error")
	}
}

func TestParsePerplexityOutput_LargeScore(t *testing.T) {
	output := "Final estimate: PPL = 12.3456 +/- 1.2345"

	result, err := ParsePerplexityOutput(output)
	if err != nil {
		t.Fatalf("When output valid with large score, parse should succeed, got: %v", err)
	}
	if result.Score != 12.3456 {
		t.Errorf("expected 12.3456, got %.4f", result.Score)
	}
	if result.StdDev != 1.2345 {
		t.Errorf("expected 1.2345, got %.4f", result.StdDev)
	}
}

func TestLoadSavePerplexityCache(t *testing.T) {
	tmpDir := t.TempDir()
	cachePath := filepath.Join(tmpDir, "cache.json")

	origDefault := DefaultCachePath
	DefaultCachePath = cachePath
	defer func() { DefaultCachePath = origDefault }()

	cache := map[string]PerplexityResult{
		"model-q5.gguf": {
			Score:   6.78,
			StdDev:  0.05,
			Model:   "model-q5.gguf",
			Dataset: "wikitext-2",
			RunAt:   time.Date(2026, 9, 14, 12, 0, 0, 0, time.UTC),
		},
	}

	err := SavePerplexityCache(cache)
	if err != nil {
		t.Fatalf("When saving cache, should succeed, got: %v", err)
	}

	loaded, err := LoadPerplexityCache()
	if err != nil {
		t.Fatalf("When loading cache, should succeed, got: %v", err)
	}

	if len(loaded) != 1 {
		t.Fatalf("When cache has 1 entry, loaded should have 1, got %d", len(loaded))
	}

	r, ok := loaded["model-q5.gguf"]
	if !ok {
		t.Fatal("When cache has model-q5.gguf, loaded should contain it")
	}
	if r.Score != 6.78 {
		t.Errorf("When cached score is 6.78, loaded should match, got %.2f", r.Score)
	}
}

func TestLoadPerplexityCache_Missing(t *testing.T) {
	origDefault := DefaultCachePath
	DefaultCachePath = "/tmp/nonexistent-perplexity-cache-test.json"
	defer func() { DefaultCachePath = origDefault }()

	cache, err := LoadPerplexityCache()
	if err != nil {
		t.Fatalf("When cache file missing, should return empty map, got error: %v", err)
	}
	if len(cache) != 0 {
		t.Errorf("When cache file missing, should return empty map, got %d entries", len(cache))
	}
}

func TestLoadPerplexityCache_InvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	cachePath := filepath.Join(tmpDir, "bad.json")

	origDefault := DefaultCachePath
	DefaultCachePath = cachePath
	defer func() { DefaultCachePath = origDefault }()

	os.WriteFile(cachePath, []byte("not json"), 0o644)

	_, err := LoadPerplexityCache()
	if err == nil {
		t.Error("When cache has invalid JSON, should error")
	}
}

func TestGetCachedPerplexity_Hit(t *testing.T) {
	tmpDir := t.TempDir()
	cachePath := filepath.Join(tmpDir, "cache.json")

	origDefault := DefaultCachePath
	DefaultCachePath = cachePath
	defer func() { DefaultCachePath = origDefault }()

	cache := map[string]PerplexityResult{
		"test-model.gguf": {Score: 7.5, StdDev: 0.1, Model: "test-model.gguf"},
	}
	data, _ := json.Marshal(cache)
	os.WriteFile(cachePath, data, 0o644)

	result, ok := GetCachedPerplexity("test-model.gguf")
	if !ok {
		t.Fatal("When model in cache, GetCachedPerplexity should return true")
	}
	if result.Score != 7.5 {
		t.Errorf("When cached score is 7.5, got %.1f", result.Score)
	}
}

func TestGetCachedPerplexity_Miss(t *testing.T) {
	tmpDir := t.TempDir()
	cachePath := filepath.Join(tmpDir, "cache.json")

	origDefault := DefaultCachePath
	DefaultCachePath = cachePath
	defer func() { DefaultCachePath = origDefault }()

	os.WriteFile(cachePath, []byte("{}"), 0o644)

	_, ok := GetCachedPerplexity("missing-model.gguf")
	if ok {
		t.Error("When model not in cache, GetCachedPerplexity should return false")
	}
}

func TestGetCachedPerplexity_FullPath(t *testing.T) {
	tmpDir := t.TempDir()
	cachePath := filepath.Join(tmpDir, "cache.json")

	origDefault := DefaultCachePath
	DefaultCachePath = cachePath
	defer func() { DefaultCachePath = origDefault }()

	cache := map[string]PerplexityResult{
		"test-model.gguf": {Score: 5.5, StdDev: 0.1, Model: "test-model.gguf"},
	}
	data, _ := json.Marshal(cache)
	os.WriteFile(cachePath, data, 0o644)

	result, ok := GetCachedPerplexity("/home/user/models/gguf/test-model.gguf")
	if !ok {
		t.Fatal("When full path given, should match by basename")
	}
	if result.Score != 5.5 {
		t.Errorf("When cached score is 5.5, got %.1f", result.Score)
	}
}

func TestPerplexityBinExists_Missing(t *testing.T) {
	origDefault := DefaultPerplexityBin
	DefaultPerplexityBin = "/nonexistent/path/llama-perplexity"
	defer func() { DefaultPerplexityBin = origDefault }()

	if PerplexityBinExists() {
		t.Error("When binary does not exist, PerplexityBinExists should return false")
	}
}

func TestDatasetExists_Missing(t *testing.T) {
	origDefault := DefaultDatasetPath
	DefaultDatasetPath = "/nonexistent/path/wiki.test.raw"
	defer func() { DefaultDatasetPath = origDefault }()

	if DatasetExists() {
		t.Error("When dataset does not exist, DatasetExists should return false")
	}
}

func TestRunPerplexity_BinaryMissing(t *testing.T) {
	origDefault := DefaultPerplexityBin
	DefaultPerplexityBin = "/nonexistent/llama-perplexity"
	defer func() { DefaultPerplexityBin = origDefault }()

	_, err := RunPerplexity("/some/model.gguf")
	if err == nil {
		t.Error("When binary missing, RunPerplexity should error")
	}
}

func TestRunPerplexity_DatasetMissing(t *testing.T) {
	tmpBin := filepath.Join(t.TempDir(), "llama-perplexity")
	os.WriteFile(tmpBin, []byte("#!/bin/sh\n"), 0o755)

	origBin := DefaultPerplexityBin
	origDS := DefaultDatasetPath
	DefaultPerplexityBin = tmpBin
	DefaultDatasetPath = "/nonexistent/wiki.test.raw"
	defer func() {
		DefaultPerplexityBin = origBin
		DefaultDatasetPath = origDS
	}()

	_, err := RunPerplexity("/some/model.gguf")
	if err == nil {
		t.Error("When dataset missing, RunPerplexity should error")
	}
}

func TestRunPerplexity_ModelMissing(t *testing.T) {
	tmpBin := filepath.Join(t.TempDir(), "llama-perplexity")
	os.WriteFile(tmpBin, []byte("#!/bin/sh\n"), 0o755)

	tmpDS := filepath.Join(t.TempDir(), "wiki.test.raw")
	os.WriteFile(tmpDS, []byte("test data"), 0o644)

	origBin := DefaultPerplexityBin
	origDS := DefaultDatasetPath
	DefaultPerplexityBin = tmpBin
	DefaultDatasetPath = tmpDS
	defer func() {
		DefaultPerplexityBin = origBin
		DefaultDatasetPath = origDS
	}()

	_, err := RunPerplexity("/nonexistent/model.gguf")
	if err == nil {
		t.Error("When model missing, RunPerplexity should error")
	}
}

func TestPplRegex(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		score  float64
		stddev float64
	}{
		{"standard", "Final estimate: PPL = 6.7794 +/- 0.0500", 6.7794, 0.05},
		{"high ppl", "Final estimate: PPL = 15.1234 +/- 2.5678", 15.1234, 2.5678},
		{"low ppl", "Final estimate: PPL = 3.14 +/- 0.01", 3.14, 0.01},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matches := pplRegex.FindStringSubmatch(tt.input)
			if len(matches) < 3 {
				t.Fatal("regex should match")
			}
		})
	}
}

func TestSavePerplexityCache_CreatesDir(t *testing.T) {
	tmpDir := filepath.Join(t.TempDir(), "nested", "dir")
	cachePath := filepath.Join(tmpDir, "cache.json")

	origDefault := DefaultCachePath
	DefaultCachePath = cachePath
	defer func() { DefaultCachePath = origDefault }()

	cache := map[string]PerplexityResult{
		"test.gguf": {Score: 5.0},
	}

	err := SavePerplexityCache(cache)
	if err != nil {
		t.Fatalf("When saving to nested dir, should create dir and succeed, got: %v", err)
	}

	if _, err := os.Stat(cachePath); err != nil {
		t.Errorf("When saved, cache file should exist, got: %v", err)
	}
}
