package sweep

import (
	"fmt"
	"os"
	"runtime"
	"strconv"

	"github.com/jparrill/auriga-cli/internal/config"
	"github.com/jparrill/auriga-cli/internal/ui"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

func newSweepValidateCmd() *cobra.Command {
	var configFile string
	cmd := &cobra.Command{
		Use:   "validate",
		Short: "Validate a sweep config file",
		Long: `Validate a sweep YAML config before running. Checks profile existence,
binary paths, parameter validity, and estimates runtime.

Example:
  auriga sweep validate --config sweep.yaml`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSweepValidate(configFile)
		},
	}
	cmd.Flags().StringVar(&configFile, "config", "", "Path to sweep config YAML")
	cmd.MarkFlagRequired("config")
	return cmd
}

func loadSweepConfig(path string) (SweepConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return SweepConfig{}, fmt.Errorf("cannot read sweep config: %w", err)
	}
	var cfg SweepConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return SweepConfig{}, fmt.Errorf("cannot parse sweep config: %w", err)
	}
	return cfg, nil
}

func runSweepValidate(configPath string) error {
	cfg, err := loadSweepConfig(configPath)
	if err != nil {
		return err
	}

	issues := validateSweepConfig(cfg)

	ui.Info(fmt.Sprintf("Sweep config: %s", configPath))
	ui.Info(fmt.Sprintf("Profile:      %s", cfg.Profile))

	profileKey := fmt.Sprintf("profiles.%s", cfg.Profile)
	model := viper.GetString(profileKey + ".model")
	if model != "" {
		ui.Info(fmt.Sprintf("Model:        %s", model))
	}
	fmt.Println()

	hasErrors := false
	for _, issue := range issues {
		if issue.Level == "error" {
			hasErrors = true
			ui.Warn(fmt.Sprintf("  ✗ %s", issue.Message))
		} else {
			ui.Info(fmt.Sprintf("  ⚠ %s", issue.Message))
		}
	}

	if !hasErrors {
		for _, check := range getPassedChecks(cfg, issues) {
			ui.Ok(fmt.Sprintf("  ✓ %s", check))
		}
	}

	combos := cartesianProduct(cfg)
	estMinutes := float64(len(combos)) * 3.0
	fmt.Println()
	ui.Info(fmt.Sprintf("Combinations: %d", len(combos)))
	ui.Info(fmt.Sprintf("Est. runtime: ~%.1fh (%d × ~3min)", estMinutes/60.0, len(combos)))

	if hasErrors {
		fmt.Println()
		ui.Warn("Config has errors. Fix them before running.")
		return fmt.Errorf("validation failed")
	}

	fmt.Println()
	ui.Ok("Config is valid.")
	return nil
}

