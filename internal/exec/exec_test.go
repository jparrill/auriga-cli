package exec

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/jparrill/auriga-cli/internal/config"
	"github.com/jparrill/auriga-cli/internal/ui"
)

func TestMain(m *testing.M) {
	ui.InitLogger(false)
	os.Exit(m.Run())
}

func TestRunCapture_Success(t *testing.T) {
	out, err := RunCapture(context.Background(), "echo", []string{"hello"}, RunOpts{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "hello") {
		t.Errorf("expected 'hello' in output, got %q", out)
	}
}

func TestRunCapture_Failure(t *testing.T) {
	_, err := RunCapture(context.Background(), "false", nil, RunOpts{})
	if err == nil {
		t.Error("expected error from 'false' command")
	}
}

func TestRunCapture_DryRun(t *testing.T) {
	out, err := RunCapture(context.Background(), "echo", []string{"test"}, RunOpts{DryRun: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out != "" {
		t.Errorf("expected empty output in dry-run, got %q", out)
	}
}

func TestRunCapture_GlobalDryRun(t *testing.T) {
	config.DryRun = true
	defer func() { config.DryRun = false }()

	out, err := RunCapture(context.Background(), "echo", []string{"test"}, RunOpts{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out != "" {
		t.Errorf("expected empty output in dry-run, got %q", out)
	}
}

func TestRunCapture_WorkingDir(t *testing.T) {
	out, err := RunCapture(context.Background(), "pwd", nil, RunOpts{Dir: "/tmp"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "tmp") {
		t.Errorf("expected /tmp in output, got %q", out)
	}
}

func TestRunStreaming_Success(t *testing.T) {
	err := RunStreaming(context.Background(), "true", nil, RunOpts{})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRunStreaming_Failure(t *testing.T) {
	err := RunStreaming(context.Background(), "false", nil, RunOpts{})
	if err == nil {
		t.Error("expected error from 'false' command")
	}
}

func TestRunStreaming_DryRun(t *testing.T) {
	err := RunStreaming(context.Background(), "false", nil, RunOpts{DryRun: true})
	if err != nil {
		t.Errorf("expected nil in dry-run, got %v", err)
	}
}

func TestBuildEnv(t *testing.T) {
	env := buildEnv(map[string]string{"FOO": "bar"})
	found := false
	for _, e := range env {
		if e == "FOO=bar" {
			found = true
		}
	}
	if !found {
		t.Error("expected FOO=bar in env")
	}
}

func TestBuildEnv_Empty(t *testing.T) {
	env := buildEnv(nil)
	if env != nil {
		t.Errorf("expected nil for empty env, got %v", env)
	}
}

func TestRunSandboxed_DryRun(t *testing.T) {
	config.DryRun = true
	defer func() { config.DryRun = false }()

	out, err := RunSandboxed(context.Background(), "npm", []string{"install"}, SandboxOpts{
		Dir:   "/tmp/test",
		Image: ImageNode,
	})
	if err != nil {
		t.Fatalf("When dry-run is enabled, RunSandboxed should not error: %v", err)
	}
	if out != "" {
		t.Errorf("When dry-run is enabled, RunSandboxed should return empty output, got %q", out)
	}
}

func TestDockerAvailable(t *testing.T) {
	result := dockerAvailable()
	// Just verify it returns a boolean without panicking — actual availability depends on environment
	_ = result
}

func TestSandboxOpts_Fields(t *testing.T) {
	opts := SandboxOpts{Dir: "/work", Image: ImageGo}
	if opts.Dir != "/work" {
		t.Errorf("When setting Dir, it should be '/work', got %q", opts.Dir)
	}
	if opts.Image != "golang:1.22" {
		t.Errorf("When using ImageGo, it should be 'golang:1.22', got %q", opts.Image)
	}
}

func TestImageConstants(t *testing.T) {
	if ImageNode == "" || ImageGo == "" || ImagePython == "" {
		t.Error("When checking image constants, none should be empty")
	}
}
