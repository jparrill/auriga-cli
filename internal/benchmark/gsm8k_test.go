package benchmark

import (
	"strings"
	"testing"

	"github.com/jparrill/auriga-cli/internal/benchmark/formats"
)

func TestGSM8KBuildPrompt(t *testing.T) {
	runner := &GSM8KRunner{}
	problem := formats.Problem{
		TaskID: "1",
		Prompt: "Janet's ducks lay 16 eggs per day. She eats three for breakfast every morning and bakes muffins for her friends every day with four. She sells every duck egg at the farmers' market daily for $2. How much in dollars does she make every day at the farmers' market?",
	}

	prompt, err := runner.BuildPrompt(problem, formats.Suite{})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(prompt, "Janet's ducks") {
		t.Error("prompt should contain the question")
	}
	if !strings.Contains(prompt, "#### <number>") {
		t.Error("prompt should instruct the model to use #### format")
	}
}

func TestGSM8KValidateResponse_Correct(t *testing.T) {
	runner := &GSM8KRunner{}
	problem := formats.Problem{
		TaskID: "1",
		Prompt: "Janet's ducks lay 16 eggs per day.",
		Test:   "Janet sells 16 - 3 - 4 = <<16-3-4=9>>9 duck eggs a day.\nShe makes 9 * 2 = $<<9*2=18>>18 every day.\n#### 18",
	}

	ok, errMsg, err := runner.ValidateResponse("Step 1: 16 - 3 - 4 = 9 eggs sold\nStep 2: 9 * 2 = 18 dollars\n#### 18", problem, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Errorf("expected pass, got fail: %s", errMsg)
	}
}

func TestGSM8KValidateResponse_Wrong(t *testing.T) {
	runner := &GSM8KRunner{}
	problem := formats.Problem{
		TaskID: "1",
		Prompt: "Janet's ducks lay 16 eggs per day.",
		Test:   "She makes 9 * 2 = $<<9*2=18>>18 every day.\n#### 18",
	}

	ok, errMsg, err := runner.ValidateResponse("I think the answer is\n#### 20", problem, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Error("expected fail for wrong answer")
	}
	if !strings.Contains(errMsg, "expected 18") || !strings.Contains(errMsg, "got 20") {
		t.Errorf("error message should mention expected and got, got: %s", errMsg)
	}
}

func TestGSM8KValidateResponse_NoAnswer(t *testing.T) {
	runner := &GSM8KRunner{}
	problem := formats.Problem{
		TaskID: "1",
		Prompt: "Janet's ducks lay 16 eggs per day.",
		Test:   "#### 18",
	}

	ok, errMsg, err := runner.ValidateResponse("The answer is 18 dollars.", problem, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Error("expected fail when model doesn't use #### format")
	}
	if !strings.Contains(errMsg, "missing") {
		t.Errorf("error message should mention missing marker, got: %s", errMsg)
	}
}

func TestGSM8KExtractAnswer(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"simple", "#### 18", "18"},
		{"with_comma", "#### 1,000", "1000"},
		{"with_dollar", "#### $42", "42"},
		{"trailing_dot", "#### 18.", "18"},
		{"negative", "#### -5", "-5"},
		{"multiline_last_wins", "#### 10\nsome text\n#### 18", "18"},
		{"no_marker", "the answer is 18", ""},
		{"with_spaces", "####   42  ", "42"},
		{"combo", "#### $1,234.", "1234"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			raw := extractGSM8KAnswer(tt.input)
			got := normalizeNumber(raw)
			if tt.expected == "" {
				if raw != "" {
					t.Errorf("extractGSM8KAnswer(%q) = %q, want empty", tt.input, raw)
				}
				return
			}
			if got != tt.expected {
				t.Errorf("normalizeNumber(extractGSM8KAnswer(%q)) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestGSM8KBuildRetryPrompt(t *testing.T) {
	runner := &GSM8KRunner{}
	problem := formats.Problem{
		TaskID: "1",
		Prompt: "Janet's ducks lay 16 eggs per day.",
	}

	prompt, err := runner.BuildRetryPrompt(problem, t.TempDir(), "expected 18 got 20")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(prompt, "expected 18 got 20") {
		t.Error("retry prompt should include the validation error")
	}
	if !strings.Contains(prompt, "Janet's ducks") {
		t.Error("retry prompt should include the original question")
	}
}
