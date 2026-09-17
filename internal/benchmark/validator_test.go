package benchmark

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/jparrill/auriga-cli/internal/exec"
)

func TestTruncate_Short(t *testing.T) {
	result := truncate("hello", 10)
	if result != "hello" {
		t.Errorf("expected 'hello', got %q", result)
	}
}

func TestTruncate_Long(t *testing.T) {
	input := "this is a very long string that should be truncated"
	result := truncate(input, 10)
	if len(result) != 10 {
		t.Errorf("expected length 10, got %d", len(result))
	}
	expected := input[len(input)-10:]
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestTruncate_Exact(t *testing.T) {
	result := truncate("12345", 5)
	if result != "12345" {
		t.Errorf("expected '12345', got %q", result)
	}
}

func TestValidateBuild_NoPackageJson(t *testing.T) {
	dir := t.TempDir()
	ok, errMsg := validateBuildWith(dir, fakeSandboxRunner("", nil))
	if ok {
		t.Error("expected failure for missing package.json")
	}
	if errMsg != "No package.json found" {
		t.Errorf("unexpected error: %q", errMsg)
	}
}

func TestValidateBuild_RunsInstallAndBuild(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"scripts":{"build":"tsc"}}`), 0644); err != nil {
		t.Fatal(err)
	}

	var calls []struct {
		name string
		args []string
		opts exec.SandboxOpts
	}
	ok, errMsg := validateBuildWith(dir, func(_ context.Context, name string, args []string, opts exec.SandboxOpts) (string, error) {
		calls = append(calls, struct {
			name string
			args []string
			opts exec.SandboxOpts
		}{name, args, opts})
		return "", nil
	})
	if !ok || errMsg != "" {
		t.Fatalf("expected build validation to pass, got ok=%v err=%q", ok, errMsg)
	}
	if len(calls) != 2 {
		t.Fatalf("expected install and build calls, got %d", len(calls))
	}
	if calls[0].name != "npm" || !reflect.DeepEqual(calls[0].args, []string{"install", "--legacy-peer-deps"}) {
		t.Errorf("unexpected install call: %+v", calls[0])
	}
	if calls[1].name != "npm" || !reflect.DeepEqual(calls[1].args, []string{"run", "build"}) {
		t.Errorf("unexpected build call: %+v", calls[1])
	}
}

func TestValidateBuild_StopsWhenInstallFails(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{}`), 0644); err != nil {
		t.Fatal(err)
	}

	calls := 0
	ok, errMsg := validateBuildWith(dir, func(context.Context, string, []string, exec.SandboxOpts) (string, error) {
		calls++
		return "install failed", errors.New("exit status 1")
	})
	if ok || calls != 1 {
		t.Fatalf("expected install failure without build call, got ok=%v calls=%d", ok, calls)
	}
	if errMsg == "" {
		t.Fatal("expected install failure details")
	}
}
