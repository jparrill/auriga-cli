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
	var failedOnly bool

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List benchmark results",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runBenchmarkList(failedOnly)
		},
	}

	cmd.Flags().BoolVar(&failedOnly, "failed", false, "Only show failed results")

	return cmd
}

func runBenchmarkList(failedOnly bool) error {
	resultsDir := config.ExpandHome(viper.GetString("benchmark.results_dir"))

	entries, err := os.ReadDir(resultsDir)
	if err != nil {
		return fmt.Errorf("cannot read results dir %s: %w", resultsDir, err)
	}

	var results []benchmarkResult
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		metaPath := filepath.Join(resultsDir, e.Name(), "metadata.json")
		data, err := os.ReadFile(metaPath)
		if err != nil {
			continue
		}
		var r benchmarkResult
		if err := json.Unmarshal(data, &r); err != nil {
			continue
		}
		r.Dir = filepath.Join(resultsDir, e.Name())
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

	pass := ui.SuccessStyle.Render("✓")
	fail := ui.ErrorStyle.Render("✗")

	fmt.Printf("\n  %-45s %-14s %4s  %5s  %6s  %4s\n",
		"MODEL", "BACKEND", "PASS", "FILES", "TIME", "SRC")
	fmt.Printf("  %s\n", "──────────────────────────────────────────────────────────────────────────────────────")

	for _, r := range results {
		status := fail
		if r.Success {
			status = pass
		}
		src := fail
		if r.HasSrc {
			src = pass
		}
		model := r.Model
		if len(model) > 44 {
			model = model[:44]
		}
		fmt.Printf("  %-45s %-14s %4s  %5d  %5ds  %4s\n",
			model, r.Backend, status, r.FilesCreated, r.Duration, src)
	}
	fmt.Println()

	return nil
}
