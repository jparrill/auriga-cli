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
hermes:
    moe_profile: local
`
	os.WriteFile(tmpFile.Name(), []byte(initial), 0644)
	viper.SetConfigFile(tmpFile.Name())

	pc := ProfileConfig{
		Repo:   "unsloth/gemma-4-12b-it-GGUF",
		Model:  "gemma-4-12b-it-Q4_K_M.gguf",
		MMProj: "mmproj-BF16.gguf",
		Flags:  []string{"--jinja"},
	}

	if err := addProfileToConfig("gemma4-12b-vision", pc); err != nil {
		t.Fatal(err)
	}

	content, _ := os.ReadFile(tmpFile.Name())
	result := string(content)

	if !strings.Contains(result, "gemma4-12b-vision") {
		t.Error("When adding a profile, the new profile name should appear in config")
	}
	if !strings.Contains(result, "gemma-4-12b-it-Q4_K_M.gguf") {
		t.Error("When adding a profile, the model should appear")
	}
	if !strings.Contains(result, "mmproj-BF16.gguf") {
		t.Error("When adding a profile with vision, mmproj should appear")
	}
	if !strings.Contains(result, "existing") {
		t.Error("When adding a profile, existing profiles should be preserved")
	}
	if !strings.Contains(result, "ollama") {
		t.Error("When adding a profile, other sections should be preserved")
	}
	if !strings.Contains(result, "hermes") {
		t.Error("When adding a profile, all sections should be preserved")
	}
	if !strings.Contains(result, "--jinja") {
		t.Error("When adding a profile with flags, flags should appear")
	}
}

func TestAddProfileToConfig_NoProfilesSection(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "auriga-config-*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())

	initial := "ollama:\n    host: http://localhost:11434\n"
	os.WriteFile(tmpFile.Name(), []byte(initial), 0644)
	viper.SetConfigFile(tmpFile.Name())

	pc := ProfileConfig{Model: "test.gguf"}
	if err := addProfileToConfig("new-profile", pc); err != nil {
		t.Fatal(err)
	}

	content, _ := os.ReadFile(tmpFile.Name())
	result := string(content)

	if !strings.Contains(result, "profiles") {
		t.Error("When no profiles section exists, it should be created")
	}
	if !strings.Contains(result, "new-profile") {
		t.Error("When no profiles section exists, the new profile should be added")
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

	if strings.Contains(result, "delete-me") {
		t.Error("When removing a profile, it should be gone from config")
	}
	if !strings.Contains(result, "keep-me") {
		t.Error("When removing a profile, other profiles should be preserved")
	}
	if !strings.Contains(result, "also-keep") {
		t.Error("When removing a profile, all other profiles should be preserved")
	}
}

func TestRemoveProfileFromConfig_NonExistent(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "auriga-config-*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())

	initial := "profiles:\n    existing:\n        model: test.gguf\n"
	os.WriteFile(tmpFile.Name(), []byte(initial), 0644)
	viper.SetConfigFile(tmpFile.Name())

	err = removeProfileFromConfig("nonexistent")
	if err != nil {
		t.Errorf("When removing a nonexistent profile, it should not error, got: %v", err)
	}

	content, _ := os.ReadFile(tmpFile.Name())
	if !strings.Contains(string(content), "existing") {
		t.Error("When removing a nonexistent profile, existing profiles should be preserved")
	}
}

func TestBuildProfileBlock(t *testing.T) {
	pc := ProfileConfig{
		Repo:   "unsloth/test-GGUF",
		Model:  "test-Q4_K_M.gguf",
		MMProj: "mmproj-BF16.gguf",
		Flags:  []string{"--jinja"},
	}

	lines := buildProfileBlock("test-vision", pc)
	if len(lines) != 5 {
		t.Errorf("When profile has vision + flags, it should produce 5 lines, got %d: %v", len(lines), lines)
	}
	if lines[0] != "  test-vision:" {
		t.Errorf("unexpected first line: %q", lines[0])
	}
}

func TestBuildProfileBlock_WithDFlash(t *testing.T) {
	pc := ProfileConfig{
		Repo:   "meta-models/Muse-Glimmer-30B-GGUF",
		Model:  "muse-glimmer-30B-kquant-17gb.gguf",
		DFlash: "dflash-kquant.gguf",
	}

	lines := buildProfileBlock("glimmer", pc)
	found := false
	for _, line := range lines {
		if strings.Contains(line, "dflash: dflash-kquant.gguf") {
			found = true
		}
	}
	if !found {
		t.Errorf("When profile has dflash, it should appear in block: %v", lines)
	}
}

func TestAddProfileToConfig_WithDFlash(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "auriga-config-*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())

	initial := "profiles:\n    existing:\n        model: some.gguf\n"
	os.WriteFile(tmpFile.Name(), []byte(initial), 0644)
	viper.SetConfigFile(tmpFile.Name())

	pc := ProfileConfig{
		Repo:   "meta-models/Muse-Glimmer-30B-GGUF",
		Model:  "glimmer.gguf",
		DFlash: "dflash-kquant.gguf",
	}

	if err := addProfileToConfig("glimmer", pc); err != nil {
		t.Fatal(err)
	}

	content, _ := os.ReadFile(tmpFile.Name())
	result := string(content)

	if !strings.Contains(result, "dflash") {
		t.Errorf("When profile has dflash, it should be in config. Got:\n%s", result)
	}
	if !strings.Contains(result, "dflash-kquant.gguf") {
		t.Errorf("When profile has dflash, filename should appear. Got:\n%s", result)
	}
}

func TestBuildProfileBlock_NoVision(t *testing.T) {
	pc := ProfileConfig{
		Repo:  "unsloth/test-GGUF",
		Model: "test-Q4_K_M.gguf",
	}

	lines := buildProfileBlock("test", pc)
	if len(lines) != 3 {
		t.Errorf("When profile has no vision and no flags, it should produce 3 lines, got %d: %v", len(lines), lines)
	}
}

func TestBuildProfileBlock_MultipleFlags(t *testing.T) {
	pc := ProfileConfig{
		Model: "model.gguf",
		Flags: []string{"--jinja", "--verbose"},
	}

	lines := buildProfileBlock("test", pc)
	found := false
	for _, line := range lines {
		if strings.Contains(line, "flags: [--jinja, --verbose]") {
			found = true
		}
	}
	if !found {
		t.Errorf("When profile has multiple flags, they should be comma-separated: %v", lines)
	}
}

func TestRemoveProfileFromConfig_SimilarNames(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "auriga-config-*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())

	initial := `profiles:
    qwen3:
        model: qwen3.gguf
    qwen3-vision:
        model: qwen3-vision.gguf
        mmproj: mmproj.gguf
