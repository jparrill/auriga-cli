package benchmark

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/jparrill/auriga-cli/internal/benchmark/formats"
	"github.com/jparrill/auriga-cli/internal/llamaserver"
	"github.com/jparrill/auriga-cli/internal/ui"
)

type RunConfig struct {
	MaxRetries  int
	MaxTokens   int
	GenTimeout  time.Duration
	ResultsDir  string
	Host        string
	Temperature float64
	PlanFile    string
	SourceHTML  string
	Benchmarks  string
	SuiteName   string
}

type Result struct {
	Model            string  `json:"model"`
	Backend          string  `json:"backend,omitempty"`
	Suite            string  `json:"suite,omitempty"`
	TaskID           string  `json:"task_id,omitempty"`
	Level            string  `json:"level,omitempty"`
	Attempts         int     `json:"attempts"`
	Success          bool    `json:"success"`
	Duration         int     `json:"total_duration_seconds"`
	FilesCreated     int     `json:"files_created"`
	PassCount        int     `json:"pass_count,omitempty"`
	TotalCount       int     `json:"total_count,omitempty"`
	PromptTokens     int     `json:"prompt_tokens,omitempty"`
	CompletionTokens int     `json:"completion_tokens,omitempty"`
	PromptTokPerSec  float64 `json:"prompt_tok_per_sec,omitempty"`
	GenTokPerSec     float64 `json:"gen_tok_per_sec,omitempty"`
	TTFT             float64 `json:"ttft_ms,omitempty"`
	Timestamp        string  `json:"timestamp"`
	Error            string  `json:"error,omitempty"`
}

func RunAll(cfg RunConfig) ([]Result, error) {
	rawModel := detectRunningModel(cfg.Host)
	if rawModel == "" {
		return nil, fmt.Errorf("no model detected on %s — is llama-server running?", cfg.Host)
	}
	model := filepath.Base(rawModel)
	ui.Ok(fmt.Sprintf("Detected model: %s", model))

	ui.Info("Warmup request...")
	warmupStart := time.Now()
	warmupPrompt := "Write a Python function that checks if a number is prime. Include type hints and a docstring. Then write 3 unit tests for it."
	_, warmupErr := llamaserver.Generate(warmupPrompt, 512, 0.0, 60*time.Second)
	warmupDur := time.Since(warmupStart).Seconds()
	if warmupErr != nil {
		ui.Warn(fmt.Sprintf("Warmup failed (%.1fs): %v", warmupDur, warmupErr))
	} else {
		ui.Ok(fmt.Sprintf("Warmup done (%.1fs)", warmupDur))
	}

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

	runTimestamp := time.Now().Format("2006-01-02_1504")
	runDir := filepath.Join(cfg.ResultsDir, runTimestamp)
	if err := os.MkdirAll(runDir, 0755); err != nil {
		return nil, fmt.Errorf("cannot create run dir: %w", err)
	}

	latestLink := filepath.Join(cfg.ResultsDir, "latest")
	os.Remove(latestLink)
	os.Symlink(runTimestamp, latestLink)

	ui.Info(fmt.Sprintf("Run: %s", runTimestamp))
	ui.Info(fmt.Sprintf("Model: %s", model))
	ui.Info(fmt.Sprintf("Suite: %s (%s)", fmtSuite.Name, fmtSuite.Format))
	ui.Info(fmt.Sprintf("Problems: %d", len(problems)))

	fmt.Printf("\n%s\n%s — %d problems\n%s\n",
		ui.BoldStyle.Render(strings.Repeat("═", 60)),
		model, len(problems),
		ui.BoldStyle.Render(strings.Repeat("═", 60)))

	var results []Result
	passCount := 0
	failCount := 0

	for i, problem := range problems {
		r := runSingle(model, problem, fmtSuite, format, cfg, runDir, i+1, len(problems))
		results = append(results, r)
		if r.Success {
			passCount++
		} else {
			failCount++
		}
	}

	total := passCount + failCount
	rate := float64(0)
	if total > 0 {
		rate = float64(passCount) / float64(total) * 100
	}
	fmt.Printf("\n  %s %s %s %s\n",
		ui.BoldStyle.Render(fmt.Sprintf("Done: %d/%d", total, len(problems))),
		ui.SuccessStyle.Render(fmt.Sprintf("Pass: %d", passCount)),
		ui.ErrorStyle.Render(fmt.Sprintf("Fail: %d", failCount)),
		ui.AccentStyle.Render(fmt.Sprintf("Rate: %.1f%%", rate)))

	summaryPath := filepath.Join(runDir, "summary.json")
	data, _ := json.MarshalIndent(results, "", "  ")
	os.WriteFile(summaryPath, data, 0644)

	return results, nil
}

