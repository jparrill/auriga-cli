package sweep

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/viper"
)

func TestValidateSweepConfig_ProfileChecks(t *testing.T) {
	viper.Reset()
	defer viper.Reset()

	t.Run("empty profile", func(t *testing.T) {
		cfg := SweepConfig{Profile: "", Iterations: 5}
		issues := validateSweepConfig(cfg)
		found := false
		for _, i := range issues {
			if i.Level == "error" && i.Check == "profile" {
				found = true
			}
		}
		if !found {
			t.Error("expected error with check=profile")
		}
	})

	t.Run("profile not found", func(t *testing.T) {
		cfg := SweepConfig{Profile: "nonexistent", Iterations: 5}
		issues := validateSweepConfig(cfg)
		found := false
		for _, i := range issues {
			if i.Level == "error" && i.Check == "profile" {
				found = true
			}
		}
		if !found {
			t.Error("expected error with check=profile")
		}
	})

	t.Run("valid profile", func(t *testing.T) {
		viper.Set("profiles.testmodel.model", "test.gguf")
		defer viper.Reset()

		cfg := SweepConfig{Profile: "testmodel", Iterations: 5}
		issues := validateSweepConfig(cfg)
		for _, i := range issues {
			if i.Level == "error" && i.Check == "profile" {
				t.Errorf("unexpected profile error: %s", i.Message)
			}
		}
	})
}

func TestValidateSweepConfig_Iterations(t *testing.T) {
	viper.Reset()
	viper.Set("profiles.test.model", "test.gguf")
	defer viper.Reset()

	t.Run("zero iterations", func(t *testing.T) {
		cfg := SweepConfig{Profile: "test", Iterations: 0}
		issues := validateSweepConfig(cfg)
		found := false
		for _, i := range issues {
			if i.Level == "error" && i.Check == "iterations" {
				found = true
			}
		}
		if !found {
			t.Error("expected error with check=iterations")
		}
	})

	t.Run("high iterations warns", func(t *testing.T) {
		cfg := SweepConfig{Profile: "test", Iterations: 20}
		issues := validateSweepConfig(cfg)
		found := false
		for _, i := range issues {
			if i.Level == "warning" && i.Check == "iterations" {
				found = true
			}
		}
		if !found {
			t.Error("expected warning with check=iterations")
		}
	})
}

func TestValidateSweepConfig_BinaryPaths(t *testing.T) {
	viper.Reset()
	viper.Set("profiles.test.model", "test.gguf")
	defer viper.Reset()

	t.Run("binary not found", func(t *testing.T) {
		cfg := SweepConfig{
			Profile:       "test",
			Iterations:    5,
			ProfileFields: map[string][]string{"bin": {"/nonexistent/path/llama-server"}},
		}
		issues := validateSweepConfig(cfg)
		found := false
		for _, i := range issues {
			if i.Level == "error" && i.Check == "binary" {
				found = true
			}
		}
		if !found {
			t.Error("expected error with check=binary")
		}
	})

	t.Run("binary exists", func(t *testing.T) {
		tmpBin := filepath.Join(t.TempDir(), "llama-server")
		os.WriteFile(tmpBin, []byte("#!/bin/sh"), 0755)

		cfg := SweepConfig{
			Profile:       "test",
			Iterations:    5,
			ProfileFields: map[string][]string{"bin": {tmpBin}},
		}
		issues := validateSweepConfig(cfg)
		for _, i := range issues {
			if i.Level == "error" && i.Check == "binary" {
				t.Errorf("should not report binary error: %s", i.Message)
			}
		}
	})
}

