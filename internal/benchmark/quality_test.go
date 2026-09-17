package benchmark

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/jparrill/auriga-cli/internal/benchmark/formats"
	"github.com/jparrill/auriga-cli/internal/exec"
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
	ok, _ := runBuildCheckWith(workDir, formats.Problem{}, fakeSandboxRunner("", nil))
	if !ok {
		t.Error("expected pass when no build system detected")
	}
}

func TestRunTestCheck_NoBuildSystem(t *testing.T) {
	workDir := t.TempDir()
	ok, _ := runTestCheckWith(workDir, formats.Problem{}, fakeSandboxRunner("", nil))
	if !ok {
		t.Error("expected pass when no test system detected")
	}
}

func TestRunTestCheck_CustomCmd(t *testing.T) {
	workDir := t.TempDir()
	problem := formats.Problem{TestCmd: "true"}
	ok, _ := runTestCheckWith(workDir, problem, fakeSandboxRunner("", nil))
	if !ok {
		t.Error("expected pass for 'true' command")
	}
}

func TestRunBuildCheck_UsesDetectedGoProject(t *testing.T) {
	workDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(workDir, "go.mod"), []byte("module example.com/test"), 0644); err != nil {
		t.Fatal(err)
	}

	var gotName string
	var gotArgs []string
	var gotOpts exec.SandboxOpts
	ok, errMsg := runBuildCheckWith(workDir, formats.Problem{}, func(_ context.Context, name string, args []string, opts exec.SandboxOpts) (string, error) {
		gotName, gotArgs, gotOpts = name, args, opts
		return "", nil
	})
	if !ok || errMsg != "" {
		t.Fatalf("expected build check to pass, got ok=%v err=%q", ok, errMsg)
	}
	if gotName != "go" || !reflect.DeepEqual(gotArgs, []string{"build", "./..."}) {
		t.Errorf("expected go build ./..., got %s %v", gotName, gotArgs)
	}
	if gotOpts.Image != exec.ImageGo || gotOpts.Dir != workDir {
		t.Errorf("unexpected sandbox options: %+v", gotOpts)
	}
}

func TestRunTestCheck_UsesCustomCommand(t *testing.T) {
	workDir := t.TempDir()
	problem := formats.Problem{TestCmd: "go test ./pkg"}

	var gotName string
	var gotArgs []string
	ok, errMsg := runTestCheckWith(workDir, problem, func(_ context.Context, name string, args []string, _ exec.SandboxOpts) (string, error) {
		gotName, gotArgs = name, args
		return "", nil
	})
	if !ok || errMsg != "" {
		t.Fatalf("expected custom test command to pass, got ok=%v err=%q", ok, errMsg)
	}
	if gotName != "go" || !reflect.DeepEqual(gotArgs, []string{"test", "./pkg"}) {
		t.Errorf("expected go test ./pkg, got %s %v", gotName, gotArgs)
	}
}

func TestRunTestCheck_EmptyCustomCommand(t *testing.T) {
	workDir := t.TempDir()
	ok, errMsg := runTestCheckWith(workDir, formats.Problem{TestCmd: "   "}, fakeSandboxRunner("", nil))
	if ok || errMsg != "empty test command" {
		t.Errorf("expected empty command error, got ok=%v err=%q", ok, errMsg)
	}
}

func TestRunTestCheck_ReportsSandboxFailure(t *testing.T) {
	workDir := t.TempDir()
	problem := formats.Problem{TestCmd: "go test ./..."}
	runErr := errors.New("exit status 1")
	ok, errMsg := runTestCheckWith(workDir, problem, fakeSandboxRunner("FAIL test_add", runErr))
	if ok {
		t.Fatal("expected test check to fail")
	}
	if !strings.Contains(errMsg, "FAIL test_add") {
		t.Errorf("expected command output in error, got %q", errMsg)
	}
}

func fakeSandboxRunner(output string, runErr error) sandboxRunner {
	return func(context.Context, string, []string, exec.SandboxOpts) (string, error) {
		return output, runErr
	}
}
