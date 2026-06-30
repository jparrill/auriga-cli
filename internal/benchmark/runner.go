package benchmark

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/jparrill/auriga-cli/internal/benchmark/formats"
	"github.com/jparrill/auriga-cli/internal/llamaserver"
	"github.com/jparrill/auriga-cli/internal/ollama"
	"github.com/jparrill/auriga-cli/internal/ui"
)

type RunConfig struct {
	Backend    string
	Models     []string
	MaxRetries int
	MaxTokens  int
	GenTimeout time.Duration
	ResultsDir string
	Host       string
	// Legacy (used when no suite specified)
	PlanFile   string
	SourceHTML string
	Benchmarks string
	// Suite mode
	SuiteName  string
}

type Result struct {
	Model        string `json:"model"`
	Backend      string `json:"backend"`
	Suite        string `json:"suite,omitempty"`
	TaskID       string `json:"task_id,omitempty"`
	Level        string `json:"level,omitempty"`
	Attempts     int    `json:"attempts"`
	Success      bool   `json:"success"`
	Duration     int    `json:"total_duration_seconds"`
	FilesCreated int    `json:"files_created"`
	PassCount    int    `json:"pass_count,omitempty"`
	TotalCount   int    `json:"total_count,omitempty"`
	Timestamp    string `json:"timestamp"`
	Error        string `json:"error,omitempty"`
}

func RunAll(cfg RunConfig) ([]Result, error) {
	var fmtSuite formats.Suite
	var format formats.FormatRunner
	var problems []formats.Problem

	if cfg.SuiteName != "" {
		suite, err := LoadSuite(cfg.SuiteName)
		if err != nil {
			return nil, err
		}
		format, err = formats.Get(suite.Format)
		if err != nil {
			return nil, err
		}
		rawProblems, err := LoadProblems(suite)
		if err != nil {
			return nil, err
		}
		// Convert Problem types
		for _, p := range rawProblems {
			problems = append(problems, formats.Problem{
				TaskID: p.TaskID, Prompt: p.Prompt, Test: p.Test,
				EntryPoint: p.EntryPoint, Level: p.Level, Eval: p.Eval, TestCmd: p.TestCmd,
			})
		}
		if len(problems) == 0 && suite.Format == "webgen" {
			problems = []formats.Problem{{TaskID: "webgen"}}
		}
		fmtSuite = formats.Suite{
			Name: suite.Name, Format: suite.Format, Language: suite.Language, Dir: suite.Dir,
			PlanFile: suite.PlanFile, SourceHTML: suite.SourceHTML, BenchJSON: suite.BenchJSON,
		}
	} else {
		fmtSuite = formats.Suite{
			Name:       "astro-webgen",
			Format:     "webgen",
			PlanFile:   cfg.PlanFile,
			SourceHTML: cfg.SourceHTML,
			BenchJSON:  cfg.Benchmarks,
			Dir:        filepath.Dir(cfg.PlanFile),
		}
		var err error
		format, err = formats.Get("webgen")
		if err != nil {
			return nil, fmt.Errorf("webgen format not registered: %w", err)
		}
		problems = []formats.Problem{{TaskID: "webgen"}}
	}

	// Create timestamped run directory
	runTimestamp := time.Now().Format("2006-01-02_1504")
	runDir := filepath.Join(cfg.ResultsDir, runTimestamp)
	if err := os.MkdirAll(runDir, 0755); err != nil {
		return nil, fmt.Errorf("cannot create run dir: %w", err)
	}

	latestLink := filepath.Join(cfg.ResultsDir, "latest")
	os.Remove(latestLink)
	os.Symlink(runTimestamp, latestLink)

	ui.Info(fmt.Sprintf("Run: %s", runTimestamp))
	ui.Info(fmt.Sprintf("Suite: %s (%s)", fmtSuite.Name, fmtSuite.Format))
	ui.Info(fmt.Sprintf("Problems: %d", len(problems)))

	// Build jobs
	type job struct {
		model   string
		backend string
	}
	var jobs []job

	if cfg.Backend == "ollama" || cfg.Backend == "all" {
		for _, m := range cfg.Models {
			jobs = append(jobs, job{m, "ollama"})
		}
	}
	if cfg.Backend == "llama-server" || cfg.Backend == "all" {
		for _, m := range cfg.Models {
			jobs = append(jobs, job{m, "llama-server"})
		}
	}
	if len(cfg.Models) > 0 && cfg.Backend != "all" {
		jobs = nil
		for _, m := range cfg.Models {
			jobs = append(jobs, job{m, cfg.Backend})
		}
	}

	ui.Info(fmt.Sprintf("Total jobs: %d", len(jobs)))

	var results []Result
	for _, j := range jobs {
		for _, problem := range problems {
			r := runSingle(j.model, j.backend, problem, fmtSuite, format, cfg, runDir)
			results = append(results, r)
		}

		if j.backend == "ollama" {
			ollama.StopModel(j.model)
			time.Sleep(3 * time.Second)
		}
	}

	summaryPath := filepath.Join(runDir, "summary.json")
	data, _ := json.MarshalIndent(results, "", "  ")
	os.WriteFile(summaryPath, data, 0644)

	return results, nil
}