`
	os.WriteFile(tmpFile.Name(), []byte(initial), 0644)
	viper.SetConfigFile(tmpFile.Name())

	if err := removeProfileFromConfig("qwen3"); err != nil {
		t.Fatal(err)
	}

	content, _ := os.ReadFile(tmpFile.Name())
	result := string(content)

	if strings.Contains(result, "qwen3.gguf") {
		t.Error("When removing 'qwen3', its model should be gone")
	}
	if !strings.Contains(result, "qwen3-vision") {
		t.Error("When removing 'qwen3', 'qwen3-vision' should NOT be affected")
	}
}

func TestAddProfileToConfig_WithFlags(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "auriga-config-*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())

	initial := "profiles:\n    existing:\n        model: some.gguf\n"
	os.WriteFile(tmpFile.Name(), []byte(initial), 0644)
	viper.SetConfigFile(tmpFile.Name())

	pc := ProfileConfig{
		Model:  "vision-model.gguf",
		MMProj: "mmproj.gguf",
		Flags:  []string{"--jinja"},
	}

	if err := addProfileToConfig("vision-profile", pc); err != nil {
		t.Fatal(err)
	}

	content, _ := os.ReadFile(tmpFile.Name())
	result := string(content)

	if !strings.Contains(result, "--jinja") {
		t.Errorf("When profile has flags, they should be written to config. Got:\n%s", result)
	}
}
