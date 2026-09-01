package sweep

import (
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/jparrill/auriga-cli/internal/cli/profile"
	"github.com/jparrill/auriga-cli/internal/config"
	"github.com/jparrill/auriga-cli/internal/llamaserver"
	"github.com/jparrill/auriga-cli/internal/perf"
	"github.com/jparrill/auriga-cli/internal/ui"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

func newSweepRunCmd() *cobra.Command {
	var configFile string
	var format string
	cmd := &cobra.Command{
		Use:   "run",
		Short: "Execute a parameter sweep",
		Long: `Run a full parameter sweep from a config file. This modifies the auriga config,
restarts the server for each combination, and benchmarks performance.

The original config is backed up and restored after the sweep completes.
Handles SIGINT/SIGTERM gracefully: restores config and saves partial results.

Output format: json (default) or csv.

Example:
  auriga sweep run --config sweep.yaml
  auriga sweep run --config sweep.yaml --format csv`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if _, err := GetReportWriter(format); err != nil {
				return err
			}
			return runSweep(configFile, format)
		},
	}
	cmd.Flags().StringVar(&configFile, "config", "", "Path to sweep config YAML")
	cmd.Flags().StringVar(&format, "format", "json", "Output format: json or csv")
	cmd.MarkFlagRequired("config")
	return cmd
}

