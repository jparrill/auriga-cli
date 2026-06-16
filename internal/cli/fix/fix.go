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
	List          bool
	FailedOnly    bool
	Model         string
	Run           string
	ModelOverride string
}

func NewFixCmd() *cobra.Command {
	opts := &fixOpts{}

	cmd := &cobra.Command{
		Use:   "fix",
		Short: "Interactive project fix workflow with Pi",
		Long: `Pick a benchmark result, spin up the model that generated it, and launch Pi for interactive fixes.

Examples:
  auriga fix                                         # Interactive fzf picker (latest run)
  auriga fix --list                                  # Just list all results
  auriga fix --failed                                # Pick from failed only
  auriga fix --model gemma4                          # Jump to gemma4 result directly
  auriga fix --run 2026-06-14_1045                   # Fix from specific run
  auriga fix --run all                               # List available runs
  auriga fix --model-override qwen3.6-vision         # Use a profile with vision
  auriga fix --model-override gemma4:26b             # Use a different Ollama model`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runFix(opts)
		},
	}

	cmd.Flags().BoolVar(&opts.List, "list", false, "Only list results")
	cmd.Flags().BoolVar(&opts.FailedOnly, "failed", false, "Only show failed results")
	cmd.Flags().StringVar(&opts.Model, "model", "", "Jump to a specific model (substring match)")
	cmd.Flags().StringVar(&opts.Run, "run", "latest", "Run to use (timestamp, 'latest', or 'all')")
	cmd.Flags().StringVar(&opts.ModelOverride, "model-override", "", "Override model: profile name (vision) or Ollama model")

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

func listRuns() error {
	resultsDir := config.ExpandHome(viper.GetString("benchmark.results_dir"))
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

	latestTarget, err := os.Readlink(filepath.Join(resultsDir, "latest"))
	if err == nil {
		fmt.Printf("\n  latest → %s\n", latestTarget)
	}
	fmt.Println()
	return nil
}

func runFix(opts *fixOpts) error {
	if opts.Run == "all" {
		return listRuns()
	}

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
			return runFixSession(opts.ModelOverride,&filtered[0])
		}
		results = filtered
	}

	for {
		selected := selectResult(results)
		if selected == nil {
			ui.Info("No selection, exiting")
			break
		}

		if err := runFixSession(opts.ModelOverride,selected); err != nil {
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

func runFixSession(modelOverride string, meta *resultMeta) error {
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
	var modelID string

	if modelOverride != "" {
		// Check if override is a profile
		profileKey := fmt.Sprintf("profiles.%s", modelOverride)
		profileModel := viper.GetString(profileKey + ".model")

		if profileModel != "" {
			// It's a profile — use llama-server
			ggufDir := config.ExpandHome(viper.GetString("llama_server.gguf_dir"))
			mmprojDir := config.ExpandHome(viper.GetString("llama_server.mmproj_dir"))
			modelPath := filepath.Join(ggufDir, profileModel)
			mmprojFile := viper.GetString(profileKey + ".mmproj")
			var mmprojPath string
			if mmprojFile != "" {
				mmprojPath = filepath.Join(mmprojDir, mmprojFile)
			}

			ui.Info(fmt.Sprintf("Override: profile %s", modelOverride))
			if mmprojPath != "" {
				ui.Info(fmt.Sprintf("Vision: %s", filepath.Base(mmprojPath)))
			}

			var extraFlags []string
			if mmprojPath != "" {
				extraFlags = append(extraFlags, "--jinja")
			}

			var err error
			llamaProc, err = llamaserver.Start(ctx, modelPath, mmprojPath, extraFlags)
			if err != nil {
				return err
			}
			defer llamaserver.Stop(llamaProc)
			modelID = "local"
		} else {
			// It's an Ollama model name
			ui.Info(fmt.Sprintf("Override: Ollama model %s", modelOverride))
			if !ollama.HasModel(modelOverride) {
				ui.Warn(fmt.Sprintf("Model %s not in Ollama, trying anyway...", modelOverride))
			}
			modelID = modelOverride
		}
	} else {
		// Default: use the model that generated the project
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
			modelID = "local"
		} else {
			if !ollama.HasModel(meta.Model) {
				ui.Warn(fmt.Sprintf("Model %s not in Ollama, trying anyway...", meta.Model))
			} else {
				ui.Ok(fmt.Sprintf("Model %s available in Ollama", meta.Model))
			}
			modelID = meta.Model
		}
	}

	status := "FAIL"
	if meta.Success {
		status = "PASS"
	}
	pi.WriteSystemMD(projectDir, meta.Model, meta.Backend, status, meta.Files)
	ui.Ok("SYSTEM.md written")

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
