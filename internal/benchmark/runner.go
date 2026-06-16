package benchmark

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/jparrill/auriga-cli/internal/llamaserver"
	"github.com/jparrill/auriga-cli/internal/ollama"
	"github.com/jparrill/auriga-cli/internal/ui"
)

type RunConfig struct {
	Backend      string
	Models       []string
	MaxRetries   int
	MaxTokens    int
	GenTimeout   time.Duration
	ResultsDir   string
	PlanFile     string
	SourceHTML   string
	Benchmarks   string
}

type Result struct {
	Model        string `json:"model"`
	Backend      string `json:"backend"`
	Attempts     int    `json:"attempts"`
	Success      bool   `json:"success"`
	Duration     int    `json:"total_duration_seconds"`
	FilesCreated int    `json:"files_created"`
	Timestamp    string `json:"timestamp"`
	Error        string `json:"error,omitempty"`
}

func RunAll(cfg RunConfig) ([]Result, error) {
	prompt, err := BuildPrompt(cfg.PlanFile, cfg.SourceHTML, cfg.Benchmarks)
	if err != nil {
		return nil, err
	}

	ui.Info(fmt.Sprintf("Prompt: %d chars", len(prompt)))

	type job struct {
		model   string
		backend string
	}
	var jobs []job

	if cfg.Backend == "ollama" || cfg.Backend == "all" {
		for _, m := range cfg.Models {
			if cfg.Backend == "all" || cfg.Backend == "ollama" {
				jobs = append(jobs, job{m, "ollama"})
			}
		}
	}

	if cfg.Backend == "llama-server" {
		for _, m := range cfg.Models {
			jobs = append(jobs, job{m, "llama-server"})
		}
	}

	// If models are specified with a backend, use only those
	if len(cfg.Models) > 0 && cfg.Backend != "all" {
		jobs = nil
		for _, m := range cfg.Models {
			jobs = append(jobs, job{m, cfg.Backend})
		}
	}

	ui.Info(fmt.Sprintf("Total jobs: %d", len(jobs)))

	var results []Result
	for _, j := range jobs {
		r := runSingle(j.model, j.backend, prompt, cfg)
		results = append(results, r)

		if j.backend == "ollama" {
			ollama.StopModel(j.model)
			time.Sleep(3 * time.Second)
		}
	}

	summaryPath := filepath.Join(cfg.ResultsDir, "summary.json")
	data, _ := json.MarshalIndent(results, "", "  ")
	os.WriteFile(summaryPath, data, 0644)

	return results, nil
}

func runSingle(model, backend, prompt string, cfg RunConfig) Result {
	slug := regexp.MustCompile(`[/:]`).ReplaceAllString(model, "_")
	outputDir := filepath.Join(cfg.ResultsDir, slug+"__"+backend)
	os.MkdirAll(outputDir, 0755)
	projectDir := filepath.Join(outputDir, "project")

	fmt.Printf("\n%s\n%s (%s)\n%s\n",
		ui.BoldStyle.Render(strings.Repeat("═", 60)),
		model, backend,
		ui.BoldStyle.Render(strings.Repeat("═", 60)))

	os.WriteFile(filepath.Join(outputDir, "prompt.txt"), []byte(prompt), 0644)

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
			return Result{Model: model, Backend: backend, Error: "no GGUF found"}
		}
		var err error
		llamaProc, err = llamaserver.Start(nil, gguf, "", nil)
		if err != nil {
			ui.Fail(fmt.Sprintf("llama-server failed: %v", err))
			return Result{Model: model, Backend: backend, Error: err.Error()}
		}
		defer llamaserver.Stop(llamaProc)
	}

	currentPrompt := prompt

	for attempt < cfg.MaxRetries && !success {
		attempt++
		fmt.Printf("\n  %s\n", ui.BoldStyle.Render(fmt.Sprintf("[Attempt %d/%d]", attempt, cfg.MaxRetries)))

		start := time.Now()
		var response string
		var err error

		if backend == "ollama" {
			ui.Info("Calling Ollama...")
			response, err = ollama.Generate(model, currentPrompt, cfg.MaxTokens, cfg.GenTimeout)
		} else {
			ui.Info("Calling llama-server...")
			response, err = llamaserver.Generate(currentPrompt, cfg.MaxTokens, cfg.GenTimeout)
		}

		duration := int(time.Since(start).Seconds())
		totalDuration += duration

		if err != nil {
			ui.Fail(fmt.Sprintf("Error after %ds: %v", duration, err))
			continue
		}

		ui.Info(fmt.Sprintf("Response: %d chars in %ds", len(response), duration))
		os.WriteFile(filepath.Join(outputDir, fmt.Sprintf("raw_output_%d.txt", attempt)), []byte(response), 0644)

		os.RemoveAll(projectDir)
		os.MkdirAll(projectDir, 0755)

		filesCreated, _ = ParseFiles(response, projectDir)
		ui.Info(fmt.Sprintf("Files created: %d", filesCreated))

		if filesCreated == 0 {
			ui.Warn("No files parsed — will retry with format instructions")
			if attempt < cfg.MaxRetries {
				currentPrompt = BuildFormatRetryPrompt(prompt)
			}
			continue
		}

		violations := CheckSensitiveData(projectDir)
		if len(violations) > 0 {
			ui.Fail(fmt.Sprintf("%d sensitive data violations:", len(violations)))
			for _, v := range violations {
				ui.Fail(fmt.Sprintf("  %s in %s", v.Description, v.FilePath))
			}
			if attempt < cfg.MaxRetries {
				currentPrompt = BuildSensitiveRetryPrompt(prompt, violations)
			}
			continue
		}

		ui.Ok("No sensitive data found")

		buildOk, buildErr := ValidateBuild(projectDir)
		if !buildOk {
			ui.Fail("Build failed")
			if attempt < cfg.MaxRetries {
				currentPrompt = BuildBuildRetryPrompt(prompt, buildErr)
			}
			continue
		}

		ui.Ok("Build passed — project is valid")
		success = true
	}

	result := Result{
		Model:        model,
		Backend:      backend,
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

func PrintSummary(results []Result) {
	fmt.Printf("\n%s\n", ui.BoldStyle.Render(strings.Repeat("═", 60)))
	fmt.Printf("%s\n", ui.BoldStyle.Render("BENCHMARK SUMMARY"))
	fmt.Printf("%s\n", ui.BoldStyle.Render(strings.Repeat("═", 60)))

	fmt.Printf("\n  %-45s %-15s %-6s %-7s %-8s %s\n",
		"Model", "Backend", "Pass", "Files", "Time", "Tries")
	fmt.Printf("  %s\n", strings.Repeat("─", 90))

	for _, r := range results {
		m := r.Model
		if len(m) > 44 {
			m = m[:44]
		}
		status := ui.ErrorStyle.Render("✗")
		if r.Success {
			status = ui.SuccessStyle.Render("✓")
		}
		fmt.Printf("  %-44s %-14s %s      %-7d %ds     %d\n",
			m, r.Backend, status, r.FilesCreated, r.Duration, r.Attempts)
	}

	fmt.Printf("\n  Results: results/\n")
}
