package benchmark

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jparrill/auriga-cli/internal/benchmark/formats"
)

func TestQualityRunner_BuildPrompt(t *testing.T) {
	runner := &QualityRunner{}
	problem := formats.Problem{
		TaskID: "L1-01",
		Level:  "L1-design",
		Prompt: "Design a REST API for a task manager",
	}
	suite := formats.Suite{Language: "go"}

	prompt, err := runner.BuildPrompt(problem, suite)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(prompt, "expert software engineer") {
		t.Error("missing system prompt")
	}
	if !strings.Contains(prompt, "L1-design") {
		t.Error("missing difficulty level")
	}
	if !strings.Contains(prompt, "REST API") {
		t.Error("missing task content")
	}
	if !strings.Contains(prompt, "go") {
		t.Error("missing language")
	}
}

func TestQualityRunner_ValidateResponse_WithFiles(t *testing.T) {
	runner := &QualityRunner{}
	problem := formats.Problem{
		TaskID: "test",
		Eval:   []string{},
	}

	workDir := t.TempDir()
	response := "--- FILE: main.py ---\nprint('hello')\n--- END FILE ---"

	ok, errMsg, err := runner.ValidateResponse(response, problem, workDir)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Errorf("expected pass (no evals), got fail: %s", errMsg)
	}
}

func TestQualityRunner_ValidateResponse_NoFiles(t *testing.T) {
	runner := &QualityRunner{}
	problem := formats.Problem{TaskID: "test"}
	workDir := t.TempDir()

	ok, errMsg, _ := runner.ValidateResponse("just some text", problem, workDir)
	if ok {
		t.Error("expected fail for no files")
	}
	if errMsg != "no_files" {
		t.Errorf("expected 'no_files', got %q", errMsg)
	}
}

func TestQualityRunner_BuildRetryPrompt_NoFiles(t *testing.T) {
	runner := &QualityRunner{}
	problem := formats.Problem{Prompt: "original task"}

	result, err := runner.BuildRetryPrompt(problem, t.TempDir(), "no_files")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "FORMAT REQUIREMENT") {
		t.Error("expected format retry prompt")
	}
}

func TestQualityRunner_BuildRetryPrompt_BuildFail(t *testing.T) {
	runner := &QualityRunner{}
	problem := formats.Problem{Prompt: "build a thing"}
	workDir := t.TempDir()
	os.WriteFile(filepath.Join(workDir, "main.go"), []byte("package main"), 0644)

	result, err := runner.BuildRetryPrompt(problem, workDir, "build_fail:syntax error")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "Build failed") {
		t.Error("expected build error context")
	}
	if !strings.Contains(result, "syntax error") {
		t.Error("expected error detail")
	}
	if !strings.Contains(result, "main.go") {
		t.Error("expected current files")
	}
}

func TestRunBuildCheck_NoProject(t *testing.T) {
	workDir := t.TempDir()
	ok, _ := runBuildCheck(workDir, formats.Problem{})
	if !ok {
		t.Error("expected pass when no build system detected")
	}
}

func TestRunTestCheck_NoBuildSystem(t *testing.T) {
	workDir := t.TempDir()
	ok, _ := runTestCheck(workDir, formats.Problem{})
	if !ok {
		t.Error("expected pass when no test system detected")
	}
}

func TestRunTestCheck_CustomCmd(t *testing.T) {
	workDir := t.TempDir()
	problem := formats.Problem{TestCmd: "true"}
	ok, _ := runTestCheck(workDir, problem)
	if !ok {
		t.Error("expected pass for 'true' command")
	}
}
