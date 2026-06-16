package fix

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	goexec "os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/jparrill/auriga-cli/internal/config"
	"github.com/jparrill/auriga-cli/internal/llamaserver"
	"github.com/jparrill/auriga-cli/internal/ollama"
	"github.com/jparrill/auriga-cli/internal/pi"
	"github.com/jparrill/auriga-cli/internal/ui"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

type resultMeta struct {
	Model    string `json:"model"`
	Backend  string `json:"backend"`
	Attempts int    `json:"attempts"`
	Success  bool   `json:"success"`
	Duration int    `json:"total_duration_seconds"`
	Files    int    `json:"files_created"`
	Dir      string `json:"-"`
	HasSrc   bool   `json:"-"`
}

type fixOpts struct {
	List       bool
	FailedOnly bool
	Model      string
	Run        string
}

func NewFixCmd() *cobra.Command {
	opts := &fixOpts{}

	cmd := &cobra.Command{
		Use:   "fix",
		Short: "Interactive project fix workflow with Pi",
		Long: `Pick a benchmark result, spin up the model that generated it, and launch Pi for interactive fixes.

Examples:
  auriga fix                   # Interactive fzf picker (latest run)
  auriga fix --list            # Just list all results
  auriga fix --failed          # Pick from failed only
  auriga fix --model gemma4    # Jump to gemma4 result directly
  auriga fix --run 2026-06-14_1045   # Fix from specific run`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runFix(opts)
		},
	}

	cmd.Flags().BoolVar(&opts.List, "list", false, "Only list results")
	cmd.Flags().BoolVar(&opts.FailedOnly, "failed", false, "Only show failed results")
	cmd.Flags().StringVar(&opts.Model, "model", "", "Jump to a specific model (substring match)")
	cmd.Flags().StringVar(&opts.Run, "run", "latest", "Run to use (timestamp or 'latest')")

	return cmd
}

func loadResults(run string) ([]resultMeta, error) {
	resultsDir := config.ExpandHome(viper.GetString("benchmark.results_dir"))
	runDir := resolveRunDir(resultsDir, run)
	if runDir == "" {
		return nil, fmt.Errorf("run %q not found in %s", run, resultsDir)
	}
	entries, err := os.ReadDir(runDir)
	if err != nil {
		return nil, fmt.Errorf("cannot read results: %w", err)
	}

	var results []resultMeta
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		data, err := os.ReadFile(filepath.Join(runDir, e.Name(), "metadata.json"))
		if err != nil {
			continue
		}
		var r resultMeta
		if err := json.Unmarshal(data, &r); err != nil {
			continue
		}
		r.Dir = filepath.Join(runDir, e.Name())
		_, srcErr := os.Stat(filepath.Join(r.Dir, "project", "src"))
		r.HasSrc = srcErr == nil
		results = append(results, r)
	}

	sort.Slice(results, func(i, j int) bool { return results[i].Model < results[j].Model })
	return results, nil
}

func printTable(results []resultMeta) {
	pass := ui.SuccessStyle.Render("✓")
	fail := ui.ErrorStyle.Render("✗")

	fmt.Printf("\n  %3s  %-45s %-14s %4s  %5s  %6s  %4s\n",
		"#", "MODEL", "BACKEND", "PASS", "FILES", "TIME", "SRC")
	fmt.Printf("  %s\n", strings.Repeat("─", 88))

	for i, r := range results {
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
		fmt.Printf("  %3d  %-45s %-14s %4s  %5d  %5ds  %4s\n",
			i+1, model, r.Backend, status, r.Files, r.Duration, src)
	}
	fmt.Println()
}

func selectResult(results []resultMeta) *resultMeta {
	// Try fzf
	if r := selectWithFzf(results); r != nil {
		return r
	}
	// Fallback to numeric input
	printTable(results)
	fmt.Printf("  %s ", ui.InfoStyle.Render("Select project #"))
	var choice int
	if _, err := fmt.Scanf("%d", &choice); err != nil || choice < 1 || choice > len(results) {
		return nil
	}
	return &results[choice-1]
}