func validateSweepConfig(cfg SweepConfig) []ValidationIssue {
	var issues []ValidationIssue

	profileKey := fmt.Sprintf("profiles.%s", cfg.Profile)
	model := viper.GetString(profileKey + ".model")
	if cfg.Profile == "" {
		issues = append(issues, ValidationIssue{Level: "error", Check: "profile", Message: "profile is empty"})
	} else if model == "" {
		issues = append(issues, ValidationIssue{Level: "error", Check: "profile", Message: fmt.Sprintf("profile %q not found in auriga config", cfg.Profile)})
	}

	if cfg.Iterations <= 0 {
		issues = append(issues, ValidationIssue{Level: "error", Check: "iterations", Message: "iterations must be > 0"})
	} else if cfg.Iterations > 10 {
		issues = append(issues, ValidationIssue{Level: "warning", Check: "iterations", Message: fmt.Sprintf("iterations=%d is high — each combination will take longer", cfg.Iterations)})
	}

	for field := range cfg.ProfileFields {
		if !knownProfileFields[field] {
			issues = append(issues, ValidationIssue{Level: "warning", Check: "profile-field", Message: fmt.Sprintf("unknown profile field %q", field)})
		}
	}

	if bins, ok := cfg.ProfileFields["bin"]; ok {
		for _, bin := range bins {
			expanded := config.ExpandHome(bin)
			if _, err := os.Stat(expanded); err != nil {
				issues = append(issues, ValidationIssue{Level: "error", Check: "binary", Message: fmt.Sprintf("binary not found: %s", expanded)})
			}
		}
	}

	for param, vals := range cfg.Parameters {
		if !knownParameters[param] {
			issues = append(issues, ValidationIssue{Level: "warning", Check: "parameter", Message: fmt.Sprintf("unknown parameter %q — check for typos", param)})
		}
		if len(vals) == 0 {
			issues = append(issues, ValidationIssue{Level: "error", Check: "parameter", Message: fmt.Sprintf("parameter %q has no values", param)})
		}
		if dups := findDuplicates(vals); len(dups) > 0 {
			issues = append(issues, ValidationIssue{Level: "warning", Check: "parameter", Message: fmt.Sprintf("parameter %q has duplicate values: %v", param, dups)})
		}
	}

	for field, vals := range cfg.ProfileFields {
		if len(vals) == 0 {
			issues = append(issues, ValidationIssue{Level: "error", Check: "profile-field", Message: fmt.Sprintf("profile field %q has no values", field)})
		}
	}

	for toggle, vals := range cfg.Toggles {
		if len(vals) == 0 {
			issues = append(issues, ValidationIssue{Level: "error", Check: "toggle", Message: fmt.Sprintf("toggle %q has no values", toggle)})
		}
		if dups := findDuplicates(vals); len(dups) > 0 {
			issues = append(issues, ValidationIssue{Level: "warning", Check: "toggle", Message: fmt.Sprintf("toggle %q has duplicate values: %v", toggle, dups)})
		}
	}

	issues = append(issues, validateLinkedParameters(cfg)...)

	allParams := make(map[string][]string)
	for k, v := range cfg.Parameters {
		allParams[k] = v
	}
	for _, group := range cfg.LinkedParameters {
		for k, v := range group {
			allParams[k] = v
		}
	}
	issues = append(issues, validateParameterValues(allParams)...)

	for toggle := range cfg.Toggles {
		if !knownToggles[toggle] {
			issues = append(issues, ValidationIssue{Level: "warning", Check: "toggle", Message: fmt.Sprintf("unknown toggle %q — check for typos", toggle)})
		}
	}

	if npVals, ok := cfg.Toggles["np"]; ok {
		for _, v := range npVals {
			if v != "on" && v != "off" {
				issues = append(issues, ValidationIssue{Level: "error", Check: "toggle", Message: fmt.Sprintf("np toggle value %q invalid — must be 'on' or 'off'", v)})
			}
		}
	}

	if lmVals, ok := cfg.Toggles["load-mode"]; ok {
		for _, v := range lmVals {
			if v != "off" && v != "mlock" && v != "mmap" {
				issues = append(issues, ValidationIssue{Level: "warning", Check: "toggle", Message: fmt.Sprintf("load-mode value %q is unusual — expected 'off', 'mlock', or 'mmap'", v)})
			}
		}
	}

	return issues
}

func validateParameterValues(params map[string][]string) []ValidationIssue {
	var issues []ValidationIssue

	if vals, ok := params["cache-type-k"]; ok {
		for _, v := range vals {
			if !validQuantTypes[v] {
				issues = append(issues, ValidationIssue{Level: "error", Check: "value", Message: fmt.Sprintf("cache-type-k value %q is not a valid quant type", v)})
			}
		}
	}
	if vals, ok := params["cache-type-v"]; ok {
		for _, v := range vals {
			if !validQuantTypes[v] {
				issues = append(issues, ValidationIssue{Level: "error", Check: "value", Message: fmt.Sprintf("cache-type-v value %q is not a valid quant type", v)})
			}
		}
	}

	cpuCount := runtime.NumCPU()
	if vals, ok := params["threads"]; ok {
		for _, v := range vals {
			n, err := strconv.Atoi(v)
			if err != nil {
				issues = append(issues, ValidationIssue{Level: "error", Check: "value", Message: fmt.Sprintf("threads value %q is not numeric", v)})
			} else if n <= 0 {
				issues = append(issues, ValidationIssue{Level: "error", Check: "value", Message: fmt.Sprintf("threads value %d must be > 0", n)})
			} else if n > cpuCount {
				issues = append(issues, ValidationIssue{Level: "warning", Check: "value", Message: fmt.Sprintf("threads=%d exceeds CPU count (%d)", n, cpuCount)})
			}
		}
	}

	for _, key := range []string{"batch-size", "ubatch-size"} {
		if vals, ok := params[key]; ok {
			for _, v := range vals {
				n, err := strconv.Atoi(v)
				if err != nil {
					issues = append(issues, ValidationIssue{Level: "error", Check: "value", Message: fmt.Sprintf("%s value %q is not numeric", key, v)})
				} else if n <= 0 {
					issues = append(issues, ValidationIssue{Level: "error", Check: "value", Message: fmt.Sprintf("%s value %d must be > 0", key, n)})
				}
			}
		}
	}

	if vals, ok := params["spec-draft-n-max"]; ok {
		for _, v := range vals {
			n, err := strconv.Atoi(v)
			if err != nil {
				issues = append(issues, ValidationIssue{Level: "error", Check: "value", Message: fmt.Sprintf("spec-draft-n-max value %q is not numeric", v)})
			} else if n <= 0 {
				issues = append(issues, ValidationIssue{Level: "error", Check: "value", Message: fmt.Sprintf("spec-draft-n-max value %d must be > 0", n)})
			}
		}
	}

	if vals, ok := params["spec-draft-p-min"]; ok {
		for _, v := range vals {
			f, err := strconv.ParseFloat(v, 64)
			if err != nil {
				issues = append(issues, ValidationIssue{Level: "error", Check: "value", Message: fmt.Sprintf("spec-draft-p-min value %q is not numeric", v)})
			} else if f < 0 || f > 1 {
				issues = append(issues, ValidationIssue{Level: "error", Check: "value", Message: fmt.Sprintf("spec-draft-p-min value %s must be 0.0-1.0", v)})
			}
		}
	}

	batchVals, hasBatch := params["batch-size"]
	ubatchVals, hasUbatch := params["ubatch-size"]
	if hasBatch && hasUbatch {
		for _, ub := range ubatchVals {
			ubN, ubErr := strconv.Atoi(ub)
			if ubErr != nil {
				continue
			}
			for _, b := range batchVals {
				bN, bErr := strconv.Atoi(b)
				if bErr != nil {
					continue
				}
				if ubN > bN {
					issues = append(issues, ValidationIssue{Level: "warning", Check: "value", Message: fmt.Sprintf("ubatch-size %d > batch-size %d — llama-server requires ubatch-size <= batch-size", ubN, bN)})
				}
			}
		}
	}

	return issues
}

