package benchmark

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	bench "github.com/jparrill/auriga-cli/internal/benchmark"
	"github.com/jparrill/auriga-cli/internal/config"
	"github.com/jparrill/auriga-cli/internal/ui"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func newBenchmarkCompareCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "compare <run-A> <run-B>",
		Short: "Compare results between two benchmark runs",
		Long: `Side-by-side comparison of two benchmark runs showing pass/fail deltas,
time differences, and regressions.

Examples:
  auriga benchmark compare latest 2026-07-10_1430
  auriga benchmark compare 2026-07-12_0900 2026-07-12_1400`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runBenchmarkCompare(args[0], args[1])
		},
	}

	return cmd
}

type comparisonRow struct {
	suite   string
	model   string
	taskID  string
	backend string
	resultA *bench.Result
	resultB *bench.Result
}

func runBenchmarkCompare(runA, runB string) error {
	resultsDir := config.ExpandHome(viper.GetString("benchmark.results_dir"))

	dirA := resolveRunDir(resultsDir, runA)
	if dirA == "" {
		return fmt.Errorf("run %q not found in %s", runA, resultsDir)
	}
	dirB := resolveRunDir(resultsDir, runB)
	if dirB == "" {
		return fmt.Errorf("run %q not found in %s", runB, resultsDir)
	}

	resultsA, err := loadSummary(dirA)
	if err != nil {
		return fmt.Errorf("cannot load run A (%s): %w", runA, err)
	}
	resultsB, err := loadSummary(dirB)
	if err != nil {
		return fmt.Errorf("cannot load run B (%s): %w", runB, err)
	}

	rows := buildComparison(resultsA, resultsB)
	printComparison(rows, filepath.Base(dirA), filepath.Base(dirB))
	return nil
}

func loadSummary(runDir string) ([]bench.Result, error) {
	data, err := os.ReadFile(filepath.Join(runDir, "summary.json"))
	if err != nil {
		return nil, err
	}
	var results []bench.Result
	if err := json.Unmarshal(data, &results); err != nil {
		return nil, err
	}
	return results, nil
}

func resultKey(r bench.Result) string {
	return fmt.Sprintf("%s__%s__%s__%s", r.Suite, r.TaskID, r.Model, r.Backend)
}

func buildComparison(a, b []bench.Result) []comparisonRow {
	mapA := make(map[string]*bench.Result)
	for i := range a {
		mapA[resultKey(a[i])] = &a[i]
	}

	mapB := make(map[string]*bench.Result)
	for i := range b {
		mapB[resultKey(b[i])] = &b[i]
	}

	seen := make(map[string]bool)
	var rows []comparisonRow

	for _, r := range a {
		key := resultKey(r)
		if seen[key] {
			continue
		}
		seen[key] = true
		row := comparisonRow{
			suite:   r.Suite,
			model:   r.Model,
			taskID:  r.TaskID,
			backend: r.Backend,
			resultA: mapA[key],
			resultB: mapB[key],
		}
		rows = append(rows, row)
	}

	for _, r := range b {
		key := resultKey(r)
		if seen[key] {
			continue
		}
		seen[key] = true
		rows = append(rows, comparisonRow{
			suite:   r.Suite,
			model:   r.Model,
			taskID:  r.TaskID,
			backend: r.Backend,
			resultA: nil,
			resultB: mapB[key],
		})
	}

	return rows
}

func deltaSymbol(row comparisonRow) string {
	if row.resultA == nil {
		return ui.AccentStyle.Render("new")
	}
	if row.resultB == nil {
		return ui.MutedStyle.Render("gone")
	}
	if row.resultA.Success == row.resultB.Success {
		return ui.MutedStyle.Render("=")
	}
	if !row.resultA.Success && row.resultB.Success {
		return ui.SuccessStyle.Render("+")
	}
	return ui.ErrorStyle.Render("-")
}

func passSymbol(r *bench.Result) string {
	if r == nil {
		return ui.MutedStyle.Render("—")
	}
	if r.Success {
		return ui.SuccessStyle.Render("✓")
	}
	return ui.ErrorStyle.Render("✗")
}

func timeStr(r *bench.Result) string {
	if r == nil {
		return ui.MutedStyle.Render("—")
	}
	return fmt.Sprintf("%ds", r.Duration)
}

func printComparison(rows []comparisonRow, nameA, nameB string) {
	if len(rows) == 0 {
		ui.Info("No results to compare")
		return
	}

	fmt.Printf("\n  %s\n\n", ui.BoldStyle.Render(fmt.Sprintf("Comparing: %s  vs  %s", nameA, nameB)))

	tbl := ui.NewTable("Per-problem comparison", "MODEL", "TASK", "A", "B", "DELTA", "TIME-A", "TIME-B")
	for _, row := range rows {
		model := row.model
		if len(model) > 30 {
			model = model[:30]
		}
		task := row.taskID
		if len(task) > 25 {
			task = task[:25]
		}

		tbl.AddRow(model, task, passSymbol(row.resultA), passSymbol(row.resultB),
			deltaSymbol(row), timeStr(row.resultA), timeStr(row.resultB))
	}
	tbl.Print()

	printComparisonSummary(rows, nameA, nameB)
}

func printComparisonSummary(rows []comparisonRow, nameA, nameB string) {
	var passA, failA, passB, failB, improved, regressed int
	var timeA, timeB int

	for _, row := range rows {
		if row.resultA != nil {
			if row.resultA.Success {
				passA++
			} else {
				failA++
			}
			timeA += row.resultA.Duration
		}
		if row.resultB != nil {
			if row.resultB.Success {
				passB++
			} else {
				failB++
			}
			timeB += row.resultB.Duration
		}
		if row.resultA != nil && row.resultB != nil {
			if !row.resultA.Success && row.resultB.Success {
				improved++
			}
			if row.resultA.Success && !row.resultB.Success {
				regressed++
			}
		}
	}

	totalA := passA + failA
	totalB := passB + failB
	rateA := float64(0)
	rateB := float64(0)
	if totalA > 0 {
		rateA = float64(passA) / float64(totalA) * 100
	}
	if totalB > 0 {
		rateB = float64(passB) / float64(totalB) * 100
	}

	rateDelta := rateB - rateA
	rateColor := ui.MutedStyle
	if rateDelta > 0 {
		rateColor = ui.SuccessStyle
	} else if rateDelta < 0 {
		rateColor = ui.ErrorStyle
	}

	fmt.Printf("  %s  %s  %s\n",
		ui.BoldStyle.Render("Summary:"),
		fmt.Sprintf("%s: %.1f%% (%d/%d)", nameA, rateA, passA, totalA),
		fmt.Sprintf("%s: %.1f%% (%d/%d)", nameB, rateB, passB, totalB))

	fmt.Printf("  %s  %s  %s  %s\n\n",
		rateColor.Render(fmt.Sprintf("Rate Δ: %+.1f%%", rateDelta)),
		ui.SuccessStyle.Render(fmt.Sprintf("Improved: %d", improved)),
		ui.ErrorStyle.Render(fmt.Sprintf("Regressed: %d", regressed)),
		ui.MutedStyle.Render(fmt.Sprintf("Time: %ds → %ds", timeA, timeB)))
}