func selectWithFzf(results []resultMeta) *resultMeta {
	if _, err := goexec.LookPath("fzf"); err != nil {
		return nil
	}

	var lines []string
	for i, r := range results {
		status := "PASS"
		if !r.Success {
			status = "FAIL"
		}
		lines = append(lines, fmt.Sprintf("%d|%4s | %-50s | %-14s | %3d files",
			i, status, r.Model, r.Backend, r.Files))
	}

	cmd := goexec.Command("fzf", "--header=Select a project to fix", "--reverse")
	cmd.Stdin = strings.NewReader(strings.Join(lines, "\n"))
	cmd.Stderr = os.Stderr
	out, err := cmd.Output()
	if err != nil {
		return nil
	}

	choice := strings.TrimSpace(string(out))
	var idx int
	fmt.Sscanf(choice, "%d|", &idx)
	if idx >= 0 && idx < len(results) {
		return &results[idx]
	}
	return nil
}

func runFix(opts *fixOpts) error {
	results, err := loadResults(opts.Run)
	if err != nil {
		return err
	}

	if opts.FailedOnly {
		var filtered []resultMeta
		for _, r := range results {
			if !r.Success {
				filtered = append(filtered, r)
			}
		}
		results = filtered
	}

	if len(results) == 0 {
		ui.Warn("No results found")
		return nil
	}

	if opts.List {
		printTable(results)
		return nil
	}

	if opts.Model != "" {
		var filtered []resultMeta
		for _, r := range results {
			if strings.Contains(strings.ToLower(r.Model), strings.ToLower(opts.Model)) {
				filtered = append(filtered, r)
			}
		}
		if len(filtered) == 0 {
			return fmt.Errorf("no model matching %q", opts.Model)
		}
		if len(filtered) == 1 {
			return runFixSession(&filtered[0])
		}
		results = filtered
	}

	for {
		selected := selectResult(results)
		if selected == nil {
			ui.Info("No selection, exiting")
			break
		}

		if err := runFixSession(selected); err != nil {
			ui.Fail(err.Error())
		}

		fmt.Printf("\n  %s ", ui.InfoStyle.Render("Next project? [s/N]"))
		var answer string
		fmt.Scanf("%s", &answer)
		answer = strings.ToLower(strings.TrimSpace(answer))
		if answer != "s" && answer != "si" && answer != "y" && answer != "yes" {
			break
		}
	}

	return nil
}

func runFixSession(meta *resultMeta) error {
	ctx := context.Background()
	projectDir := filepath.Join(meta.Dir, "project")

	if _, err := os.Stat(projectDir); err != nil {
		return fmt.Errorf("project dir not found: %s", projectDir)
	}

	fmt.Printf("\n%s\n", ui.BoldStyle.Render(strings.Repeat("═", 60)))
	fmt.Printf("  %s (%s)\n", meta.Model, meta.Backend)
	fmt.Printf("%s\n", ui.BoldStyle.Render(strings.Repeat("═", 60)))
	ui.Info(fmt.Sprintf("Status: %v | Files: %d | Has src/: %v", meta.Success, meta.Files, meta.HasSrc))

	var llamaProc *os.Process

	if meta.Backend == "llama-server" {
		gguf := llamaserver.FindLocalGGUF(meta.Model)
		if gguf == "" {
			return fmt.Errorf("no GGUF found for %s", meta.Model)
		}
		var err error
		llamaProc, err = llamaserver.Start(ctx, gguf, "", nil)
		if err != nil {
			return err
		}
		defer llamaserver.Stop(llamaProc)
	} else {
		if !ollama.HasModel(meta.Model) {
			ui.Warn(fmt.Sprintf("Model %s not in Ollama, trying anyway...", meta.Model))
		} else {
			ui.Ok(fmt.Sprintf("Model %s available in Ollama", meta.Model))
		}
	}

	status := "FAIL"
	if meta.Success {
		status = "PASS"
	}
	pi.WriteSystemMD(projectDir, meta.Model, meta.Backend, status, meta.Files)
	ui.Ok("SYSTEM.md written")

	modelID := meta.Model
	if meta.Backend == "llama-server" {
		modelID = "local"
	}

	return pi.Launch(ctx, projectDir, modelID)
}

func resolveRunDir(resultsDir, run string) string {
	if run == "latest" {
		latestLink := filepath.Join(resultsDir, "latest")
		target, err := os.Readlink(latestLink)
		if err != nil {
			// Fallback: legacy flat results
			entries, _ := os.ReadDir(resultsDir)
			for _, e := range entries {
				if e.IsDir() {
					meta := filepath.Join(resultsDir, e.Name(), "metadata.json")
					if _, err := os.Stat(meta); err == nil {
						return resultsDir
					}
				}
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