func runSingle(model string, problem formats.Problem, suite formats.Suite, format formats.FormatRunner, cfg RunConfig, runDir string, idx, total int) Result {
	slug := regexp.MustCompile(`[/:]`).ReplaceAllString(model, "_")
	taskSlug := regexp.MustCompile(`[/:]`).ReplaceAllString(problem.TaskID, "_")
	outputDir := filepath.Join(runDir, fmt.Sprintf("%s__%s", suite.Name, slug))
	if taskSlug != "webgen" {
		outputDir = filepath.Join(outputDir, "problems", taskSlug)
	}
	os.MkdirAll(outputDir, 0755)
	workDir := filepath.Join(outputDir, "project")

	counter := ui.MutedStyle.Render(fmt.Sprintf("[%3d/%d]", idx, total))
	taskName := problem.TaskID
	if len(taskName) > 30 {
		taskName = taskName[:30]
	}
	dotsLen := 40 - len(taskName)
	if dotsLen < 3 {
		dotsLen = 3
	}
	dots := ui.MutedStyle.Render(strings.Repeat("·", dotsLen))
	fmt.Printf("  %s %s %s ", counter, taskName, dots)

	var (
		attempt          int
		success          bool
		totalDuration    int
		filesCreated     int
		promptTokens     int
		completionTokens int
		promptTokPerSec  float64
		genTokPerSec     float64
		ttft             float64
	)

	prompt, err := format.BuildPrompt(problem, suite)
	if err != nil {
		ui.Fail(fmt.Sprintf("Cannot build prompt: %v", err))
		return Result{Model: model, Suite: suite.Name, TaskID: problem.TaskID, Error: err.Error()}
	}

	currentPrompt := prompt
	os.WriteFile(filepath.Join(outputDir, "prompt.txt"), []byte(prompt), 0644)

	for attempt < cfg.MaxRetries && !success {
		attempt++
		ui.Logger.Debug("attempt", "num", attempt, "max", cfg.MaxRetries)

		start := time.Now()
		stats, genErr := llamaserver.GenerateWithStats(currentPrompt, cfg.MaxTokens, cfg.Temperature, cfg.GenTimeout)
		duration := int(time.Since(start).Seconds())
		totalDuration += duration

		if genErr != nil {
			ui.Logger.Debug("generation error", "duration", duration, "err", genErr)
			continue
		}

		promptTokens = stats.PromptTokens
		completionTokens = stats.CompletionTokens
		if stats.PromptTokPerSec > 0 {
			promptTokPerSec = stats.PromptTokPerSec
		}
		if stats.GenTokPerSec > 0 {
			genTokPerSec = stats.GenTokPerSec
		}
		if stats.TTFT > 0 {
			ttft = stats.TTFT
		}

		ui.Logger.Debug("response", "chars", len(stats.Content), "duration", duration,
			"prompt_tok/s", fmt.Sprintf("%.1f", promptTokPerSec),
			"gen_tok/s", fmt.Sprintf("%.1f", genTokPerSec))
		os.WriteFile(filepath.Join(outputDir, fmt.Sprintf("raw_output_%d.txt", attempt)), []byte(stats.Content), 0644)

		if attempt == 1 {
			os.RemoveAll(workDir)
		}
		os.MkdirAll(workDir, 0755)

		ok, validationErr, err := format.ValidateResponse(stats.Content, problem, workDir)
		if err != nil {
			ui.Logger.Debug("validation error", "err", err)
			continue
		}

		fileCount := 0
		filepath.Walk(workDir, func(_ string, info os.FileInfo, _ error) error {
			if info != nil && !info.IsDir() {
				fileCount++
			}
			return nil
		})
		filesCreated = fileCount

		if ok {
			ui.Logger.Debug("validation passed", "files", fileCount)
			success = true
		} else {
			ui.Logger.Debug("validation failed", "error", truncateValidationErr(validationErr))
			if attempt < cfg.MaxRetries {
				retryPrompt, err := format.BuildRetryPrompt(problem, workDir, validationErr)
				if err != nil {
					ui.Logger.Debug("retry prompt error", "err", err)
					continue
				}
				if retryPrompt != "" {
					currentPrompt = retryPrompt
				}
			}
		}
	}

	result := Result{
		Model:            model,
		Suite:            suite.Name,
		TaskID:           problem.TaskID,
		Attempts:         attempt,
		Success:          success,
		Duration:         totalDuration,
		FilesCreated:     filesCreated,
		PromptTokens:     promptTokens,
		CompletionTokens: completionTokens,
		PromptTokPerSec:  promptTokPerSec,
		GenTokPerSec:     genTokPerSec,
		TTFT:             ttft,
		Timestamp:        time.Now().Format(time.RFC3339),
	}

	data, _ := json.MarshalIndent(result, "", "  ")
	os.WriteFile(filepath.Join(outputDir, "metadata.json"), data, 0644)

	perfInfo := ""
	if genTokPerSec > 0 {
		perfInfo = fmt.Sprintf(" [%dt %.1f tok/s]", completionTokens, genTokPerSec)
	}

	if success {
		fmt.Printf("%s %s%s\n",
			ui.SuccessStyle.Render("PASS"),
			ui.MutedStyle.Render(fmt.Sprintf("%ds", totalDuration)),
			ui.MutedStyle.Render(perfInfo))
	} else {
		extra := ""
		if attempt > 1 {
			extra = fmt.Sprintf(" (%d attempts)", attempt)
		}
		fmt.Printf("%s %s%s%s\n",
			ui.ErrorStyle.Render("FAIL"),
			ui.MutedStyle.Render(fmt.Sprintf("%ds", totalDuration)),
			ui.MutedStyle.Render(perfInfo),
			ui.MutedStyle.Render(extra))
	}

	return result
}

