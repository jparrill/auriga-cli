package exec

import (
	"context"
	"strings"
	"testing"

	"github.com/jparrill/auriga-cli/internal/config"
	"github.com/jparrill/auriga-cli/internal/ui"
)

func init() {
	ui.InitLogger(false)
}

func TestRunCapture(t *testing.T) {
	tests := []struct {
		name      string
		cmd       string
		args      []string
		opts      RunOpts
		wantErr   bool
		wantEmpty bool
		wantHas   string
	}{
		{
			name:    "When command succeeds, it should capture output",
			cmd:     "echo",
			args:    []string{"hello"},
			wantHas: "hello",
		},
		{
			name:    "When command fails, it should return an error",
			cmd:     "false",
			wantErr: true,
		},
		{
			name:      "When dry-run is set via opts, it should return empty output",
			cmd:       "echo",
			args:      []string{"test"},
			opts:      RunOpts{DryRun: true},
			wantEmpty: true,
		},
		{
			name:    "When working dir is set, it should run in that directory",
			cmd:     "pwd",
			opts:    RunOpts{Dir: "/tmp"},
			wantHas: "tmp",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, err := RunCapture(context.Background(), tt.cmd, tt.args, tt.opts)
			if tt.wantErr && err == nil {
				t.Error("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tt.wantEmpty && out != "" {
				t.Errorf("expected empty output, got %q", out)
			}
			if tt.wantHas != "" && !strings.Contains(out, tt.wantHas) {
				t.Errorf("expected %q in output, got %q", tt.wantHas, out)
			}
		})
	}
}

func TestRunCapture_GlobalDryRun(t *testing.T) {
	config.DryRun = true
	defer func() { config.DryRun = false }()

	out, err := RunCapture(context.Background(), "echo", []string{"test"}, RunOpts{})
	if err != nil {
		t.Fatalf("When global dry-run is enabled, it should not error: %v", err)
	}
	if out != "" {
		t.Errorf("When global dry-run is enabled, it should return empty output, got %q", out)
	}
}

func TestRunStreaming(t *testing.T) {
	tests := []struct {
		name    string
		cmd     string
		opts    RunOpts
		wantErr bool
	}{
		{"When command succeeds, it should return nil", "true", RunOpts{}, false},
		{"When command fails, it should return an error", "false", RunOpts{}, true},
		{"When dry-run is set, it should return nil even for failing command", "false", RunOpts{DryRun: true}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := RunStreaming(context.Background(), tt.cmd, nil, tt.opts)
			if (err != nil) != tt.wantErr {
				t.Errorf("got err=%v, wantErr=%v", err, tt.wantErr)
			}
		})
	}
}

func TestBuildEnv(t *testing.T) {
	tests := []struct {
		name    string
		extra   map[string]string
		wantNil bool
		wantHas string
	}{
		{
			name:    "When extra env is nil, it should return nil",
			extra:   nil,
			wantNil: true,
		},
		{
			name:    "When extra env has FOO=bar, it should include FOO=bar",
			extra:   map[string]string{"FOO": "bar"},
			wantHas: "FOO=bar",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := buildEnv(tt.extra)
			if tt.wantNil && env != nil {
				t.Errorf("expected nil, got %v", env)
			}
			if tt.wantHas != "" {
				found := false
				for _, e := range env {
					if e == tt.wantHas {
						found = true
					}
				}
				if !found {
					t.Errorf("expected %q in env", tt.wantHas)
				}
			}
		})
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

func TestImageConstants(t *testing.T) {
	tests := []struct {
		name  string
		image string
	}{
		{"When checking ImageNode, it should not be empty", ImageNode},
		{"When checking ImageGo, it should not be empty", ImageGo},
		{"When checking ImagePython, it should not be empty", ImagePython},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.image == "" {
				t.Error("image constant is empty")
			}
		})
	}
}

func TestSandboxOpts(t *testing.T) {
	tests := []struct {
		name      string
		opts      SandboxOpts
		wantDir   string
		wantImage string
	}{
		{
			name:      "When using Go image, it should set golang:1.22",
			opts:      SandboxOpts{Dir: "/work", Image: ImageGo},
			wantDir:   "/work",
			wantImage: "golang:1.22",
		},
		{
			name:      "When using Node image, it should set node:20-slim",
			opts:      SandboxOpts{Dir: "/app", Image: ImageNode},
			wantDir:   "/app",
			wantImage: "node:20-slim",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.opts.Dir != tt.wantDir {
				t.Errorf("got Dir %q, want %q", tt.opts.Dir, tt.wantDir)
			}
			if tt.opts.Image != tt.wantImage {
				t.Errorf("got Image %q, want %q", tt.opts.Image, tt.wantImage)
			}
		})
	}
}