func runSweep(configPath, format string) error {
	cfg, err := loadSweepConfig(configPath)
	if err != nil {
		return err
	}

	issues := validateSweepConfig(cfg)
	var errors []string
	for _, issue := range issues {
		if issue.Level == "error" {
			errors = append(errors, issue.Message)
		}
	}
	if len(errors) > 0 {
		for _, e := range errors {
			ui.Warn(fmt.Sprintf("  ✗ %s", e))
		}
		return fmt.Errorf("validation failed: %d error(s)", len(errors))
	}

	combos := cartesianProduct(cfg)
	if len(combos) == 0 {
		ui.Warn("No combinations to test")
		return nil
	}

	profileKey := fmt.Sprintf("profiles.%s", cfg.Profile)
	model := viper.GetString(profileKey + ".model")
	profileType := viper.GetString(profileKey + ".type")
	if profileType == "" {
		profileType = "dense"
	}
	estMinutes := float64(len(combos)) * 3.0
	hostname, _ := os.Hostname()

	params := []ui.OrderedParam{
		{Key: "Profile", Value: cfg.Profile},
		{Key: "Type", Value: profileType},
		{Key: "Model", Value: model},
		{Key: "Combinations", Value: fmt.Sprintf("%d", len(combos))},
		{Key: "Iterations/combo", Value: fmt.Sprintf("%d", cfg.Iterations)},
		{Key: "Est. runtime", Value: fmt.Sprintf("~%.1fh", estMinutes/60.0)},
	}

	if config.DryRun {
		ui.Info("Dry run — showing combinations without executing")
		for i, combo := range combos {
			ui.Info(fmt.Sprintf("  [%d/%d] %s", i+1, len(combos), comboLabel(combo)))
		}
		return nil
	}

	confirmed, err := ui.ConfirmOperationOrdered("Run parameter sweep", params, "", config.Yes)
	if err != nil || !confirmed {
		return nil
	}

	backupPath := profile.ConfigPath() + ".sweep-backup"
	if err := backupConfig(backupPath); err != nil {
		return fmt.Errorf("failed to backup config: %w", err)
	}
	ui.Ok(fmt.Sprintf("Config backed up to %s", backupPath))

	var results []SweepResult
	var interrupted atomic.Bool

	sigCh := make(chan os.Signal, 1)
	doneCh := make(chan struct{})
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		select {
		case <-sigCh:
			interrupted.Store(true)
		case <-doneCh:
		}
	}()

	port := resolvePort(cfg.Profile)
	sweepStart := time.Now()

	for i, combo := range combos {
		if interrupted.Load() {
			break
		}

		label := comboLabel(combo)
		comboStart := time.Now()

		var result SweepResult
		spinErr := ui.WithSpinner(fmt.Sprintf("[%d/%d] %s", i+1, len(combos), label), func() error {
			if err := applyOverrides(cfg.Profile, combo, backupPath); err != nil {
				result = SweepResult{
					Index:      i,
					Overrides:  flattenCombo(combo),
					DurationMs: time.Since(comboStart).Milliseconds(),
					Error:      err.Error(),
				}
				return err
			}

			resolvedFlags := viper.GetStringSlice(fmt.Sprintf("profiles.%s.flags", cfg.Profile))

			ctxSize := viper.GetInt(fmt.Sprintf("profiles.%s.ctx_size", cfg.Profile))
			if ctxSize <= 0 {
				ctxSize = viper.GetInt("llama_server.ctx_size")
			}
			if ctxSize <= 0 {
				ctxSize = 131072
			}

			if err := profile.RunProfileSwitch(cfg.Profile, profile.SwitchOpts{
				Persistent: true,
				CtxSize:    ctxSize,
				Quiet:      true,
			}); err != nil {
				result = SweepResult{
					Index:         i,
					Overrides:     flattenCombo(combo),
					ResolvedFlags: resolvedFlags,
					DurationMs:    time.Since(comboStart).Milliseconds(),
					Error:         err.Error(),
				}
				return err
			}

			if !waitForHealthy(port, 120*time.Second) {
				result = SweepResult{
					Index:         i,
					Overrides:     flattenCombo(combo),
					ResolvedFlags: resolvedFlags,
					DurationMs:    time.Since(comboStart).Milliseconds(),
					Error:         "server not healthy after timeout",
				}
				return fmt.Errorf("server not healthy")
			}

			warmup := perf.RunSingleBench(port, false)
			if warmup.Error != "" {
				// warmup failed but continue
			}

			ttft, _ := perf.MeasureTTFT(port)

			iterations := cfg.Iterations
			if iterations <= 0 {
				iterations = perf.Iterations
			}

			noThink := perf.RunBenchN(port, false, iterations)
			think := perf.RunBenchN(port, true, iterations)

			result = SweepResult{
				Index:         i,
				Overrides:     flattenCombo(combo),
				ResolvedFlags: resolvedFlags,
				TTFT:          ttft.Milliseconds(),
				Prompt:        noThink.PromptTokPerSec,
				DurationMs:    time.Since(comboStart).Milliseconds(),
				NoThink: BenchData{
					Median: noThink.GenerationTokPerSec,
					Min:    noThink.GenMin,
					Max:    noThink.GenMax,
				},
				Think: BenchData{
					Median: think.GenerationTokPerSec,
					Min:    think.GenMin,
					Max:    think.GenMax,
				},
			}
			if noThink.Error != "" {
				result.Error = noThink.Error
			}
			if think.Error != "" && result.Error == "" {
				result.Error = think.Error
			}

			return nil
		})

		elapsed := time.Since(comboStart).Seconds()
		if spinErr != nil {
			ui.Warn(fmt.Sprintf("[%d/%d] %s — %s (%.0fs)", i+1, len(combos), label, result.Error, elapsed))
		} else {
			ui.Ok(fmt.Sprintf("[%d/%d] %s (%.0fs)", i+1, len(combos), label, elapsed))
		}

		results = append(results, result)
	}

	signal.Stop(sigCh)
	close(doneCh)

	if interrupted.Load() {
		ui.Warn("Interrupted — restoring config and saving partial results...")
	}

	err = ui.WithSpinner("Restoring original config...", func() error {
		if err := restoreConfig(backupPath); err != nil {
			return err
		}
		ctxSize := viper.GetInt(fmt.Sprintf("profiles.%s.ctx_size", cfg.Profile))
		if ctxSize <= 0 {
			ctxSize = 131072
		}
		if err := profile.RunProfileSwitch(cfg.Profile, profile.SwitchOpts{
			Persistent: true,
			CtxSize:    ctxSize,
			Quiet:      true,
		}); err != nil {
			return err
		}
		os.Remove(backupPath)
		return nil
	})
	if err != nil {
		ui.Warn(fmt.Sprintf("Failed to restore: %v — backup at %s", err, backupPath))
	} else {
		ui.Ok("Original config restored")
	}

	printSweepResults(results, cfg)

	report := &SweepReport{
		Profile:       cfg.Profile,
		Model:         model,
		Hostname:      hostname,
		Timestamp:     sweepStart.Format(time.RFC3339),
		TotalDuration: time.Since(sweepStart).Milliseconds(),
		Results:       results,
		Complete:      !interrupted.Load(),
	}
	reportPath, err := saveSweepReport(report, format, sweepStart)
	if err != nil {
		ui.Warn(fmt.Sprintf("Failed to save report: %v", err))
	} else {
		ui.Ok(fmt.Sprintf("Report saved to %s", reportPath))
	}

	return nil
}

