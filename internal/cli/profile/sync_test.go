package profile

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/viper"
)

func TestNewProfileSyncCmd(t *testing.T) {
	tests := []struct {
		name  string
		check func(t *testing.T)
	}{
		{
			name: "When building profile command, it should register the sync subcommand",
			check: func(t *testing.T) {
				cmd := NewProfileCmd()
				for _, sub := range cmd.Commands() {
					if sub.Name() == "sync" {
						return
					}
				}
				t.Error("sync subcommand not found")
			},
		},
		{
			name: "When creating sync command, it should have name flag defaulting to empty",
			check: func(t *testing.T) {
				cmd := newProfileSyncCmd()
				f := cmd.Flags().Lookup("name")
				if f == nil {
					t.Fatal("--name flag not found")
				}
				if f.DefValue != "" {
					t.Errorf("got default %q, want empty", f.DefValue)
				}
			},
		},
		{
			name: "When creating sync command, it should accept zero arguments",
			check: func(t *testing.T) {
				cmd := newProfileSyncCmd()
				if cmd.Args != nil {
					if err := cmd.Args(cmd, []string{}); err != nil {
						t.Errorf("expected no error with 0 args, got %v", err)
					}
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, tt.check)
	}
}

func TestRunProfileSync_ProfileNotFound(t *testing.T) {
	viper.Reset()
	err := runProfileSync("nonexistent")
	if err == nil {
		t.Error("When syncing a nonexistent profile, it should return an error")
	}
	if err != nil && err.Error() != `profile "nonexistent" not found` {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestSyncProfile_AllFilesPresent(t *testing.T) {
	ggufDir := t.TempDir()
	mmprojDir := t.TempDir()

	modelFile := "test-model-Q4_K_M.gguf"
	mmprojFile := "test-mmproj-BF16.gguf"
	os.WriteFile(filepath.Join(ggufDir, modelFile), []byte("model"), 0644)
	profileMmprojDir := filepath.Join(mmprojDir, "test-profile")
	os.MkdirAll(profileMmprojDir, 0755)
	os.WriteFile(filepath.Join(profileMmprojDir, mmprojFile), []byte("mmproj"), 0644)

	viper.Reset()
	viper.Set("profiles.test-profile.repo", "unsloth/test-GGUF")
	viper.Set("profiles.test-profile.model", modelFile)
	viper.Set("profiles.test-profile.mmproj", mmprojFile)
	viper.Set("llama_server.gguf_dir", ggufDir)
	viper.Set("llama_server.mmproj_dir", mmprojDir)

	result := SyncProfile("test-profile")

	if result.Status != "skip" {
		t.Errorf("When all files present, status should be 'skip', got %q", result.Status)
	}
	if result.Name != "test-profile" {
		t.Errorf("When syncing, result Name should match profile name, got %q", result.Name)
	}
}

func TestSyncProfile_ModelOnly_Present(t *testing.T) {
	ggufDir := t.TempDir()

	modelFile := "test-model-Q4_K_M.gguf"
	os.WriteFile(filepath.Join(ggufDir, modelFile), []byte("model"), 0644)

	viper.Reset()
	viper.Set("profiles.test-profile.repo", "unsloth/test-GGUF")
	viper.Set("profiles.test-profile.model", modelFile)
	viper.Set("llama_server.gguf_dir", ggufDir)
	viper.Set("llama_server.mmproj_dir", t.TempDir())

	result := SyncProfile("test-profile")

	if result.Status != "skip" {
		t.Errorf("When model-only profile has file present, status should be 'skip', got %q", result.Status)
	}
}

func TestSyncProfile_NoRepo_MissingFiles(t *testing.T) {
	viper.Reset()
	viper.Set("profiles.manual.model", "manual-model.gguf")
	viper.Set("profiles.manual.mmproj", "manual-mmproj.gguf")
	viper.Set("llama_server.gguf_dir", t.TempDir())
	viper.Set("llama_server.mmproj_dir", t.TempDir())

	result := SyncProfile("manual")

	if result.Status != "warn" {
		t.Errorf("When profile has no repo and files missing, status should be 'warn', got %q", result.Status)
	}
	if result.Detail == "" {
		t.Error("When profile has no repo, detail should describe missing files")
	}
}

func TestSyncProfile_NoRepo_ModelMissing(t *testing.T) {
	viper.Reset()
	viper.Set("profiles.no-repo.model", "missing.gguf")
	viper.Set("llama_server.gguf_dir", t.TempDir())
	viper.Set("llama_server.mmproj_dir", t.TempDir())

	result := SyncProfile("no-repo")

	if result.Status != "warn" {
		t.Errorf("When profile has no repo and model missing, status should be 'warn', got %q", result.Status)
	}
}

func TestSyncProfile_NoModel(t *testing.T) {
	viper.Reset()
	viper.Set("profiles.empty.repo", "unsloth/test-GGUF")
	viper.Set("llama_server.gguf_dir", t.TempDir())
	viper.Set("llama_server.mmproj_dir", t.TempDir())

	result := SyncProfile("empty")

	if result.Status != "fail" {
		t.Errorf("When profile has no model configured, status should be 'fail', got %q", result.Status)
	}
}

func TestSyncProfile_MmprojMissing_ModelPresent_NoRepo(t *testing.T) {
	ggufDir := t.TempDir()
	modelFile := "test-model.gguf"
	os.WriteFile(filepath.Join(ggufDir, modelFile), []byte("model"), 0644)

	viper.Reset()
	viper.Set("profiles.partial.model", modelFile)
	viper.Set("profiles.partial.mmproj", "missing-mmproj.gguf")
	viper.Set("llama_server.gguf_dir", ggufDir)
	viper.Set("llama_server.mmproj_dir", t.TempDir())

	result := SyncProfile("partial")

	if result.Status != "warn" {
		t.Errorf("When model exists but mmproj missing and no repo, status should be 'warn', got %q", result.Status)
	}
}

func TestSyncAllProfiles_Empty(t *testing.T) {
	viper.Reset()

	results := SyncAllProfiles()

	if results != nil {
		t.Errorf("When no profiles configured, should return nil, got %v", results)
	}
}

func TestSyncAllProfiles_Multiple(t *testing.T) {
	ggufDir := t.TempDir()
	mmprojDir := t.TempDir()

	os.WriteFile(filepath.Join(ggufDir, "present.gguf"), []byte("ok"), 0644)

	viper.Reset()
	viper.Set("profiles.present.model", "present.gguf")
	viper.Set("profiles.missing-no-repo.model", "missing.gguf")
	viper.Set("llama_server.gguf_dir", ggufDir)
	viper.Set("llama_server.mmproj_dir", mmprojDir)

	results := SyncAllProfiles()

	if len(results) != 2 {
		t.Fatalf("When 2 profiles configured, should return 2 results, got %d", len(results))
	}

	statusMap := map[string]string{}
	for _, r := range results {
		statusMap[r.Name] = r.Status
	}

	if statusMap["present"] != "skip" {
		t.Errorf("When profile files present, should be 'skip', got %q", statusMap["present"])
	}
	if statusMap["missing-no-repo"] != "warn" {
		t.Errorf("When profile missing files and no repo, should be 'warn', got %q", statusMap["missing-no-repo"])
	}
}

func TestPrintSyncSummary_NoPanic(t *testing.T) {
	tests := []struct {
		name    string
		results []SyncResult
	}{
		{
			name:    "When results are nil, it should not panic",
			results: nil,
		},
		{
			name:    "When results are empty, it should not panic",
			results: []SyncResult{},
		},
		{
			name: "When results have mixed statuses, it should not panic",
			results: []SyncResult{
				{Name: "a", Status: "ok"},
				{Name: "b", Status: "skip"},
				{Name: "c", Status: "warn"},
				{Name: "d", Status: "fail"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			PrintSyncSummary(tt.results)
		})
	}
}

func TestRepoFilename(t *testing.T) {
	tests := []struct {
		name        string
		profileName string
		repo        string
		want        string
	}{
		{
			name:        "When mmproj was renamed with repo prefix, it should strip the prefix",
			profileName: "Qwen3.6-35B-A3B-mmproj-BF16.gguf",
			repo:        "unsloth/Qwen3.6-35B-A3B-GGUF",
			want:        "mmproj-BF16.gguf",
		},
		{
			name:        "When mmproj has no repo prefix, it should return as-is",
			profileName: "mmproj-BF16.gguf",
			repo:        "unsloth/Qwen3.6-35B-A3B-GGUF",
			want:        "mmproj-BF16.gguf",
		},
		{
			name:        "When repo has no -GGUF suffix, it should use full repo base",
			profileName: "SomeModel-mmproj.gguf",
			repo:        "org/SomeModel",
			want:        "mmproj.gguf",
		},
		{
			name:        "When profile name does not match repo at all, it should return as-is",
			profileName: "totally-different.gguf",
			repo:        "unsloth/Qwen3-GGUF",
			want:        "totally-different.gguf",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := repoFilename(tt.profileName, tt.repo)
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFileExists(t *testing.T) {
	tests := []struct {
		name string
		want bool
		path string
	}{
		{
			name: "When file exists, it should return true",
			want: true,
		},
		{
			name: "When file does not exist, it should return false",
			want: false,
			path: "/nonexistent/path/to/file.gguf",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := tt.path
			if tt.want {
				f, _ := os.CreateTemp("", "test-*.gguf")
				path = f.Name()
				defer os.Remove(path)
			}
			got := fileExists(path)
			if got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSyncProfile_AllFilesWithDrafters(t *testing.T) {
	ggufDir := t.TempDir()
	mmprojDir := t.TempDir()

	os.WriteFile(filepath.Join(ggufDir, "model.gguf"), []byte("model"), 0644)
	profileMmprojDir := filepath.Join(mmprojDir, "full")
	os.MkdirAll(profileMmprojDir, 0755)
	os.WriteFile(filepath.Join(profileMmprojDir, "mmproj.gguf"), []byte("mmproj"), 0644)
	os.WriteFile(filepath.Join(ggufDir, "drafter.gguf"), []byte("drafter"), 0644)

	viper.Reset()
	defer viper.Reset()
	viper.Set("profiles.full.repo", "org/repo")
	viper.Set("profiles.full.model", "model.gguf")
	viper.Set("profiles.full.mmproj", "mmproj.gguf")
	viper.Set("profiles.full.dflash", "drafter.gguf")
	viper.Set("llama_server.gguf_dir", ggufDir)
	viper.Set("llama_server.mmproj_dir", mmprojDir)

	result := SyncProfile("full")

	if result.Status != "skip" {
		t.Errorf("When all files including drafter present, status should be 'skip', got %q", result.Status)
	}
}

func TestSyncProfile_DFlashMissing_NoRepo(t *testing.T) {
	ggufDir := t.TempDir()
	os.WriteFile(filepath.Join(ggufDir, "model.gguf"), []byte("model"), 0644)

	viper.Reset()
	defer viper.Reset()
	viper.Set("profiles.nodrafter.model", "model.gguf")
	viper.Set("profiles.nodrafter.dflash", "missing-drafter.gguf")
	viper.Set("llama_server.gguf_dir", ggufDir)
	viper.Set("llama_server.mmproj_dir", t.TempDir())

	result := SyncProfile("nodrafter")

	if result.Status != "warn" {
		t.Errorf("When dflash missing and no repo, status should be 'warn', got %q", result.Status)
	}
}

func TestSyncProfile_MTPDrafterMissing_NoRepo(t *testing.T) {
	ggufDir := t.TempDir()
	os.WriteFile(filepath.Join(ggufDir, "model.gguf"), []byte("model"), 0644)

	viper.Reset()
	defer viper.Reset()
	viper.Set("profiles.nodrafter.model", "model.gguf")
	viper.Set("profiles.nodrafter.mtp_drafter", "missing-mtp.gguf")
	viper.Set("llama_server.gguf_dir", ggufDir)
	viper.Set("llama_server.mmproj_dir", t.TempDir())

	result := SyncProfile("nodrafter")

	if result.Status != "warn" {
		t.Errorf("When mtp_drafter missing and no repo, status should be 'warn', got %q", result.Status)
	}
}

func TestSyncResult_Fields(t *testing.T) {
	r := SyncResult{Name: "test", Status: "ok", Detail: "downloaded"}
	if r.Name != "test" {
		t.Errorf("expected Name 'test', got %q", r.Name)
	}
	if r.Status != "ok" {
		t.Errorf("expected Status 'ok', got %q", r.Status)
	}
	if r.Detail != "downloaded" {
		t.Errorf("expected Detail 'downloaded', got %q", r.Detail)
	}
}

func TestMmprojPath(t *testing.T) {
	viper.Reset()
	defer viper.Reset()
	viper.Set("llama_server.mmproj_dir", "/data/mmproj")

	tests := []struct {
		name    string
		profile string
		mmproj  string
		want    string
	}{
		{
			name:    "When profile and mmproj given, path includes profile subdirectory",
			profile: "gemma4-31b-vision",
			mmproj:  "mmproj-BF16.gguf",
			want:    "/data/mmproj/gemma4-31b-vision/mmproj-BF16.gguf",
		},
		{
			name:    "When two profiles use same mmproj filename, paths differ",
			profile: "gemma4-12b-vision",
			mmproj:  "mmproj-BF16.gguf",
			want:    "/data/mmproj/gemma4-12b-vision/mmproj-BF16.gguf",
		},
		{
			name:    "When mmproj has model-specific name, still uses profile subdir",
			profile: "qwen3.6-vision",
			mmproj:  "Qwen3.6-35B-A3B-MTP-mmproj-BF16.gguf",
			want:    "/data/mmproj/qwen3.6-vision/Qwen3.6-35B-A3B-MTP-mmproj-BF16.gguf",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MmprojPath(tt.profile, tt.mmproj)
			if got != tt.want {
				t.Errorf("MmprojPath(%q, %q) = %q, want %q", tt.profile, tt.mmproj, got, tt.want)
			}
		})
	}
}

func TestMmprojPath_NoCollision(t *testing.T) {
	viper.Reset()
	defer viper.Reset()
	viper.Set("llama_server.mmproj_dir", "/data/mmproj")

	path31b := MmprojPath("gemma4-31b-vision", "mmproj-BF16.gguf")
	path12b := MmprojPath("gemma4-12b-vision", "mmproj-BF16.gguf")
	path26b := MmprojPath("gemma4-26b", "mmproj-BF16.gguf")

	if path31b == path12b {
		t.Errorf("31B and 12B should have different paths, both got %q", path31b)
	}
	if path31b == path26b {
		t.Errorf("31B and 26B should have different paths, both got %q", path31b)
	}
	if path12b == path26b {
		t.Errorf("12B and 26B should have different paths, both got %q", path12b)
	}
}

func TestSyncProfile_MmprojSubdirectory(t *testing.T) {
	t.Run("When mmproj in profile subdir, sync finds it", func(t *testing.T) {
		ggufDir := t.TempDir()
		mmprojDir := t.TempDir()

		os.WriteFile(filepath.Join(ggufDir, "model.gguf"), []byte("model"), 0644)
		profileDir := filepath.Join(mmprojDir, "subdir-test")
		os.MkdirAll(profileDir, 0755)
		os.WriteFile(filepath.Join(profileDir, "mmproj-BF16.gguf"), []byte("mmproj"), 0644)

		viper.Reset()
		defer viper.Reset()
		viper.Set("profiles.subdir-test.repo", "org/repo")
		viper.Set("profiles.subdir-test.model", "model.gguf")
		viper.Set("profiles.subdir-test.mmproj", "mmproj-BF16.gguf")
		viper.Set("llama_server.gguf_dir", ggufDir)
		viper.Set("llama_server.mmproj_dir", mmprojDir)

		result := SyncProfile("subdir-test")

		if result.Status != "skip" {
			t.Errorf("When mmproj in profile subdir, status should be 'skip', got %q (detail: %s)", result.Status, result.Detail)
		}
	})

	t.Run("When mmproj not in profile subdir and no repo, sync warns", func(t *testing.T) {
		ggufDir := t.TempDir()
		mmprojDir := t.TempDir()

		os.WriteFile(filepath.Join(ggufDir, "model.gguf"), []byte("model"), 0644)

		viper.Reset()
		defer viper.Reset()
		viper.Set("profiles.no-mmproj.model", "model.gguf")
		viper.Set("profiles.no-mmproj.mmproj", "mmproj-BF16.gguf")
		viper.Set("llama_server.gguf_dir", ggufDir)
		viper.Set("llama_server.mmproj_dir", mmprojDir)

		result := SyncProfile("no-mmproj")

		if result.Status == "skip" {
			t.Error("When mmproj missing from profile subdir, status should not be 'skip'")
		}
		if result.Status != "warn" {
			t.Errorf("When mmproj missing without repo, status should be 'warn', got %q", result.Status)
		}
	})

	t.Run("When same mmproj name in flat dir but not profile subdir, sync does not find it", func(t *testing.T) {
		ggufDir := t.TempDir()
		mmprojDir := t.TempDir()

		os.WriteFile(filepath.Join(ggufDir, "model.gguf"), []byte("model"), 0644)
		os.WriteFile(filepath.Join(mmprojDir, "mmproj-BF16.gguf"), []byte("wrong-model-mmproj"), 0644)

		viper.Reset()
		defer viper.Reset()
		viper.Set("profiles.isolated.model", "model.gguf")
		viper.Set("profiles.isolated.mmproj", "mmproj-BF16.gguf")
		viper.Set("llama_server.gguf_dir", ggufDir)
		viper.Set("llama_server.mmproj_dir", mmprojDir)

		result := SyncProfile("isolated")

		if result.Status == "skip" {
			t.Error("When mmproj only in flat dir (not profile subdir), should not be 'skip'")
		}
	})

	t.Run("When two profiles have same mmproj filename, they are isolated", func(t *testing.T) {
		ggufDir := t.TempDir()
		mmprojDir := t.TempDir()

		os.WriteFile(filepath.Join(ggufDir, "model-a.gguf"), []byte("model-a"), 0644)
		os.WriteFile(filepath.Join(ggufDir, "model-b.gguf"), []byte("model-b"), 0644)

		dirA := filepath.Join(mmprojDir, "profile-a")
		dirB := filepath.Join(mmprojDir, "profile-b")
		os.MkdirAll(dirA, 0755)
		os.MkdirAll(dirB, 0755)
		os.WriteFile(filepath.Join(dirA, "mmproj-BF16.gguf"), []byte("mmproj-a"), 0644)
		os.WriteFile(filepath.Join(dirB, "mmproj-BF16.gguf"), []byte("mmproj-b"), 0644)

		viper.Reset()
		defer viper.Reset()
		viper.Set("profiles.profile-a.repo", "org/model-a")
		viper.Set("profiles.profile-a.model", "model-a.gguf")
		viper.Set("profiles.profile-a.mmproj", "mmproj-BF16.gguf")
		viper.Set("profiles.profile-b.repo", "org/model-b")
		viper.Set("profiles.profile-b.model", "model-b.gguf")
		viper.Set("profiles.profile-b.mmproj", "mmproj-BF16.gguf")
		viper.Set("llama_server.gguf_dir", ggufDir)
		viper.Set("llama_server.mmproj_dir", mmprojDir)

		resultA := SyncProfile("profile-a")
		resultB := SyncProfile("profile-b")

		if resultA.Status != "skip" {
			t.Errorf("profile-a should be 'skip', got %q", resultA.Status)
		}
		if resultB.Status != "skip" {
			t.Errorf("profile-b should be 'skip', got %q", resultB.Status)
		}
	})

	t.Run("When no mmproj configured, subdir is not required", func(t *testing.T) {
		ggufDir := t.TempDir()
		os.WriteFile(filepath.Join(ggufDir, "model.gguf"), []byte("model"), 0644)

		viper.Reset()
		defer viper.Reset()
		viper.Set("profiles.no-vision.repo", "org/repo")
		viper.Set("profiles.no-vision.model", "model.gguf")
		viper.Set("llama_server.gguf_dir", ggufDir)
		viper.Set("llama_server.mmproj_dir", t.TempDir())

		result := SyncProfile("no-vision")

		if result.Status != "skip" {
			t.Errorf("When no mmproj configured, status should be 'skip', got %q", result.Status)
		}
	})
}
