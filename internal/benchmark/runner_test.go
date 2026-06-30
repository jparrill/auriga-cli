package benchmark

import (
	"testing"

	"github.com/jparrill/auriga-cli/internal/ui"
)

func TestMain(m *testing.M) {
	ui.InitLogger(false)
	m.Run()
}

func TestPrintSummary_Mixed(t *testing.T) {
	results := []Result{
		{Model: "gemma4:26b", Backend: "ollama", Suite: "humaneval", TaskID: "HumanEval/0", Success: true, Duration: 15, FilesCreated: 1, Attempts: 1},
		{Model: "gemma4:26b", Backend: "ollama", Suite: "humaneval", TaskID: "HumanEval/1", Success: true, Duration: 10, FilesCreated: 1, Attempts: 1},
		{Model: "gemma4:26b", Backend: "ollama", Suite: "humaneval", TaskID: "HumanEval/2", Success: false, Duration: 30, FilesCreated: 1, Attempts: 3},
		{Model: "gemma4:26b", Backend: "ollama", Suite: "humaneval", TaskID: "HumanEval/3", Success: true, Duration: 8, FilesCreated: 1, Attempts: 1},
		{Model: "gemma4:26b", Backend: "ollama", Suite: "humaneval", TaskID: "HumanEval/4", Success: false, Duration: 45, FilesCreated: 0, Attempts: 3},
	}

	// Should not panic
	PrintSummary(results)
}

func TestPrintSummary_AllPass(t *testing.T) {
	var results []Result
	for i := 0; i < 10; i++ {
		results = append(results, Result{
			Model: "gpt-oss:20b", Backend: "ollama", Suite: "humaneval",
			TaskID: "HumanEval/" + string(rune('0'+i)), Success: true, Duration: 10, Attempts: 1,
		})
	}
	PrintSummary(results)
}

func TestPrintSummary_AllFail(t *testing.T) {
	results := []Result{
		{Model: "deepseek-r1:14b", Backend: "ollama", Suite: "coding-quality", TaskID: "L1-01", Success: false, Duration: 120, Attempts: 5},
		{Model: "deepseek-r1:14b", Backend: "ollama", Suite: "coding-quality", TaskID: "L2-01", Success: false, Duration: 90, Attempts: 5},
	}
	PrintSummary(results)
}

func TestPrintSummary_MultiModel(t *testing.T) {
	results := []Result{
		{Model: "gemma4:26b", Backend: "ollama", Suite: "humaneval", TaskID: "HumanEval/0", Success: true, Duration: 15, Attempts: 1},
		{Model: "gemma4:26b", Backend: "ollama", Suite: "humaneval", TaskID: "HumanEval/1", Success: true, Duration: 10, Attempts: 1},
		{Model: "gemma4:26b", Backend: "ollama", Suite: "humaneval", TaskID: "HumanEval/2", Success: false, Duration: 30, Attempts: 3},
		{Model: "qwen3.6:27b", Backend: "ollama", Suite: "humaneval", TaskID: "HumanEval/0", Success: true, Duration: 20, Attempts: 1},
		{Model: "qwen3.6:27b", Backend: "ollama", Suite: "humaneval", TaskID: "HumanEval/1", Success: false, Duration: 25, Attempts: 2},
		{Model: "qwen3.6:27b", Backend: "ollama", Suite: "humaneval", TaskID: "HumanEval/2", Success: true, Duration: 12, Attempts: 1},
		{Model: "ornith-1.0", Backend: "llama-server", Suite: "humaneval", TaskID: "HumanEval/0", Success: true, Duration: 8, Attempts: 1},
		{Model: "ornith-1.0", Backend: "llama-server", Suite: "humaneval", TaskID: "HumanEval/1", Success: true, Duration: 9, Attempts: 1},
		{Model: "ornith-1.0", Backend: "llama-server", Suite: "humaneval", TaskID: "HumanEval/2", Success: true, Duration: 7, Attempts: 1},
	}
	PrintSummary(results)
}

func TestPrintSummary_Empty(t *testing.T) {
	PrintSummary(nil)
}

func TestTruncateValidationErr(t *testing.T) {
	short := "small error"
	if truncateValidationErr(short) != short {
		t.Error("should not truncate short strings")
	}

	long := ""
	for i := 0; i < 200; i++ {
		long += "x"
	}
	result := truncateValidationErr(long)
	if len(result) != 103 { // 100 + "..."
		t.Errorf("expected 103 chars, got %d", len(result))
	}
}