func TestValidateSweepConfig_Parameters(t *testing.T) {
	viper.Reset()
	viper.Set("profiles.test.model", "test.gguf")
	defer viper.Reset()

	t.Run("unknown parameter warns", func(t *testing.T) {
		cfg := SweepConfig{
			Profile:    "test",
			Iterations: 5,
			Parameters: map[string][]string{"chache-type-k": {"q4_0"}},
		}
		issues := validateSweepConfig(cfg)
		found := false
		for _, i := range issues {
			if i.Level == "warning" && i.Check == "parameter" {
				found = true
			}
		}
		if !found {
			t.Error("expected warning with check=parameter")
		}
	})

	t.Run("invalid quant type", func(t *testing.T) {
		cfg := SweepConfig{
			Profile:    "test",
			Iterations: 5,
			Parameters: map[string][]string{"cache-type-k": {"invalid_type"}},
		}
		issues := validateSweepConfig(cfg)
		found := false
		for _, i := range issues {
			if i.Level == "error" && i.Check == "value" {
				found = true
			}
		}
		if !found {
			t.Error("expected error with check=value")
		}
	})

	t.Run("valid quant types", func(t *testing.T) {
		cfg := SweepConfig{
			Profile:    "test",
			Iterations: 5,
			Parameters: map[string][]string{
				"cache-type-k": {"q4_0", "q8_0"},
				"cache-type-v": {"q4_0", "f16"},
			},
		}
		issues := validateSweepConfig(cfg)
		for _, i := range issues {
			if i.Level == "error" && i.Check == "value" {
				t.Errorf("should not report value error: %s", i.Message)
			}
		}
	})
}

func TestValidateParameterValues(t *testing.T) {
	t.Run("threads non-numeric", func(t *testing.T) {
		params := map[string][]string{"threads": {"abc"}}
		issues := validateParameterValues(params)
		if len(issues) == 0 {
			t.Error("expected error for non-numeric threads")
		}
		if issues[0].Check != "value" {
			t.Errorf("expected check=value, got %q", issues[0].Check)
		}
	})

	t.Run("threads zero", func(t *testing.T) {
		params := map[string][]string{"threads": {"0"}}
		issues := validateParameterValues(params)
		found := false
		for _, i := range issues {
			if i.Level == "error" && strings.Contains(i.Message, "must be > 0") {
				found = true
			}
		}
		if !found {
			t.Error("expected error for threads=0")
		}
	})

	t.Run("batch-size valid", func(t *testing.T) {
		params := map[string][]string{"batch-size": {"1024", "2048"}}
		issues := validateParameterValues(params)
		for _, i := range issues {
			if i.Level == "error" {
				t.Errorf("unexpected error: %s", i.Message)
			}
		}
	})

	t.Run("spec-draft-p-min out of range", func(t *testing.T) {
		params := map[string][]string{"spec-draft-p-min": {"1.5"}}
		issues := validateParameterValues(params)
		found := false
		for _, i := range issues {
			if i.Level == "error" && strings.Contains(i.Message, "0.0-1.0") {
				found = true
			}
		}
		if !found {
			t.Error("expected error for p-min out of range")
		}
	})

	t.Run("spec-draft-p-min valid", func(t *testing.T) {
		params := map[string][]string{"spec-draft-p-min": {"0.5", "0.75"}}
		issues := validateParameterValues(params)
		for _, i := range issues {
			if i.Level == "error" {
				t.Errorf("unexpected error: %s", i.Message)
			}
		}
	})

	t.Run("spec-draft-n-max negative", func(t *testing.T) {
		params := map[string][]string{"spec-draft-n-max": {"-1"}}
		issues := validateParameterValues(params)
		found := false
		for _, i := range issues {
			if i.Level == "error" && strings.Contains(i.Message, "must be > 0") {
				found = true
			}
		}
		if !found {
			t.Error("expected error for negative n-max")
		}
	})
}

func TestValidateSweepConfig_Toggles(t *testing.T) {
	viper.Reset()
	viper.Set("profiles.test.model", "test.gguf")
	defer viper.Reset()

	t.Run("unknown toggle warns", func(t *testing.T) {
		cfg := SweepConfig{
			Profile:    "test",
			Iterations: 5,
			Toggles:    map[string][]string{"unknown-toggle": {"on", "off"}},
		}
		issues := validateSweepConfig(cfg)
		found := false
		for _, i := range issues {
			if i.Level == "warning" && i.Check == "toggle" {
				found = true
			}
		}
		if !found {
			t.Error("expected warning with check=toggle")
		}
	})

	t.Run("np invalid value", func(t *testing.T) {
		cfg := SweepConfig{
			Profile:    "test",
			Iterations: 5,
			Toggles:    map[string][]string{"np": {"on", "maybe"}},
		}
		issues := validateSweepConfig(cfg)
		found := false
		for _, i := range issues {
			if i.Level == "error" && i.Check == "toggle" {
				found = true
			}
		}
		if !found {
			t.Error("expected error with check=toggle")
		}
	})

	t.Run("load-mode unusual value warns", func(t *testing.T) {
		cfg := SweepConfig{
			Profile:    "test",
			Iterations: 5,
			Toggles:    map[string][]string{"load-mode": {"strange"}},
		}
		issues := validateSweepConfig(cfg)
		found := false
		for _, i := range issues {
			if i.Level == "warning" && i.Check == "toggle" {
				found = true
			}
		}
		if !found {
			t.Error("expected warning with check=toggle")
		}
	})
}

