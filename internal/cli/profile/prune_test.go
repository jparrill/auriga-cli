package profile

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/viper"
)

func TestReferencedFiles(t *testing.T) {
	tests := []struct {
		name     string
		profiles map[string]interface{}
		want     map[string]bool
	}{
		{
			name: "When profiles have model and mmproj, it should collect both",
			profiles: map[string]interface{}{
				"test-dense": map[string]interface{}{
					"model":  "Qwen3.6-27B-Q8_0.gguf",
					"mmproj": "mmproj-BF16.gguf",
				},
			},
			want: map[string]bool{
				"Qwen3.6-27B-Q8_0.gguf": true,
				"mmproj-BF16.gguf":      true,
			},
		},
		{
			name: "When profile has mtp_drafter, it should be referenced",
			profiles: map[string]interface{}{
				"test-mtp": map[string]interface{}{
					"model":       "gemma-4-12b-it-Q8_0.gguf",
					"mtp_drafter": "mtp-gemma-4-12b-it.gguf",
				},
			},
			want: map[string]bool{
				"gemma-4-12b-it-Q8_0.gguf": true,
				"mtp-gemma-4-12b-it.gguf":  true,
			},
		},
		{
			name: "When profile has dflash, it should be referenced",
			profiles: map[string]interface{}{
				"test-dflash": map[string]interface{}{
					"model":  "muse-glimmer-30B.gguf",
					"dflash": "dflash-kquant.gguf",
				},
			},
			want: map[string]bool{
				"muse-glimmer-30B.gguf": true,
				"dflash-kquant.gguf":    true,
			},
		},
		{
			name:     "When no profiles exist, it should return empty map",
			profiles: map[string]interface{}{},
			want:     map[string]bool{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			viper.Reset()
			viper.Set("profiles", tt.profiles)

			got := referencedFiles()
			if len(got) != len(tt.want) {
				t.Fatalf("got %d refs, want %d: %v", len(got), len(tt.want), got)
			}
			for k := range tt.want {
				if !got[k] {
					t.Errorf("missing reference: %s", k)
				}
			}
		})
	}
}

func TestFindOrphans(t *testing.T) {
	tests := []struct {
		name       string
		setup      func(dir string)
		referenced map[string]bool
		wantCount  int
	}{
		{
			name: "When 3 files exist and 2 are referenced, it should find 1 orphan",
			setup: func(dir string) {
				os.WriteFile(filepath.Join(dir, "model-a.gguf"), make([]byte, 1024), 0644)
				os.WriteFile(filepath.Join(dir, "model-b.gguf"), make([]byte, 2048), 0644)
				os.WriteFile(filepath.Join(dir, "model-c.gguf"), make([]byte, 512), 0644)
			},
			referenced: map[string]bool{
				"model-a.gguf": true,
				"model-b.gguf": true,
			},
			wantCount: 1,
		},
		{
			name: "When all files are referenced, it should find no orphans",
			setup: func(dir string) {
				os.WriteFile(filepath.Join(dir, "model.gguf"), make([]byte, 1024), 0644)
			},
			referenced: map[string]bool{
				"model.gguf": true,
			},
			wantCount: 0,
		},
		{
			name: "When no files are referenced, it should find all as orphans",
			setup: func(dir string) {
				os.WriteFile(filepath.Join(dir, "old-model.gguf"), make([]byte, 1024), 0644)
				os.WriteFile(filepath.Join(dir, "stale.gguf"), make([]byte, 2048), 0644)
			},
			referenced: map[string]bool{},
			wantCount:  2,
		},
		{
			name: "When dir has non-gguf files, it should skip them",
			setup: func(dir string) {
				os.WriteFile(filepath.Join(dir, "orphan.gguf"), make([]byte, 1024), 0644)
				os.WriteFile(filepath.Join(dir, "readme.txt"), make([]byte, 100), 0644)
				os.WriteFile(filepath.Join(dir, "imatrix.dat"), make([]byte, 200), 0644)
			},
			referenced: map[string]bool{},
			wantCount:  1,
		},
		{
			name: "When dir has subdirectories with .gguf name, it should skip them",
			setup: func(dir string) {
				os.Mkdir(filepath.Join(dir, "subdir.gguf"), 0755)
				os.WriteFile(filepath.Join(dir, "orphan.gguf"), make([]byte, 1024), 0644)
			},
			referenced: map[string]bool{},
			wantCount:  1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			tt.setup(dir)
			orphans, err := findOrphans(dir, "gguf", tt.referenced)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(orphans) != tt.wantCount {
				t.Fatalf("got %d orphans, want %d", len(orphans), tt.wantCount)
			}
		})
	}
}