func runSingle(model, backend string, problem formats.Problem, suite formats.Suite, format formats.FormatRunner, cfg RunConfig, runDir string) Result {
	slug := regexp.MustCompile(`[/:]`).ReplaceAllString(model, "_")
	taskSlug := regexp.MustCompile(`[/:]`).ReplaceAllString(problem.TaskID, "_")
	outputDir := filepath.Join(runDir, fmt.Sprintf("%s__%s__%s", suite.Name, slug, backend))
	if taskSlug != "webgen" {
		outputDir = filepath.Join(outputDir, "problems", taskSlug)
	}
	os.MkdirAll(outputDir, 0755)
	workDir := filepath.Join(outputDir, "project")

	fmt.Printf("\n%s\n%s (%s) — %s\n%s\n",
		ui.BoldStyle.Render(strings.Repeat("═", 60)),
		model, backend, problem.TaskID,
		ui.BoldStyle.Render(strings.Repeat("═", 60)))

	var (
		attempt       int
		success       bool
		totalDuration int
		filesCreated  int
		llamaProc     *os.Process
	)

	if backend == "llama-server" {
		gguf := llamaserver.FindLocalGGUF(model)
		if gguf == "" {
			ui.Fail(fmt.Sprintf("No GGUF found for %s", model))
			return Result{Model: model, Backend: backend, Suite: suite.Name, TaskID: problem.TaskID, Error: "no GGUF found"}
		}
		ctx := context.Background()
		var err error
		llamaProc, err = llamaserver.Start(ctx, gguf, "", nil)
		if err != nil {
			ui.Fail(fmt.Sprintf("llama-server failed: %v", err))
			return Result{Model: model, Backend: backend, Suite: suite.Name, TaskID: problem.TaskID, Error: err.Error()}
		}
		defer llamaserver.Stop(llamaProc)
	}

	// Build initial prompt
	prompt, err := format.BuildPrompt(problem, suite)
	if err != nil {
		ui.Fail(fmt.Sprintf("Cannot build prompt: %v", err))
		return Result{Model: model, Backend: backend, Suite: suite.Name, TaskID: problem.TaskID, Error: err.Error()}
	}

	currentPrompt := prompt
	os.WriteFile(filepath.Join(outputDir, "prompt.txt"), []byte(prompt), 0644)

	for attempt < cfg.MaxRetries && !success {
		attempt++
		fmt.Printf("\n  %s\n", ui.BoldStyle.Render(fmt.Sprintf("[Attempt %d/%d]", attempt, cfg.MaxRetries)))

		start := time.Now()
		var response string
		var genErr error

		if backend == "ollama" {
			ui.Info("Calling Ollama...")
			response, genErr = ollama.Generate(model, currentPrompt, cfg.MaxTokens, cfg.GenTimeout)
		} else {
			ui.Info("Calling llama-server...")
			response, genErr = llamaserver.Generate(currentPrompt, cfg.MaxTokens, cfg.GenTimeout)
		}

		duration := int(time.Since(start).Seconds())
		totalDuration += duration

		if genErr != nil {
			ui.Fail(fmt.Sprintf("Error after %ds: %v", duration, genErr))
			continue
		}

		ui.Info(fmt.Sprintf("Response: %d chars in %ds", len(response), duration))
		os.WriteFile(filepath.Join(outputDir, fmt.Sprintf("raw_output_%d.txt", attempt)), []byte(response), 0644)

		if attempt == 1 {
			os.RemoveAll(workDir)
		}
		os.MkdirAll(workDir, 0755)

		// Validate via format runner
		ok, validationErr, err := format.ValidateResponse(response, problem, workDir)
		if err != nil {
			ui.Fail(fmt.Sprintf("Validation error: %v", err))
			continue
		}

		// Count files
		entries, _ := os.ReadDir(workDir)
		fileCount := 0
		filepath.Walk(workDir, func(_ string, info os.FileInfo, _ error) error {
			if info != nil && !info.IsDir() {
				fileCount++
			}
			return nil
		})
		if attempt == 1 {
			filesCreated = fileCount
		} else {
			filesCreated = fileCount
		}
		_ = entries

		if ok {
			ui.Ok(fmt.Sprintf("Validation passed — %d files", fileCount))
			success = true
		} else {
			ui.Fail(fmt.Sprintf("Validation: %s", truncateValidationErr(validationErr)))
			if attempt < cfg.MaxRetries {
				retryPrompt, err := format.BuildRetryPrompt(problem, workDir, validationErr)
				if err != nil {
					ui.Warn(fmt.Sprintf("Cannot build retry prompt: %v", err))
					continue
				}
				if retryPrompt != "" {
					currentPrompt = retryPrompt
				}
			}
		}
	}

	result := Result{
		Model:        model,
		Backend:      backend,
		Suite:        suite.Name,
		TaskID:       problem.TaskID,
		Attempts:     attempt,
		Success:      success,
		Duration:     totalDuration,
		FilesCreated: filesCreated,
		Timestamp:    time.Now().Format(time.RFC3339),
	}

	data, _ := json.MarshalIndent(result, "", "  ")
	os.WriteFile(filepath.Join(outputDir, "metadata.json"), data, 0644)

	if success {
		fmt.Printf("\n  Result: %s | Files: %d | Duration: %ds\n",
			ui.SuccessStyle.Render("✓ PASS"), filesCreated, totalDuration)
	} else {
		fmt.Printf("\n  Result: %s | Files: %d | Duration: %ds\n",
			ui.ErrorStyle.Render("✗ FAIL"), filesCreated, totalDuration)
	}

	return result
}

func truncateValidationErr(s string) string {
	if len(s) > 100 {
		return s[:100] + "..."
	}
	return s
}

func PrintSummary(results []Result) {
	fmt.Printf("\n%s\n", ui.BoldStyle.Render(strings.Repeat("═", 60)))
	fmt.Printf("%s\n", ui.BoldStyle.Render("BENCHMARK SUMMARY"))
	fmt.Printf("%s\n", ui.BoldStyle.Render(strings.Repeat("═", 60)))

	tbl := ui.NewTable("", "SUITE", "MODEL", "BACKEND", "TASK", "PASS", "FILES", "TIME", "TRIES")
	for _, r := range results {
		status := ui.ErrorStyle.Render("✗")
		if r.Success {
			status = ui.SuccessStyle.Render("✓")
		}
		model := r.Model
		if len(model) > 35 {
			model = model[:35]
		}
		tbl.AddRow(r.Suite, model, r.Backend, r.TaskID, status,
			fmt.Sprintf("%d", r.FilesCreated),
			fmt.Sprintf("%ds", r.Duration),
			fmt.Sprintf("%d", r.Attempts))
	}
	tbl.Print()
}
