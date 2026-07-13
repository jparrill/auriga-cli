package model

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/spf13/viper"
)

func TestCollectGGUFCandidates_FindsGGUFFiles(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "Qwen3-32B-Q4_K_M.gguf"), make([]byte, 1024*1024), 0644)
	os.WriteFile(filepath.Join(dir, "gemma-12b-Q4_K_M.gguf"), make([]byte, 512*1024), 0644)
	os.WriteFile(filepath.Join(dir, "README.md"), []byte("not a model"), 0644)

	candidates := collectGGUFCandidates(dir, "gguf")
	if len(candidates) != 2 {
		t.Errorf("When dir has 2 GGUF files and 1 non-GGUF, it should find 2 candidates, got %d", len(candidates))
	}
	for _, c := range candidates {
		if c.Backend != "gguf" {
			t.Errorf("When collecting GGUF candidates, backend should be 'gguf', got %q", c.Backend)
		}
		if c.Path == "" {
			t.Error("When collecting GGUF candidates, it should set the filesystem path")
		}
	}
}

func TestCollectGGUFCandidates_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	candidates := collectGGUFCandidates(dir, "gguf")
	if len(candidates) != 0 {
		t.Errorf("When dir is empty, it should return no candidates, got %d", len(candidates))
	}
}

func TestCollectGGUFCandidates_NonexistentDir(t *testing.T) {
	candidates := collectGGUFCandidates("/nonexistent/path", "gguf")
	if candidates != nil {
		t.Errorf("When dir does not exist, it should return nil, got %v", candidates)
	}
}

func TestCollectGGUFCandidates_MmprojBackend(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "mmproj-model-f16.gguf"), make([]byte, 256*1024), 0644)
	os.WriteFile(filepath.Join(dir, "projector.bin"), make([]byte, 128*1024), 0644)

	candidates := collectGGUFCandidates(dir, "mmproj")
	if len(candidates) != 2 {
		t.Errorf("When backend is mmproj, it should include all files, got %d", len(candidates))
	}
	for _, c := range candidates {
		if c.Backend != "mmproj" {
			t.Errorf("When collecting mmproj candidates, backend should be 'mmproj', got %q", c.Backend)
		}
	}
}

func TestFormatSize_GigabyteRange(t *testing.T) {
	result := formatSize(21_474_836_480) // 20 GB
	if result != "20.0 GB" {
		t.Errorf("When size is 20 GB, it should format as '20.0 GB', got %q", result)
	}
}

func TestFormatSize_MegabyteRange(t *testing.T) {
	result := formatSize(524_288_000) // 500 MB
	if result != "500 MB" {
		t.Errorf("When size is 500 MB, it should format as '500 MB', got %q", result)
	}
}

func TestNewModelPruneCmd_Registered(t *testing.T) {
	cmd := NewModelCmd()
	found := false
	for _, sub := range cmd.Commands() {
		if sub.Name() == "prune" {
			found = true
			break
		}
	}
	if !found {
		t.Error("When building model command, it should register the prune subcommand")
	}
}

func TestNewModelPruneCmd_BackendFlag(t *testing.T) {
	cmd := newModelPruneCmd()
	f := cmd.Flags().Lookup("backend")
	if f == nil {
		t.Fatal("When creating prune command, it should have a --backend flag")
	}
	if f.DefValue != "all" {
		t.Errorf("When creating prune command, --backend default should be 'all', got %q", f.DefValue)
	}
}

func TestCollectGGUFCandidates_SkipsDirectories(t *testing.T) {
	dir := t.TempDir()
	os.Mkdir(filepath.Join(dir, "subdir.gguf"), 0755)
	os.WriteFile(filepath.Join(dir, "model.gguf"), make([]byte, 1024), 0644)

	candidates := collectGGUFCandidates(dir, "gguf")
	if len(candidates) != 1 {
		t.Errorf("When dir has a directory with .gguf extension, it should skip it, got %d candidates", len(candidates))
	}
}

func TestCollectGGUFCandidates_SetsModTime(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "model.gguf")
	os.WriteFile(path, make([]byte, 1024), 0644)

	_ = viper.GetString("llama_server.gguf_dir")
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