func TestLoadSweepConfig(t *testing.T) {
	t.Run("valid config", func(t *testing.T) {
		content := `profile: test
iterations: 5
parameters:
  cache-type-k:
    - q4_0
    - q8_0
toggles:
  np:
    - "on"
    - "off"
`
		path := filepath.Join(t.TempDir(), "sweep.yaml")
		os.WriteFile(path, []byte(content), 0644)

		cfg, err := loadSweepConfig(path)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.Profile != "test" {
			t.Errorf("profile: got %q, want %q", cfg.Profile, "test")
		}
		if cfg.Iterations != 5 {
			t.Errorf("iterations: got %d, want 5", cfg.Iterations)
		}
		if len(cfg.Parameters["cache-type-k"]) != 2 {
			t.Errorf("cache-type-k values: got %d, want 2", len(cfg.Parameters["cache-type-k"]))
		}
	})

	t.Run("file not found", func(t *testing.T) {
		_, err := loadSweepConfig("/nonexistent/path.yaml")
		if err == nil {
			t.Error("expected error for missing file")
		}
	})

	t.Run("invalid yaml", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "bad.yaml")
		os.WriteFile(path, []byte("{{invalid"), 0644)

		_, err := loadSweepConfig(path)
		if err == nil {
			t.Error("expected error for invalid yaml")
		}
	})
}

func TestValidateSweepConfig_EmptyValues(t *testing.T) {
	viper.Reset()
	viper.Set("profiles.test.model", "test.gguf")
	defer viper.Reset()

	t.Run("empty parameter values", func(t *testing.T) {
		cfg := SweepConfig{
			Profile:    "test",
			Iterations: 5,
			Parameters: map[string][]string{"cache-type-k": {}},
		}
		issues := validateSweepConfig(cfg)
		found := false
		for _, i := range issues {
			if i.Level == "error" && i.Check == "parameter" && strings.Contains(i.Message, "no values") {
				found = true
			}
		}
		if !found {
			t.Error("expected error for empty parameter values")
		}
	})

	t.Run("empty toggle values", func(t *testing.T) {
		cfg := SweepConfig{
			Profile:    "test",
			Iterations: 5,
			Toggles:    map[string][]string{"np": {}},
		}
		issues := validateSweepConfig(cfg)
		found := false
		for _, i := range issues {
			if i.Level == "error" && i.Check == "toggle" && strings.Contains(i.Message, "no values") {
				found = true
			}
		}
		if !found {
			t.Error("expected error for empty toggle values")
		}
	})

	t.Run("empty profile field values", func(t *testing.T) {
		cfg := SweepConfig{
			Profile:       "test",
			Iterations:    5,
			ProfileFields: map[string][]string{"bin": {}},
		}
		issues := validateSweepConfig(cfg)
		found := false
		for _, i := range issues {
			if i.Level == "error" && i.Check == "profile-field" && strings.Contains(i.Message, "no values") {
				found = true
			}
		}
		if !found {
			t.Error("expected error for empty profile field values")
		}
	})
}

