package model

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCollectGGUFCandidates(t *testing.T) {
	tests := []struct {
		name      string
		setup     func(dir string)
		backend   string
		wantCount int
		wantBack  string
	}{
		{
			name: "When dir has 2 GGUF files and 1 non-GGUF, it should find 2 candidates",
			setup: func(dir string) {
				os.WriteFile(filepath.Join(dir, "Qwen3-32B-Q4_K_M.gguf"), make([]byte, 1024*1024), 0644)
				os.WriteFile(filepath.Join(dir, "gemma-12b-Q4_K_M.gguf"), make([]byte, 512*1024), 0644)
				os.WriteFile(filepath.Join(dir, "README.md"), []byte("not a model"), 0644)
			},
			backend:   "gguf",
			wantCount: 2,
			wantBack:  "gguf",
		},
		{
			name:      "When dir is empty, it should return no candidates",
			setup:     func(dir string) {},
			backend:   "gguf",
			wantCount: 0,
		},
		{
			name: "When backend is mmproj, it should include all files",
			setup: func(dir string) {
				os.WriteFile(filepath.Join(dir, "mmproj-model-f16.gguf"), make([]byte, 256*1024), 0644)
				os.WriteFile(filepath.Join(dir, "projector.bin"), make([]byte, 128*1024), 0644)
			},
			backend:   "mmproj",
			wantCount: 2,
			wantBack:  "mmproj",
		},
		{
			name: "When dir has a directory with .gguf extension, it should skip it",
			setup: func(dir string) {
				os.Mkdir(filepath.Join(dir, "subdir.gguf"), 0755)
				os.WriteFile(filepath.Join(dir, "model.gguf"), make([]byte, 1024), 0644)
			},
			backend:   "gguf",
			wantCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			tt.setup(dir)
			candidates := collectGGUFCandidates(dir, tt.backend)
			if len(candidates) != tt.wantCount {
				t.Fatalf("got %d candidates, want %d", len(candidates), tt.wantCount)
			}
			if tt.wantBack != "" {
				for _, c := range candidates {
					if c.Backend != tt.wantBack {
						t.Errorf("got backend %q, want %q", c.Backend, tt.wantBack)
					}
				}
			}
		})
	}
}

func TestCollectGGUFCandidates_NonexistentDir(t *testing.T) {
	candidates := collectGGUFCandidates("/nonexistent/path", "gguf")
	if candidates != nil {
		t.Errorf("When dir does not exist, it should return nil, got %v", candidates)
	}
}

func TestCollectGGUFCandidates_SetsModTime(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "model.gguf"), make([]byte, 1024), 0644)

	candidates := collectGGUFCandidates(dir, "gguf")
	if len(candidates) != 1 {
		t.Fatalf("expected 1 candidate, got %d", len(candidates))
	}
	if candidates[0].ModTime.IsZero() {
		t.Error("When collecting candidates, it should set a non-zero ModTime")
	}
	if time.Since(candidates[0].ModTime) > 5*time.Second {
		t.Error("When file was just created, ModTime should be recent")
	}
}

func TestFormatSize(t *testing.T) {
	tests := []struct {
		name  string
		bytes int64
		want  string
	}{
		{"When size is 20 GB, it should format as 20.0 GB", 21_474_836_480, "20.0 GB"},
		{"When size is 500 MB, it should format as 500 MB", 524_288_000, "500 MB"},
		{"When size is 1.5 GB, it should format as 1.5 GB", 1_610_612_736, "1.5 GB"},
		{"When size is 100 MB, it should format as 100 MB", 104_857_600, "100 MB"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatSize(tt.bytes)
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestNewModelPruneCmd(t *testing.T) {
	tests := []struct {
		name string
		check func(t *testing.T)
	}{
		{
			name: "When building model command, it should register the prune subcommand",
			check: func(t *testing.T) {
				cmd := NewModelCmd()
				for _, sub := range cmd.Commands() {
					if sub.Name() == "prune" {
						return
					}
				}
				t.Error("prune subcommand not found")
			},
		},
		{
			name: "When creating prune command, backend default should be all",
			check: func(t *testing.T) {
				cmd := newModelPruneCmd()
				f := cmd.Flags().Lookup("backend")
				if f == nil {
					t.Fatal("--backend flag not found")
				}
				if f.DefValue != "all" {
					t.Errorf("got default %q, want %q", f.DefValue, "all")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, tt.check)
	}
}
