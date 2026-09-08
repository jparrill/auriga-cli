package benchmark

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/jparrill/auriga-cli/internal/config"
	"github.com/jparrill/auriga-cli/internal/ui"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

type summaryResult struct {
	Model    string  `json:"model"`
	Suite    string  `json:"suite"`
	Success  bool    `json:"success"`
	Duration int     `json:"total_duration_seconds"`
	GenTokPS float64 `json:"gen_tok_per_sec,omitempty"`
	TTFT     float64 `json:"ttft_ms,omitempty"`
}

func newBenchmarkListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List benchmark runs",
		Long: `List available benchmark runs.

Examples:
  auriga benchmark list                     # All runs
  auriga benchmark show                     # Detail of latest run
  auriga benchmark show 2026-09-08_0956     # Detail of specific run`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runBenchmarkList()
		},
	}
	return cmd
}

func runBenchmarkList() error {
	resultsDir := config.ExpandHome(viper.GetString("benchmark.results_dir"))

	entries, err := os.ReadDir(resultsDir)
	if err != nil {
		return fmt.Errorf("cannot read results dir %s: %w", resultsDir, err)
	}

	type runInfo struct {
		name  string
		suite string
		model string
		pass  int
		total int
		time  int
	}

	var runs []runInfo

	for _, e := range entries {
		if !e.IsDir() || e.Name() == "latest" {
			continue
		}

		runDir := filepath.Join(resultsDir, e.Name())

		summaryPath := filepath.Join(runDir, "summary.json")
		data, err := os.ReadFile(summaryPath)
		if err != nil {
			continue
		}

		var results []summaryResult
		if json.Unmarshal(data, &results) != nil || len(results) == 0 {
			continue
		}

		ri := runInfo{name: e.Name()}
		for _, r := range results {
			if ri.suite == "" {
				ri.suite = r.Suite
			}
			if ri.model == "" {
				ri.model = filepath.Base(r.Model)
			}
			ri.total++
			ri.time += r.Duration
			if r.Success {
				ri.pass++
			}
		}
		runs = append(runs, ri)
	}

	sort.Slice(runs, func(i, j int) bool {
		return runs[i].name > runs[j].name
	})

	latestTarget, _ := os.Readlink(filepath.Join(resultsDir, "latest"))

	tbl := ui.NewTable("Benchmark Runs", "RUN", "SUITE", "MODEL", "PASS", "RATE", "TIME")
	for _, r := range runs {
		rate := float64(0)
		if r.total > 0 {
			rate = float64(r.pass) / float64(r.total) * 100
		}

		name := r.name
		if r.name == latestTarget {
			name = r.name + " *"
		}

		model := r.model
		if len(model) > 40 {
			model = model[:40]
		}

		rateStr := fmt.Sprintf("%.0f%% (%d/%d)", rate, r.pass, r.total)
		if rate >= 80 {
			rateStr = ui.SuccessStyle.Render(rateStr)
		} else if rate >= 50 {
			rateStr = ui.WarningStyle.Render(rateStr)
		} else {
			rateStr = ui.ErrorStyle.Render(rateStr)
		}

		tbl.AddRow(name, r.suite, model, fmt.Sprintf("%d", r.pass), rateStr, fmt.Sprintf("%ds", r.time))
	}
	tbl.Print()

	if len(runs) == 0 {
		ui.Info(fmt.Sprintf("No benchmark runs found in %s", resultsDir))
	}

	return nil
}