func cartesianProduct(cfg SweepConfig) []Combination {
	type dimension struct {
		category string
		key      string
		values   []string
	}

	var dims []dimension

	keys := sortedKeys(cfg.ProfileFields)
	for _, k := range keys {
		dims = append(dims, dimension{category: "profile", key: k, values: cfg.ProfileFields[k]})
	}

	keys = sortedKeys(cfg.Parameters)
	for _, k := range keys {
		dims = append(dims, dimension{category: "param", key: k, values: cfg.Parameters[k]})
	}

	keys = sortedKeys(cfg.Toggles)
	for _, k := range keys {
		dims = append(dims, dimension{category: "toggle", key: k, values: cfg.Toggles[k]})
	}

	if len(dims) == 0 {
		return nil
	}

	total := 1
	for _, d := range dims {
		if len(d.values) == 0 {
			return nil
		}
		total *= len(d.values)
	}

	combos := make([]Combination, 0, total)
	indices := make([]int, len(dims))

	for {
		combo := Combination{
			ProfileFields: make(map[string]string),
			Parameters:    make(map[string]string),
			Toggles:       make(map[string]string),
		}

		for i, d := range dims {
			val := d.values[indices[i]]
			switch d.category {
			case "profile":
				combo.ProfileFields[d.key] = val
			case "param":
				combo.Parameters[d.key] = val
			case "toggle":
				combo.Toggles[d.key] = val
			}
		}

		combos = append(combos, combo)

		carry := true
		for i := len(dims) - 1; i >= 0 && carry; i-- {
			indices[i]++
			if indices[i] < len(dims[i].values) {
				carry = false
			} else {
				indices[i] = 0
			}
		}
		if carry {
			break
		}
	}

	return combos
}

func applyOverrides(profileName string, combo Combination, backupPath string) error {
	if err := restoreConfig(backupPath); err != nil {
		return fmt.Errorf("restore before apply: %w", err)
	}

	doc, err := profile.ReadConfigDoc()
	if err != nil {
		return err
	}

	root := doc.Content[0]
	profilesNode := profile.FindMappingKey(root, "profiles")
	if profilesNode == nil {
		return fmt.Errorf("profiles section not found")
	}
	profileNode := profile.FindMappingKey(profilesNode, profileName)
	if profileNode == nil {
		return fmt.Errorf("profile %q not found", profileName)
	}

	for key, val := range combo.ProfileFields {
		setMappingValue(profileNode, key, val)
	}

	flagsNode := profile.FindMappingKey(profileNode, "flags")
	if flagsNode == nil {
		flagsNode = &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq"}
		profileNode.Content = append(profileNode.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Value: "flags"},
			flagsNode,
		)
	}

	for param, val := range combo.Parameters {
		if targets, ok := paramAliases[param]; ok {
			for _, t := range targets {
				derived := val
				if t.Divide > 0 {
					if n, err := strconv.Atoi(val); err == nil {
						derived = strconv.Itoa(n / t.Divide)
					}
				}
				setFlagValue(flagsNode, "--"+t.Flag, derived)
			}
		} else {
			setFlagValue(flagsNode, "--"+param, val)
		}
	}

	for toggle, val := range combo.Toggles {
		applyToggle(flagsNode, toggle, val)
	}

	if err := profile.WriteConfigDoc(doc); err != nil {
		return err
	}
	viper.SetConfigFile(profile.ConfigPath())
	return viper.ReadInConfig()
}

func setMappingValue(mapping *yaml.Node, key, value string) {
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		if mapping.Content[i].Value == key {
			mapping.Content[i+1].Value = value
			return
		}
	}
	mapping.Content = append(mapping.Content,
		&yaml.Node{Kind: yaml.ScalarNode, Value: key},
		&yaml.Node{Kind: yaml.ScalarNode, Value: value},
	)
}

func setFlagValue(flagsSeq *yaml.Node, flag, value string) {
	for i := 0; i < len(flagsSeq.Content)-1; i++ {
		if flagsSeq.Content[i].Value == flag {
			flagsSeq.Content[i+1].Value = value
			return
		}
	}
	flagsSeq.Content = append(flagsSeq.Content,
		&yaml.Node{Kind: yaml.ScalarNode, Value: flag},
		&yaml.Node{Kind: yaml.ScalarNode, Value: value},
	)
}

func applyToggle(flagsSeq *yaml.Node, toggle, value string) {
	flagName := "--" + toggle
	if toggle == "np" {
		flagName = "-np"
	}

	if value == "off" {
		removeFlag(flagsSeq, flagName)
		return
	}

	if toggle == "np" {
		setFlagValue(flagsSeq, flagName, "1")
		return
	}

	setFlagValue(flagsSeq, flagName, value)
}