func TestFindOrphans_NonexistentDir(t *testing.T) {
	_, err := findOrphans("/nonexistent/path", "gguf", map[string]bool{})
	if err == nil {
		t.Error("When dir does not exist, it should return an error")
	}
}

func TestFindOrphans_PreservesSize(t *testing.T) {
	dir := t.TempDir()
	data := make([]byte, 4096)
	os.WriteFile(filepath.Join(dir, "orphan.gguf"), data, 0644)

	orphans, _ := findOrphans(dir, "gguf", map[string]bool{})
	if len(orphans) != 1 {
		t.Fatalf("expected 1 orphan, got %d", len(orphans))
	}
	if orphans[0].Size != 4096 {
		t.Errorf("got size %d, want 4096", orphans[0].Size)
	}
	if orphans[0].Dir != "gguf" {
		t.Errorf("got dir %q, want %q", orphans[0].Dir, "gguf")
	}
}

func TestReferencedFiles_MultipleProfilesShareFile(t *testing.T) {
	viper.Reset()
	viper.Set("profiles", map[string]any{
		"profile-a": map[string]any{
			"model":  "model-a.gguf",
			"mmproj": "mmproj-BF16.gguf",
		},
		"profile-b": map[string]any{
			"model":  "model-b.gguf",
			"mmproj": "mmproj-BF16.gguf",
		},
	})

	got := referencedFiles()
	if len(got) != 3 {
		t.Fatalf("got %d refs, want 3 (shared mmproj deduped): %v", len(got), got)
	}
	for _, f := range []string{"model-a.gguf", "model-b.gguf", "mmproj-BF16.gguf"} {
		if !got[f] {
			t.Errorf("missing reference: %s", f)
		}
	}
}

func TestReferencedFiles_AllFieldsCombined(t *testing.T) {
	viper.Reset()
	viper.Set("profiles", map[string]any{
		"full": map[string]any{
			"model":       "model.gguf",
			"mmproj":      "mmproj.gguf",
			"mtp_drafter": "drafter.gguf",
			"dflash":      "dflash.gguf",
		},
	})

	got := referencedFiles()
	if len(got) != 4 {
		t.Fatalf("got %d refs, want 4: %v", len(got), got)
	}
}

func TestReferencedFiles_IgnoresNonFileFields(t *testing.T) {
	viper.Reset()
	viper.Set("profiles", map[string]any{
		"test": map[string]any{
			"model":            "model.gguf",
			"repo":             "unsloth/SomeModel-GGUF",
			"mtp_drafter_repo": "unsloth/SomeModel-MTP-GGUF",
			"type":             "dense",
			"flags":            []string{"--jinja"},
		},
	})

	got := referencedFiles()
	if len(got) != 1 {
		t.Fatalf("got %d refs, want 1 (only model): %v", len(got), got)
	}
	if !got["model.gguf"] {
		t.Error("missing model.gguf reference")
	}
}

func TestReferencedFiles_SkipsEmptyFields(t *testing.T) {
	viper.Reset()
	viper.Set("profiles", map[string]any{
		"minimal": map[string]any{
			"model":  "model.gguf",
			"mmproj": "",
		},
	})

	got := referencedFiles()
	if len(got) != 1 {
		t.Fatalf("got %d refs, want 1: %v", len(got), got)
	}
}

func TestFindOrphans_SetsFullPath(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "orphan.gguf"), make([]byte, 1024), 0644)

	orphans, _ := findOrphans(dir, "gguf", map[string]bool{})
	if len(orphans) != 1 {
		t.Fatalf("expected 1 orphan, got %d", len(orphans))
	}
	want := filepath.Join(dir, "orphan.gguf")
	if orphans[0].Path != want {
		t.Errorf("got path %q, want %q", orphans[0].Path, want)
	}
}

func TestFindOrphans_MmprojLabel(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "old-mmproj.gguf"), make([]byte, 512), 0644)

	orphans, _ := findOrphans(dir, "mmproj", map[string]bool{})
	if len(orphans) != 1 {
		t.Fatalf("expected 1 orphan, got %d", len(orphans))
	}
	if orphans[0].Dir != "mmproj" {
		t.Errorf("got dir %q, want %q", orphans[0].Dir, "mmproj")
	}
}

func TestNewProfilePruneCmd(t *testing.T) {
	cmd := NewProfileCmd()
	found := false
	for _, sub := range cmd.Commands() {
		if sub.Name() == "prune" {
			found = true
			f := sub.Flags().Lookup("dry-run")
			if f == nil {
				t.Error("When creating prune command, it should have --dry-run flag")
			}
			if f != nil && f.DefValue != "false" {
				t.Errorf("--dry-run default should be false, got %q", f.DefValue)
			}
			break
		}
	}
	if !found {
		t.Error("When building profile command, it should register prune subcommand")
	}
}
