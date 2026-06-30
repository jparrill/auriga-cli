package benchmark

import (
	"strings"
	"testing"

	"github.com/jparrill/auriga-cli/internal/benchmark/formats"
)

func TestCleanPythonResponse_Plain(t *testing.T) {
	problem := formats.Problem{EntryPoint: "add"}
	response := "    return a + b"
	result := cleanPythonResponse(response, problem)
	if !strings.Contains(result, "return a + b") {
		t.Errorf("unexpected result: %q", result)
	}
}

func TestCleanPythonResponse_WithCodeBlock(t *testing.T) {
	problem := formats.Problem{EntryPoint: "add"}
	response := "```python\n    return a + b\n```"
	result := cleanPythonResponse(response, problem)
	if !strings.Contains(result, "return a + b") {
		t.Errorf("unexpected result: %q", result)
	}
	if strings.Contains(result, "```") {
		t.Error("code block markers should be stripped")
	}
}

func TestCleanPythonResponse_WithSignature(t *testing.T) {
	problem := formats.Problem{EntryPoint: "add"}
	response := "def add(a, b):\n    return a + b"
	result := cleanPythonResponse(response, problem)
	if strings.Contains(result, "def add") {
		t.Error("function signature should be stripped")
	}
	if !strings.Contains(result, "return a + b") {
		t.Errorf("body should be preserved: %q", result)
	}
}

func TestCleanPythonResponse_WithThinking(t *testing.T) {
	problem := formats.Problem{EntryPoint: "add"}
	response := "<think>\nI need to add two numbers\n</think>\n    return a + b"
	result := cleanPythonResponse(response, problem)
	if strings.Contains(result, "think") {
		t.Error("thinking block should be stripped")
	}
	if !strings.Contains(result, "return a + b") {
		t.Errorf("body should be preserved: %q", result)
	}
}

func TestCleanPythonResponse_NoIndent(t *testing.T) {
	problem := formats.Problem{EntryPoint: "add"}
	response := "return a + b"
	result := cleanPythonResponse(response, problem)
	if !strings.Contains(result, "    return a + b") {
		t.Errorf("should add 4-space indent: %q", result)
	}
}

func TestSanitizeTaskID(t *testing.T) {
	tests := []struct{ in, out string }{
		{"HumanEval/0", "HumanEval_0"},
		{"test/foo:bar", "test_foo_bar"},
		{"simple", "simple"},
	}
	for _, tt := range tests {
		result := sanitizeTaskID(tt.in)
		if result != tt.out {
			t.Errorf("sanitizeTaskID(%q) = %q, want %q", tt.in, result, tt.out)
		}
	}
}

func TestHumanEvalRunner_BuildPrompt(t *testing.T) {
	runner := &HumanEvalRunner{}
	problem := formats.Problem{
		TaskID: "HumanEval/0",
		Prompt: "def add(a: int, b: int) -> int:\n    \"\"\"Add two numbers\"\"\"\n",
	}

	prompt, err := runner.BuildPrompt(problem, formats.Suite{})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(prompt, "def add") {
		t.Error("prompt should contain function signature")
	}
	if !strings.Contains(prompt, "expert Python programmer") {
		t.Error("prompt should contain system instructions")
	}
}

func TestHumanEvalRunner_ValidateResponse_Pass(t *testing.T) {
	runner := &HumanEvalRunner{}
	problem := formats.Problem{
		TaskID:     "test/add",
		Prompt:     "def add(a: int, b: int) -> int:\n    \"\"\"Add two numbers.\"\"\"\n",
		Test:       "\ndef check(candidate):\n    assert candidate(1, 2) == 3\n    assert candidate(0, 0) == 0\n",
		EntryPoint: "add",
	}

	workDir := t.TempDir()
	ok, errMsg, err := runner.ValidateResponse("    return a + b", problem, workDir)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Errorf("expected pass, got fail: %s", errMsg)
	}
}

func TestHumanEvalRunner_ValidateResponse_Fail(t *testing.T) {
	runner := &HumanEvalRunner{}
	problem := formats.Problem{
		TaskID:     "test/add",
		Prompt:     "def add(a: int, b: int) -> int:\n    \"\"\"Add two numbers.\"\"\"\n",
		Test:       "\ndef check(candidate):\n    assert candidate(1, 2) == 3\n",
		EntryPoint: "add",
	}

	workDir := t.TempDir()
	ok, _, err := runner.ValidateResponse("    return a - b", problem, workDir)
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Error("expected fail for wrong implementation")
	}
}