func removeFlag(flagsSeq *yaml.Node, flag string) {
	var filtered []*yaml.Node
	skip := false
	for i, node := range flagsSeq.Content {
		if skip {
			skip = false
			continue
		}
		if node.Value == flag {
			if i+1 < len(flagsSeq.Content) && !strings.HasPrefix(flagsSeq.Content[i+1].Value, "--") {
				skip = true
			}
			continue
		}
		filtered = append(filtered, node)
	}
	flagsSeq.Content = filtered
}

func backupConfig(backupPath string) error {
	src := profile.ConfigPath()
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(backupPath, data, 0644)
}

func restoreConfig(backupPath string) error {
	data, err := os.ReadFile(backupPath)
	if err != nil {
		return err
	}
	dst := profile.ConfigPath()
	if err := os.WriteFile(dst, data, 0644); err != nil {
		return err
	}
	viper.SetConfigFile(dst)
	return viper.ReadInConfig()
}

func resolvePort(profileName string) int {
	profileKey := fmt.Sprintf("profiles.%s", profileName)
	if p := viper.GetInt(profileKey + ".port"); p > 0 {
		return p
	}
	t := viper.GetString(profileKey + ".type")
	if t == "moe" {
		moePort := viper.GetInt("llama_server.moe_port")
		if moePort > 0 {
			return moePort
		}
		return 8091
	}
	densePort := viper.GetInt("llama_server.dense_port")
	if densePort > 0 {
		return densePort
	}
	return 8090
}

func waitForHealthy(port int, timeout time.Duration) bool {
	return llamaserver.WaitForHealthOnPort(port, timeout) == nil
}

func comboLabel(combo Combination) string {
	var parts []string
	for k, v := range combo.ProfileFields {
		parts = append(parts, fmt.Sprintf("%s=%s", k, filepath.Base(v)))
	}
	for k, v := range combo.Parameters {
		parts = append(parts, fmt.Sprintf("%s=%s", k, v))
	}
	for k, v := range combo.Toggles {
		parts = append(parts, fmt.Sprintf("%s=%s", k, v))
	}
	sort.Strings(parts)
	return strings.Join(parts, " ")
}

func flattenCombo(combo Combination) map[string]string {
	flat := make(map[string]string)
	for k, v := range combo.ProfileFields {
		flat["field:"+k] = v
	}
	for k, v := range combo.Parameters {
		flat[k] = v
	}
	for k, v := range combo.Toggles {
		flat["toggle:"+k] = v
	}
	return flat
}

func printSweepResults(results []SweepResult, cfg SweepConfig) {
	fmt.Println()

	sorted := make([]SweepResult, len(results))
	copy(sorted, results)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].NoThink.Median > sorted[j].NoThink.Median
	})

	headers := []string{"#", "TTFT", "PROMPT", "NO-THINK", "THINK", "TIME"}

	allKeys := collectOverrideKeys(sorted)
	sort.Strings(allKeys)
	headers = append(headers, allKeys...)

	tbl := ui.NewTable("Sweep Results", headers...)

	for _, r := range sorted {
		if r.Error != "" {
			row := []string{fmt.Sprintf("%d", r.Index+1), "err", "err", "err", "err", "-"}
			for range allKeys {
				row = append(row, "-")
			}
			tbl.AddRow(row...)
			continue
		}

		row := []string{
			fmt.Sprintf("%d", r.Index+1),
			fmt.Sprintf("%dms", r.TTFT),
			fmt.Sprintf("%.1f", r.Prompt),
			fmt.Sprintf("%.1f (%.1f-%.1f)", r.NoThink.Median, r.NoThink.Min, r.NoThink.Max),
			fmt.Sprintf("%.1f (%.1f-%.1f)", r.Think.Median, r.Think.Min, r.Think.Max),
			fmt.Sprintf("%.0fs", float64(r.DurationMs)/1000),
		}
		for _, k := range allKeys {
			if v, ok := r.Overrides[k]; ok {
				if strings.Contains(v, "/") {
					v = filepath.Base(v)
				}
				row = append(row, v)
			} else {
				row = append(row, "-")
			}
		}
		tbl.AddRow(row...)
	}
	tbl.Print()

	if len(sorted) > 0 && sorted[0].Error == "" {
		fmt.Println()
		ui.Ok(fmt.Sprintf("Best no-think gen: %.1f tok/s (combo #%d)", sorted[0].NoThink.Median, sorted[0].Index+1))
	}
}

func collectOverrideKeys(results []SweepResult) []string {
	seen := make(map[string]bool)
	for _, r := range results {
		for k := range r.Overrides {
			seen[k] = true
		}
	}
	var keys []string
	for k := range seen {
		keys = append(keys, k)
	}
	return keys
}

func sortedKeys(m map[string][]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
