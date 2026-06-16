package profile

import (
	"os"
	"strings"
	"testing"

	"github.com/spf13/viper"
)

func TestAddProfileToConfig(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "auriga-config-*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())

	initial := `ollama:
  host: http://localhost:11434

profiles:
  existing:
    model: some-model.gguf

pi:
  bin: pi
`
	os.WriteFile(tmpFile.Name(), []byte(initial), 0644)
	viper.SetConfigFile(tmpFile.Name())

	pc := ProfileConfig{
		Repo:   "unsloth/gemma-4-12b-it-GGUF",
		Model:  "gemma-4-12b-it-Q4_K_M.gguf",
		MMProj: "mmproj-BF16.gguf",
	}

	if err := addProfileToConfig("gemma4-12b-vision", pc); err != nil {
		t.Fatal(err)
	}

	content, _ := os.ReadFile(tmpFile.Name())
	result := string(content)

	if !strings.Contains(result, "gemma4-12b-vision:") {
		t.Error("new profile not found in config")
	}
	if !strings.Contains(result, "repo: unsloth/gemma-4-12b-it-GGUF") {
		t.Error("repo not found")
	}
	if !strings.Contains(result, "mmproj: mmproj-BF16.gguf") {
		t.Error("mmproj not found")
	}
	if !strings.Contains(result, "existing:") {
		t.Error("existing profile was removed")
	}
	if !strings.Contains(result, "ollama:") {
		t.Error("ollama section was removed")
	}
	if !strings.Contains(result, "pi:") {
		t.Error("pi section was removed")
	}
}

func TestRemoveProfileFromConfig(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "auriga-config-*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())

	initial := `profiles:
  keep-me:
    model: keep.gguf
  delete-me:
    repo: unsloth/something
    model: delete.gguf
    mmproj: mmproj.gguf
  also-keep:
    model: also.gguf
`
	os.WriteFile(tmpFile.Name(), []byte(initial), 0644)
	viper.SetConfigFile(tmpFile.Name())

	if err := removeProfileFromConfig("delete-me"); err != nil {
		t.Fatal(err)
	}

	content, _ := os.ReadFile(tmpFile.Name())
	result := string(content)

	if strings.Contains(result, "delete-me:") {
		t.Error("deleted profile still present")
	}
	if !strings.Contains(result, "keep-me:") {
		t.Error("keep-me was removed")
	}
	if !strings.Contains(result, "also-keep:") {
		t.Error("also-keep was removed")
	}
}

func TestBuildProfileBlock(t *testing.T) {
	pc := ProfileConfig{
		Repo:   "unsloth/test-GGUF",
		Model:  "test-Q4_K_M.gguf",
		MMProj: "mmproj-BF16.gguf",
	}

	lines := buildProfileBlock("test-vision", pc)
	if len(lines) != 4 {
		t.Errorf("expected 4 lines, got %d: %v", len(lines), lines)
	}
	if lines[0] != "  test-vision:" {
		t.Errorf("unexpected first line: %q", lines[0])
	}
}

func TestBuildProfileBlock_NoVision(t *testing.T) {
	pc := ProfileConfig{
		Repo:  "unsloth/test-GGUF",
		Model: "test-Q4_K_M.gguf",
	}

	lines := buildProfileBlock("test", pc)
	if len(lines) != 3 {
		t.Errorf("expected 3 lines (no mmproj), got %d: %v", len(lines), lines)
	}
}
