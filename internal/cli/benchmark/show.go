package benchmark

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	bench "github.com/jparrill/auriga-cli/internal/benchmark"
	"github.com/jparrill/auriga-cli/internal/config"
	"github.com/jparrill/auriga-cli/internal/ui"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func newBenchmarkShowCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show [run]",
		Short: "Show detailed results for a benchmark run",
		Long: `Show per-problem results and performance metrics for a benchmark run.

Examples:
  auriga benchmark show                     # Latest run
  auriga benchmark show 2026-09-08_0956     # Specific run`,
		RunE: func(cmd *cobra.Command, args []string) error {
			run := "latest"
			if len(args) > 0 {
				run = args[0]
			}
			return runBenchmarkShow(run)
		},
	}
	return cmd
}

func runBenchmarkShow(run string) error {
	resultsDir := config.ExpandHome(viper.GetString("benchmark.results_dir"))
	runDir := resolveRunDir(resultsDir, run)
	if runDir == "" {
		return fmt.Errorf("run %q not found in %s", run, resultsDir)
	}

	summaryPath := filepath.Join(runDir, "summary.json")
	data, err := os.ReadFile(summaryPath)
	if err != nil {
		return fmt.Errorf("no summary.json in %s: %w", runDir, err)
	}

	var results []bench.Result
	if err := json.Unmarshal(data, &results); err != nil {
		return fmt.Errorf("invalid summary.json: %w", err)
	}

	if len(results) == 0 {
		ui.Info("No results in this run")
		return nil
	}

	for i := range results {
		results[i].Model = filepath.Base(results[i].Model)
	}

	runName := filepath.Base(runDir)
	fmt.Printf("\n  %s  %s\n", ui.BoldStyle.Render("Run:"), runName)
	fmt.Printf("  %s  %s\n\n", ui.BoldStyle.Render("Dir:"), runDir)

	bench.PrintSummary(results)
	return nil
}

func resolveRunDir(resultsDir, run string) string {
	if run == "latest" {
		latestLink := filepath.Join(resultsDir, "latest")
		target, err := os.Readlink(latestLink)
		if err != nil {
			return ""
		}
		if !filepath.IsAbs(target) {
			target = filepath.Join(resultsDir, target)
		}
		return target
	}

	dir := filepath.Join(resultsDir, run)
	if _, err := os.Stat(dir); err == nil {
		return dir
	}

	entries, err := os.ReadDir(resultsDir)
	if err != nil {
		return ""
	}
	for _, e := range entries {
		if e.IsDir() && strings.Contains(e.Name(), run) {
			return filepath.Join(resultsDir, e.Name())
		}
	}
	return ""
}