func validateLinkedParameters(cfg SweepConfig) []ValidationIssue {
	var issues []ValidationIssue

	standaloneParams := make(map[string]bool)
	for p := range cfg.Parameters {
		standaloneParams[p] = true
	}

	for groupName, group := range cfg.LinkedParameters {
		if len(group) == 0 {
			issues = append(issues, ValidationIssue{Level: "error", Check: "linked", Message: fmt.Sprintf("linked group %q is empty", groupName)})
			continue
		}

		var expectedLen int
		first := true
		for param, vals := range group {
			if !knownParameters[param] {
				issues = append(issues, ValidationIssue{Level: "warning", Check: "linked", Message: fmt.Sprintf("linked group %q: unknown parameter %q", groupName, param)})
			}
			if standaloneParams[param] {
				issues = append(issues, ValidationIssue{Level: "error", Check: "linked", Message: fmt.Sprintf("parameter %q appears in both parameters and linked group %q", param, groupName)})
			}
			if len(vals) == 0 {
				issues = append(issues, ValidationIssue{Level: "error", Check: "linked", Message: fmt.Sprintf("linked group %q: parameter %q has no values", groupName, param)})
				continue
			}
			if first {
				expectedLen = len(vals)
				first = false
			} else if len(vals) != expectedLen {
				issues = append(issues, ValidationIssue{Level: "error", Check: "linked", Message: fmt.Sprintf("linked group %q: arrays must have same length (got %d and %d)", groupName, expectedLen, len(vals))})
			}
		}
	}

	return issues
}

func getPassedChecks(cfg SweepConfig, issues []ValidationIssue) []string {
	var passed []string

	hasErrorInCheck := func(check string) bool {
		for _, i := range issues {
			if i.Level == "error" && i.Check == check {
				return true
			}
		}
		return false
	}

	if !hasErrorInCheck("profile") {
		passed = append(passed, "Profile exists")
	}
	if bins, ok := cfg.ProfileFields["bin"]; ok && !hasErrorInCheck("binary") {
		for _, bin := range bins {
			passed = append(passed, fmt.Sprintf("Binary %s found", bin))
		}
	}
	if !hasErrorInCheck("parameter") {
		passed = append(passed, "All parameters known")
	}
	if !hasErrorInCheck("value") {
		passed = append(passed, "Parameter values valid")
	}
	if len(cfg.LinkedParameters) > 0 && !hasErrorInCheck("linked") {
		passed = append(passed, "Linked parameters valid")
	}
	if !hasErrorInCheck("toggle") {
		passed = append(passed, "Toggles valid")
	}

	return passed
}

func findDuplicates(vals []string) []string {
	seen := make(map[string]bool)
	var dups []string
	for _, v := range vals {
		if seen[v] {
			dups = append(dups, v)
		}
		seen[v] = true
	}
	return dups
}
