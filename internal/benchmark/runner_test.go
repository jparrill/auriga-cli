package benchmark

import (
	"os"
	"testing"

	"github.com/jparrill/auriga-cli/internal/ui"
)

func TestMain(m *testing.M) {
	ui.InitLogger(false)
	os.Exit(m.Run())
}

func TestPrintSummary(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping visual output tests (run with: go test -run TestPrintSummary)")
	}

	tests := []struct {
		name    string
		results []Result
	}{
		{
			name: "When results have mixed pass and fail it should show both counts",
			results: []Result{
				{Model: "gemma4:26b", Backend: "ollama", Suite: "humaneval", TaskID: "HumanEval/0", Success: true, Duration: 15, Attempts: 1},
				{Model: "gemma4:26b", Backend: "ollama", Suite: "humaneval", TaskID: "HumanEval/1", Success: true, Duration: 10, Attempts: 1},
				{Model: "gemma4:26b", Backend: "ollama", Suite: "humaneval", TaskID: "HumanEval/2", Success: false, Duration: 30, Attempts: 3},
				{Model: "gemma4:26b", Backend: "ollama", Suite: "humaneval", TaskID: "HumanEval/3", Success: true, Duration: 8, Attempts: 1},
				{Model: "gemma4:26b", Backend: "ollama", Suite: "humaneval", TaskID: "HumanEval/4", Success: false, Duration: 45, Attempts: 3},
			},
		},
		{
			name: "When all problems pass it should show 100 percent rate",
			results: func() []Result {
				var r []Result
				for i := 0; i < 10; i++ {
					r = append(r, Result{
						Model: "gpt-oss:20b", Backend: "ollama", Suite: "humaneval",
						TaskID: "HumanEval/" + string(rune('0'+i)), Success: true, Duration: 10, Attempts: 1,
					})
				}
				return r
			}(),
		},
		{
			name: "When all problems fail it should show 0 percent rate",
			results: []Result{
				{Model: "deepseek-r1:14b", Backend: "ollama", Suite: "coding-quality", TaskID: "L1-01", Success: false, Duration: 120, Attempts: 5},
				{Model: "deepseek-r1:14b", Backend: "ollama", Suite: "coding-quality", TaskID: "L2-01", Success: false, Duration: 90, Attempts: 5},
			},
		},
		{
			name: "When multiple models are benchmarked it should show per-model stats",
			results: []Result{
				{Model: "gemma4:26b", Backend: "ollama", Suite: "humaneval", TaskID: "HumanEval/0", Success: true, Duration: 15, Attempts: 1},
				{Model: "gemma4:26b", Backend: "ollama", Suite: "humaneval", TaskID: "HumanEval/1", Success: false, Duration: 30, Attempts: 3},
				{Model: "qwen3.6:27b", Backend: "ollama", Suite: "humaneval", TaskID: "HumanEval/0", Success: true, Duration: 20, Attempts: 1},
				{Model: "qwen3.6:27b", Backend: "ollama", Suite: "humaneval", TaskID: "HumanEval/1", Success: true, Duration: 12, Attempts: 1},
				{Model: "ornith-1.0", Backend: "llama-server", Suite: "humaneval", TaskID: "HumanEval/0", Success: true, Duration: 8, Attempts: 1},
				{Model: "ornith-1.0", Backend: "llama-server", Suite: "humaneval", TaskID: "HumanEval/1", Success: true, Duration: 9, Attempts: 1},
			},
		},
		{
			name:    "When no results are provided it should not panic",
			results: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("PrintSummary panicked: %v", r)
				}
			}()
			PrintSummary(tt.results)
		})
	}
}

func TestTruncateValidationErr(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		maxLen   int
	}{
		{
			name:   "When error is short it should return unchanged",
			input:  "small error",
			maxLen: 11,
		},
		{
			name:   "When error exceeds 100 chars it should truncate with ellipsis",
			input:  func() string { s := ""; for i := 0; i < 200; i++ { s += "x" }; return s }(),
			maxLen: 103,
		},
		{
			name:   "When error is exactly 100 chars it should return unchanged",
			input:  func() string { s := ""; for i := 0; i < 100; i++ { s += "y" }; return s }(),
			maxLen: 100,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := truncateValidationErr(tt.input)
			if len(result) != tt.maxLen {
				t.Errorf("expected length %d, got %d", tt.maxLen, len(result))
			}
		})
	}
}
