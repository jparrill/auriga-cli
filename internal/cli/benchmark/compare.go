package benchmark

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/charmbracelet/lipgloss"
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
	return fmt.Sprintf("%s__%s__%s", r.Suite, r.TaskID, r.Backend)
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
	return formatDuration(r.Duration, false)
}

func formatDuration(seconds int, signed bool) string {
	sign := ""
	if signed && seconds > 0 {
		sign = "+"
	} else if signed && seconds < 0 {
		sign = "-"
	}
	seconds = absInt(seconds)
	if seconds < 60 {
		return fmt.Sprintf("%s%ds", sign, seconds)
	}
	if seconds < 3600 {
		return fmt.Sprintf("%s%dm%02ds", sign, seconds/60, seconds%60)
	}
	return fmt.Sprintf("%s%dh%02dm", sign, seconds/3600, (seconds%3600)/60)
}

func resultStr(r *bench.Result) string {
	if r == nil {
		return ui.MutedStyle.Render("—")
	}
	return fmt.Sprintf("%s %s", passSymbol(r), timeStr(r))
}

func profileLabel(runName, suite string, result *bench.Result) string {
	name := filepath.Base(runName)
	name = strings.TrimPrefix(name, suite+"-")
	name = regexp.MustCompile(`-\d{4}-\d{2}-\d{2}_\d{4}$`).ReplaceAllString(name, "")
	if name == filepath.Base(runName) && result != nil {
		name = strings.TrimSuffix(filepath.Base(result.Model), filepath.Ext(result.Model))
	}
	if len(name) > 28 {
		return name[:25] + "..."
	}
	return name
}

func comparisonProfile(rows []comparisonRow, runName string, first bool) string {
	for _, row := range rows {
		result := row.resultB
		if first {
			result = row.resultA
		}
		if result != nil {
			return profileLabel(runName, row.suite, result)
		}
	}
	return profileLabel(runName, "", nil)
}

func absInt(value int) int {
	if value < 0 {
		return -value
	}
	return value
}

func winner(row comparisonRow, profileA, profileB string) string {
	if row.resultA == nil {
		return profileB + " (only)"
	}
	if row.resultB == nil {
		return profileA + " (only)"
	}
	if row.resultA.Success != row.resultB.Success {
		if row.resultA.Success {
			return fmt.Sprintf("%s (%s)", profileA, formatDuration(row.resultA.Duration-row.resultB.Duration, true))
		}
		return fmt.Sprintf("%s (%s)", profileB, formatDuration(row.resultB.Duration-row.resultA.Duration, true))
	}
	if row.resultA.Duration == row.resultB.Duration {
		return "tie"
	}
	if row.resultA.Duration < row.resultB.Duration {
		return fmt.Sprintf("%s (%s)", profileA, formatDuration(row.resultA.Duration-row.resultB.Duration, true))
	}
	return fmt.Sprintf("%s (%s)", profileB, formatDuration(row.resultB.Duration-row.resultA.Duration, true))
}

func profileCell(name string, style lipgloss.Style, width int) string {
	return style.Render(name) + strings.Repeat(" ", width-len(name))
}

func printComparison(rows []comparisonRow, nameA, nameB string) {
	if len(rows) == 0 {
		ui.Info("No results to compare")
		return
	}

	fmt.Printf("\n  %s\n\n", ui.BoldStyle.Render(fmt.Sprintf("Comparing: %s  vs  %s", nameA, nameB)))

	profileA := comparisonProfile(rows, nameA, true)
	profileB := comparisonProfile(rows, nameB, false)
	tbl := ui.NewTable("Per-problem comparison", "BENCHMARK", "TASK", profileA, profileB, "WINNER")
	for _, row := range rows {
		task := row.taskID
		if len(task) > 25 {
			task = task[:25]
		}

		tbl.AddRow(row.suite, task, resultStr(row.resultA), resultStr(row.resultB), winner(row, profileA, profileB))
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
		rateColor = ui.OrangeStyle
	} else if rateDelta < 0 {
		rateColor = ui.InfoStyle
	}
	passedDelta := passB - passA
	passedColor := ui.MutedStyle
	if passedDelta > 0 {
		passedColor = ui.OrangeStyle
	} else if passedDelta < 0 {
		passedColor = ui.InfoStyle
	}
	timeDelta := timeB - timeA
	timeColor := ui.MutedStyle
	if timeDelta > 0 {
		timeColor = ui.InfoStyle
	} else if timeDelta < 0 {
		timeColor = ui.OrangeStyle
	}

	profileA := comparisonProfile(rows, nameA, true)
	profileB := comparisonProfile(rows, nameB, false)
	profileWidth := len(profileA)
	if len(profileB) > profileWidth {
		profileWidth = len(profileB)
	}

	fmt.Printf("\n  %s\n", ui.BoldStyle.Render("Summary"))
	fmt.Printf("  %-*s  %10s  %11s  %10s\n",
		profileWidth, "PROFILE", "PASS RATE", "PASSED", "TIME")
	fmt.Printf("  %s  %10s  %11s  %10s\n",
		profileCell(profileA, ui.InfoStyle, profileWidth),
		ui.SuccessStyle.Render(fmt.Sprintf("%9.1f%%", rateA)),
		fmt.Sprintf("%5d/%-5d", passA, totalA),
		fmt.Sprintf("%9s", formatDuration(timeA, false)))
	fmt.Printf("  %s  %10s  %11s  %10s\n",
		profileCell(profileB, ui.OrangeStyle, profileWidth),
		ui.SuccessStyle.Render(fmt.Sprintf("%9.1f%%", rateB)),
		fmt.Sprintf("%5d/%-5d", passB, totalB),
		fmt.Sprintf("%9s", formatDuration(timeB, false)))
	fmt.Printf("  %-*s  %10s  %11s  %10s\n\n",
		profileWidth, "DELTA",
		rateColor.Render(fmt.Sprintf("%+9.1fpp", rateDelta)),
		passedColor.Render(fmt.Sprintf("%+5d/%-5d", passedDelta, totalB-totalA)),
		timeColor.Render(fmt.Sprintf("%9s", formatDuration(timeDelta, true))))

	fmt.Printf("  %s  %s  %s\n\n",
		ui.SuccessStyle.Render(fmt.Sprintf("Improved: %d", improved)),
		ui.ErrorStyle.Render(fmt.Sprintf("Regressed: %d", regressed)),
		ui.MutedStyle.Render(fmt.Sprintf("Compared: %d tasks", len(rows))))
}
