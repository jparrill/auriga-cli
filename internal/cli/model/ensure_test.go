package model

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/viper"
)

func TestNewModelEnsureCmd(t *testing.T) {
	tests := []struct {
		name  string
		check func(t *testing.T)
	}{
		{
			name: "When building model command, it should register the ensure subcommand",
			check: func(t *testing.T) {
				cmd := NewModelCmd()
				for _, sub := range cmd.Commands() {
					if sub.Name() == "ensure" {
						return
					}
				}
				t.Error("ensure subcommand not found")
			},
		},
		{
			name: "When creating ensure command, it should have backend flag defaulting to all",
			check: func(t *testing.T) {
				cmd := newModelEnsureCmd()
				f := cmd.Flags().Lookup("backend")
				if f == nil {
					t.Fatal("--backend flag not found")
				}
				if f.DefValue != "all" {
					t.Errorf("got default %q, want %q", f.DefValue, "all")
				}
			},
		},
		{
			name: "When creating ensure command, it should have profile flag defaulting to empty",
			check: func(t *testing.T) {
				cmd := newModelEnsureCmd()
				f := cmd.Flags().Lookup("profile")
				if f == nil {
					t.Fatal("--profile flag not found")
				}
				if f.DefValue != "" {
					t.Errorf("got default %q, want empty", f.DefValue)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, tt.check)
	}
}

func TestRunModelEnsure_ProfileNotFound(t *testing.T) {
	viper.Reset()
	err := runModelEnsure("all", "nonexistent")
	if err == nil {
		t.Error("When ensuring a nonexistent profile, it should return an error")
	}
	if err != nil && err.Error() != `profile "nonexistent" not found` {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRunModelEnsure_ProfileFlag_SkipsBackends(t *testing.T) {
	ggufDir := t.TempDir()
	modelFile := "test-model.gguf"
	os.WriteFile(filepath.Join(ggufDir, modelFile), []byte("model"), 0644)

	viper.Reset()
	viper.Set("profiles.test.model", modelFile)
	viper.Set("profiles.test.repo", "unsloth/test-GGUF")
	viper.Set("llama_server.gguf_dir", ggufDir)
	viper.Set("llama_server.mmproj_dir", t.TempDir())
	viper.Set("ollama.models", []string{"should-not-pull"})

	err := runModelEnsure("all", "test")
	if err != nil {
		t.Errorf("When profile flag set with existing files, should not error, got: %v", err)
	}
}

func TestRunModelEnsure_NoProfileFlag_ProcessesProfiles(t *testing.T) {
	ggufDir := t.TempDir()
	modelFile := "present-model.gguf"
	os.WriteFile(filepath.Join(ggufDir, modelFile), []byte("model"), 0644)

	viper.Reset()
	viper.Set("profiles.present.model", modelFile)
	viper.Set("profiles.missing-no-repo.model", "missing.gguf")
	viper.Set("llama_server.gguf_dir", ggufDir)
	viper.Set("llama_server.mmproj_dir", t.TempDir())

	err := runModelEnsure("llama-server", "")
	if err != nil {
		t.Errorf("When running ensure without profile flag, should not error, got: %v", err)
	}
}

func TestRunModelEnsure_OllamaBackend_SkipsProfiles(t *testing.T) {
	viper.Reset()
	viper.Set("profiles.test.model", "test.gguf")
	viper.Set("llama_server.gguf_dir", t.TempDir())
	viper.Set("llama_server.mmproj_dir", t.TempDir())

	err := runModelEnsure("ollama", "")
	if err != nil {
		t.Errorf("When backend is ollama, should not error, got: %v", err)
	}
}