func detectRunningModel(host string) string {
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get(host + "/v1/models")
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return ""
	}
	var result struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if json.Unmarshal(body, &result) == nil && len(result.Data) > 0 {
		return result.Data[0].ID
	}
	return ""
}

func median(vals []float64) float64 {
	if len(vals) == 0 {
		return 0
	}
	sorted := make([]float64, len(vals))
	copy(sorted, vals)
	sort.Float64s(sorted)
	n := len(sorted)
	if n%2 == 0 {
		return (sorted[n/2-1] + sorted[n/2]) / 2
	}
	return sorted[n/2]
}

func truncateValidationErr(s string) string {
	if len(s) > 100 {
		return s[:100] + "..."
	}
	return s
}

func PrintSummary(results []Result) {
	fmt.Printf("\n%s\n", ui.BoldStyle.Render(strings.Repeat("═", 70)))
	fmt.Printf("%s\n", ui.BoldStyle.Render("BENCHMARK RESULTS"))
	fmt.Printf("%s\n\n", ui.BoldStyle.Render(strings.Repeat("═", 70)))

	detailTbl := ui.NewTable("Per-Problem Results", "TASK", "PASS", "ATT", "TIME", "PROMPT", "GEN", "P tok/s", "G tok/s", "TTFT")
	for _, r := range results {
		task := r.TaskID
		if len(task) > 25 {
			task = task[len(task)-25:]
		}
		passStr := ui.ErrorStyle.Render("FAIL")
		if r.Success {
			passStr = ui.SuccessStyle.Render("PASS")
		}
		promptTok := "-"
		if r.PromptTokens > 0 {
			promptTok = fmt.Sprintf("%d", r.PromptTokens)
		}
		genTok := "-"
		if r.CompletionTokens > 0 {
			genTok = fmt.Sprintf("%d", r.CompletionTokens)
		}
		pTokS := "-"
		if r.PromptTokPerSec > 0 {
			pTokS = fmt.Sprintf("%.1f", r.PromptTokPerSec)
		}
		gTokS := "-"
		if r.GenTokPerSec > 0 {
			gTokS = fmt.Sprintf("%.1f", r.GenTokPerSec)
		}
		ttftStr := "-"
		if r.TTFT > 0 {
			ttftStr = fmt.Sprintf("%.0fms", r.TTFT)
		}
		detailTbl.AddRow(task, passStr, fmt.Sprintf("%d", r.Attempts),
			fmt.Sprintf("%ds", r.Duration), promptTok, genTok, pTokS, gTokS, ttftStr)
	}
	detailTbl.Print()

	fmt.Printf("\n%s\n", ui.BoldStyle.Render("SUMMARY"))
	fmt.Printf("%s\n\n", ui.BoldStyle.Render(strings.Repeat("─", 70)))

	type modelStats struct {
		model, suite      string
		pass, fail, total int
		totalTime         int
		genTokSamples     []float64
		promptTokSamples  []float64
		ttftSamples       []float64
		totalPromptTok    int
		totalGenTok       int
	}
	statsMap := make(map[string]*modelStats)
	var order []string

	for _, r := range results {
		key := fmt.Sprintf("%s__%s", r.Suite, r.Model)
		s, ok := statsMap[key]
		if !ok {
			s = &modelStats{model: r.Model, suite: r.Suite}
			statsMap[key] = s
			order = append(order, key)
		}
		s.total++
		s.totalTime += r.Duration
		s.totalPromptTok += r.PromptTokens
		s.totalGenTok += r.CompletionTokens
		if r.GenTokPerSec > 0 {
			s.genTokSamples = append(s.genTokSamples, r.GenTokPerSec)
		}
		if r.PromptTokPerSec > 0 {
			s.promptTokSamples = append(s.promptTokSamples, r.PromptTokPerSec)
		}
		if r.TTFT > 0 {
			s.ttftSamples = append(s.ttftSamples, r.TTFT)
		}
		if r.Success {
			s.pass++
		} else {
			s.fail++
		}
	}

	tbl := ui.NewTable("Results by Model", "SUITE", "MODEL", "PASS", "FAIL", "RATE", "GEN tok/s", "TTFT", "TIME")
	for _, key := range order {
		s := statsMap[key]
		rate := float64(0)
		if s.total > 0 {
			rate = float64(s.pass) / float64(s.total) * 100
		}
		model := s.model
		if len(model) > 35 {
			model = model[:35]
		}

		rateStr := fmt.Sprintf("%.1f%%", rate)
		if rate >= 80 {
			rateStr = ui.SuccessStyle.Render(rateStr)
		} else if rate >= 50 {
			rateStr = ui.WarningStyle.Render(rateStr)
		} else {
			rateStr = ui.ErrorStyle.Render(rateStr)
		}

		genStr := "-"
		if len(s.genTokSamples) > 0 {
			avg := median(s.genTokSamples)
			genStr = fmt.Sprintf("%.1f", avg)
		}
		ttftStr := "-"
		if len(s.ttftSamples) > 0 {
			avg := median(s.ttftSamples)
			ttftStr = fmt.Sprintf("%.0fms", avg)
		}

		tbl.AddRow(s.suite, model,
			ui.SuccessStyle.Render(fmt.Sprintf("%d", s.pass)),
			ui.ErrorStyle.Render(fmt.Sprintf("%d", s.fail)),
			rateStr,
			genStr,
			ttftStr,
			fmt.Sprintf("%ds", s.totalTime))
	}
	tbl.Print()

	totalPass := 0
	totalFail := 0
	totalTime := 0
	var allGenTok, allPromptTok, allTTFT []float64
	totalPromptTokens := 0
	totalGenTokens := 0
	for _, s := range statsMap {
		totalPass += s.pass
		totalFail += s.fail
		totalTime += s.totalTime
		totalPromptTokens += s.totalPromptTok
		totalGenTokens += s.totalGenTok
		allGenTok = append(allGenTok, s.genTokSamples...)
		allPromptTok = append(allPromptTok, s.promptTokSamples...)
		allTTFT = append(allTTFT, s.ttftSamples...)
	}
	totalAll := totalPass + totalFail
	overallRate := float64(0)
	if totalAll > 0 {
		overallRate = float64(totalPass) / float64(totalAll) * 100
	}

	fmt.Printf("  %s  %s  %s  %s  %s\n",
		ui.BoldStyle.Render("Overall:"),
		ui.SuccessStyle.Render(fmt.Sprintf("Pass: %d", totalPass)),
		ui.ErrorStyle.Render(fmt.Sprintf("Fail: %d", totalFail)),
		ui.AccentStyle.Render(fmt.Sprintf("Rate: %.1f%%", overallRate)),
		ui.MutedStyle.Render(fmt.Sprintf("Time: %ds", totalTime)))

	if len(allGenTok) > 0 {
		fmt.Printf("  %s  Prompt: %d tok  Gen: %d tok  Gen median: %.1f tok/s",
			ui.BoldStyle.Render("Tokens:"),
			totalPromptTokens, totalGenTokens, median(allGenTok))
		if len(allPromptTok) > 0 {
			fmt.Printf("  Prompt median: %.1f tok/s", median(allPromptTok))
		}
		if len(allTTFT) > 0 {
			fmt.Printf("  TTFT median: %.0fms", median(allTTFT))
		}
		fmt.Println()
	}
	fmt.Println()
}
