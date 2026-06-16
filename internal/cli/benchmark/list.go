package benchmark

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/jparrill/auriga-cli/internal/config"
	"github.com/jparrill/auriga-cli/internal/ui"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

type benchmarkResult struct {
	Model        string `json:"model"`
	Backend      string `json:"backend"`
	Attempts     int    `json:"attempts"`
	Success      bool   `json:"success"`
	Duration     int    `json:"total_duration_seconds"`
	FilesCreated int    `json:"files_created"`
	Dir          string `json:"-"`
	HasSrc       bool   `json:"-"`
}

func newBenchmarkListCmd() *cobra.Command {
	var (
		failedOnly bool
		run        string
	)

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List benchmark results",
		Long: `List results from a benchmark run.

Examples:
  auriga benchmark list                    # Latest run
  auriga benchmark list --failed           # Only failed from latest
  auriga benchmark list --run 2026-06-14_1045  # Specific run
  auriga benchmark list --run all          # List available runs`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runBenchmarkList(failedOnly, run)
		},
	}

	cmd.Flags().BoolVar(&failedOnly, "failed", false, "Only show failed results")
	cmd.Flags().StringVar(&run, "run", "latest", "Run to list (timestamp, 'latest', or 'all')")

	return cmd
}

func runBenchmarkList(failedOnly bool, run string) error {
	resultsDir := config.ExpandHome(viper.GetString("benchmark.results_dir"))

	if run == "all" {
		return listRuns(resultsDir)
	}

	runDir := resolveRunDir(resultsDir, run)
	if runDir == "" {
		return fmt.Errorf("run %q not found in %s", run, resultsDir)
	}

	entries, err := os.ReadDir(runDir)
	if err != nil {
		return fmt.Errorf("cannot read run dir %s: %w", runDir, err)
	}

	var results []benchmarkResult
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		metaPath := filepath.Join(runDir, e.Name(), "metadata.json")
		data, err := os.ReadFile(metaPath)
		if err != nil {
			continue
		}
		var r benchmarkResult
		if err := json.Unmarshal(data, &r); err != nil {
			continue
		}
		r.Dir = filepath.Join(runDir, e.Name())
		_, srcErr := os.Stat(filepath.Join(r.Dir, "project", "src"))
		r.HasSrc = srcErr == nil

		if failedOnly && r.Success {
			continue
		}
		results = append(results, r)
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Model < results[j].Model
	})

	runName := filepath.Base(runDir)
	fmt.Printf("\n  %s\n", ui.BoldStyle.Render(fmt.Sprintf("Run: %s", runName)))

	fmt.Printf("  %-45s %-14s %-6s %5s  %6s  %s\n",
		"MODEL", "BACKEND", "PASS", "FILES", "TIME", "SRC")
	fmt.Printf("  %s\n", strings.Repeat("─", 86))

	for _, r := range results {
		status := ui.ErrorStyle.Render("✗")
		if r.Success {
			status = ui.SuccessStyle.Render("✓")
		}
		src := ui.ErrorStyle.Render("✗")
		if r.HasSrc {
			src = ui.SuccessStyle.Render("✓")
		}
		model := r.Model
		if len(model) > 44 {
			model = model[:44]
		}
		// Pad manually after ANSI-colored strings
		fmt.Printf("  %-45s %-14s %s      %5d  %5ds  %s\n",
			model, r.Backend, status, r.FilesCreated, r.Duration, src)
	}
	fmt.Println()

	return nil
}

func listRuns(resultsDir string) error {
	entries, err := os.ReadDir(resultsDir)
	if err != nil {
		return fmt.Errorf("cannot read results dir: %w", err)
	}

	fmt.Printf("\n  %s\n", ui.BoldStyle.Render("Available runs"))
	fmt.Printf("  %-25s %8s\n", "RUN", "MODELS")
	fmt.Printf("  %s\n", strings.Repeat("─", 35))

	for _, e := range entries {
		if !e.IsDir() || e.Name() == "latest" {
			continue
		}
		// Count model dirs
		subEntries, _ := os.ReadDir(filepath.Join(resultsDir, e.Name()))
		count := 0
		for _, se := range subEntries {
			if se.IsDir() {
				count++
			}
		}
		if count == 0 {
			continue
		}
		fmt.Printf("  %-25s %8d\n", e.Name(), count)
	}

	// Show what latest points to
	latestTarget, err := os.Readlink(filepath.Join(resultsDir, "latest"))
	if err == nil {
		fmt.Printf("\n  latest → %s\n", latestTarget)
	}
	fmt.Println()

	return nil
}

func resolveRunDir(resultsDir, run string) string {
	if run == "latest" {
		latestLink := filepath.Join(resultsDir, "latest")
		target, err := os.Readlink(latestLink)
		if err != nil {
			// Fallback: no symlink, check if results are flat (legacy)
			if hasLegacyResults(resultsDir) {
				return resultsDir
			}
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
	return ""
}

func hasLegacyResults(dir string) bool {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	for _, e := range entries {
		if e.IsDir() {
			meta := filepath.Join(dir, e.Name(), "metadata.json")
			if _, err := os.Stat(meta); err == nil {
				return true
			}
		}
	}
	return false
}