func TestValidateSweepConfig_DuplicateValues(t *testing.T) {
	viper.Reset()
	viper.Set("profiles.test.model", "test.gguf")
	defer viper.Reset()

	t.Run("duplicate parameter values warns", func(t *testing.T) {
		cfg := SweepConfig{
			Profile:    "test",
			Iterations: 5,
			Parameters: map[string][]string{"cache-type-k": {"q4_0", "q4_0"}},
		}
		issues := validateSweepConfig(cfg)
		found := false
		for _, i := range issues {
			if i.Level == "warning" && strings.Contains(i.Message, "duplicate") {
				found = true
			}
		}
		if !found {
			t.Error("expected warning for duplicate parameter values")
		}
	})

	t.Run("duplicate toggle values warns", func(t *testing.T) {
		cfg := SweepConfig{
			Profile:    "test",
			Iterations: 5,
			Toggles:    map[string][]string{"np": {"on", "on"}},
		}
		issues := validateSweepConfig(cfg)
		found := false
		for _, i := range issues {
			if i.Level == "warning" && strings.Contains(i.Message, "duplicate") {
				found = true
			}
		}
		if !found {
			t.Error("expected warning for duplicate toggle values")
		}
	})
}

func TestValidateParameterValues_UbatchBatchCross(t *testing.T) {
	t.Run("ubatch > batch warns", func(t *testing.T) {
		params := map[string][]string{
			"batch-size":  {"1024"},
			"ubatch-size": {"2048"},
		}
		issues := validateParameterValues(params)
		found := false
		for _, i := range issues {
			if i.Level == "warning" && strings.Contains(i.Message, "ubatch-size") && strings.Contains(i.Message, "batch-size") {
				found = true
			}
		}
		if !found {
			t.Error("expected warning when ubatch-size > batch-size")
		}
	})

	t.Run("ubatch <= batch no warning", func(t *testing.T) {
		params := map[string][]string{
			"batch-size":  {"2048"},
			"ubatch-size": {"1024"},
		}
		issues := validateParameterValues(params)
		for _, i := range issues {
			if strings.Contains(i.Message, "ubatch-size") && strings.Contains(i.Message, "batch-size") {
				t.Errorf("unexpected ubatch/batch warning: %s", i.Message)
			}
		}
	})
}

func TestFindDuplicates(t *testing.T) {
	t.Run("no duplicates", func(t *testing.T) {
		dups := findDuplicates([]string{"a", "b", "c"})
		if len(dups) != 0 {
			t.Errorf("expected no duplicates, got %v", dups)
		}
	})

	t.Run("with duplicates", func(t *testing.T) {
		dups := findDuplicates([]string{"a", "b", "a", "c", "b"})
		if len(dups) != 2 {
			t.Errorf("expected 2 duplicates, got %v", dups)
		}
	})

	t.Run("empty slice", func(t *testing.T) {
		dups := findDuplicates([]string{})
		if len(dups) != 0 {
			t.Errorf("expected no duplicates, got %v", dups)
		}
	})
}

func TestGetPassedChecks(t *testing.T) {
	t.Run("all pass when no issues", func(t *testing.T) {
		cfg := SweepConfig{
			Profile:    "test",
			Iterations: 5,
			ProfileFields: map[string][]string{"bin": {"/usr/bin/test"}},
		}
		var issues []ValidationIssue
		passed := getPassedChecks(cfg, issues)
		if len(passed) < 4 {
			t.Errorf("expected at least 4 passed checks, got %d: %v", len(passed), passed)
		}
	})

	t.Run("profile error hides profile pass", func(t *testing.T) {
		cfg := SweepConfig{Profile: "bad"}
		issues := []ValidationIssue{
			{Level: "error", Check: "profile", Message: "profile not found"},
		}
		passed := getPassedChecks(cfg, issues)
		for _, p := range passed {
			if strings.Contains(p, "Profile") {
				t.Errorf("should not show Profile passed when profile has error: %v", passed)
			}
		}
	})

	t.Run("binary error hides binary pass", func(t *testing.T) {
		cfg := SweepConfig{
			ProfileFields: map[string][]string{"bin": {"/bad/path"}},
		}
		issues := []ValidationIssue{
			{Level: "error", Check: "binary", Message: "binary not found"},
		}
		passed := getPassedChecks(cfg, issues)
		for _, p := range passed {
			if strings.Contains(p, "Binary") {
				t.Errorf("should not show Binary passed when binary has error: %v", passed)
			}
		}
	})
}
