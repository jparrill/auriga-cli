package profile

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/spf13/viper"
)

func TestResolveProfilePreflight_ValidProfile(t *testing.T) {
	viper.Reset()
	defer viper.Reset()

	root := t.TempDir()
	ggufDir := filepath.Join(root, "gguf")
	mmprojDir := filepath.Join(root, "mmproj")
	profileMMProjDir := filepath.Join(mmprojDir, "vision")
	if err := os.MkdirAll(profileMMProjDir, 0755); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{
		filepath.Join(ggufDir, "model.gguf"),
		filepath.Join(profileMMProjDir, "vision.gguf"),
		filepath.Join(ggufDir, "draft.gguf"),
		filepath.Join(root, "llama-server"),
	} {
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("fixture"), 0755); err != nil {
			t.Fatal(err)
		}
	}

	viper.Set("llama_server.gguf_dir", ggufDir)
	viper.Set("llama_server.mmproj_dir", mmprojDir)
	viper.Set("llama_server.slot_2_port", 9101)
	viper.Set("profiles.vision.model", "model.gguf")
	viper.Set("profiles.vision.mmproj", "vision.gguf")
	viper.Set("profiles.vision.mtp_drafter", "draft.gguf")
	viper.Set("profiles.vision.bin", filepath.Join(root, "llama-server"))
	viper.Set("profiles.vision.type", "moe")
	viper.Set("profiles.vision.flags", []string{"--batch-size", "1024"})

	got, err := resolveProfilePreflight("vision", 65536, 2, "auriga profile sync")
	if err != nil {
		t.Fatal(err)
	}
	if got.ModelPath != filepath.Join(ggufDir, "model.gguf") {
		t.Errorf("model path = %q", got.ModelPath)
	}
	if got.MMProjPath != filepath.Join(profileMMProjDir, "vision.gguf") {
		t.Errorf("mmproj path = %q", got.MMProjPath)
	}
	if got.Binary != filepath.Join(root, "llama-server") || got.Port != 9101 {
		t.Errorf("unexpected binary/port: %q/%d", got.Binary, got.Port)
	}
	if got.ContextSize != 65536 || got.Type != "moe" {
		t.Errorf("unexpected context/type: %d/%s", got.ContextSize, got.Type)
	}
	if len(got.Flags) != 2 || got.Flags[0] != "--batch-size" {
		t.Errorf("unexpected flags: %v", got.Flags)
	}
}

func TestResolveProfilePreflight_UsesLegacyMmprojPath(t *testing.T) {
	viper.Reset()
	defer viper.Reset()

	root := t.TempDir()
	ggufDir := filepath.Join(root, "gguf")
	mmprojDir := filepath.Join(root, "mmproj")
	if err := os.MkdirAll(ggufDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(mmprojDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ggufDir, "model.gguf"), []byte("fixture"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(mmprojDir, "vision.gguf"), []byte("fixture"), 0644); err != nil {
		t.Fatal(err)
	}
	bin := filepath.Join(root, "llama-server")
	if err := os.WriteFile(bin, []byte("fixture"), 0755); err != nil {
		t.Fatal(err)
	}

	viper.Set("llama_server.gguf_dir", ggufDir)
	viper.Set("llama_server.mmproj_dir", mmprojDir)
	viper.Set("llama_server.bin", bin)
	viper.Set("profiles.vision.model", "model.gguf")
	viper.Set("profiles.vision.mmproj", "vision.gguf")

	got, err := resolveProfilePreflight("vision", 131072, 1, "auriga model ensure")
	if err != nil {
		t.Fatal(err)
	}
	if got.MMProjPath != filepath.Join(mmprojDir, "vision.gguf") {
		t.Errorf("expected legacy mmproj path, got %q", got.MMProjPath)
	}
}

func TestResolveProfilePreflight_MissingAssetReturnsActionableError(t *testing.T) {
	viper.Reset()
	defer viper.Reset()

	viper.Set("llama_server.gguf_dir", t.TempDir())
	viper.Set("profiles.test.model", "missing.gguf")

	_, err := resolveProfilePreflight("test", 131072, 1, "auriga profile sync")
	if err == nil || !strings.Contains(err.Error(), "auriga profile sync") {
		t.Fatalf("expected actionable missing model error, got %v", err)
	}
}

func TestResolveProfilePreflight_ServeAndSwitchUseSameResolvedInputs(t *testing.T) {
	viper.Reset()
	defer viper.Reset()

	root := t.TempDir()
	ggufDir := filepath.Join(root, "gguf")
	if err := os.MkdirAll(ggufDir, 0755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"model.gguf", "llama-server"} {
		mode := os.FileMode(0644)
		if name == "llama-server" {
			mode = 0755
		}
		if err := os.WriteFile(filepath.Join(ggufDir, name), []byte("fixture"), mode); err != nil {
			t.Fatal(err)
		}
	}
	viper.Set("llama_server.gguf_dir", ggufDir)
	viper.Set("llama_server.bin", filepath.Join(ggufDir, "llama-server"))
	viper.Set("profiles.test.model", "model.gguf")
	viper.Set("profiles.test.flags", []string{"--jinja"})

	serve, err := resolveProfilePreflight("test", 65536, 1, "auriga model ensure")
	if err != nil {
		t.Fatal(err)
	}
	switchResult, err := resolveProfilePreflight("test", 65536, 1, "auriga profile sync")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(serve, switchResult) {
		t.Errorf("serve and switch preflight differ: %#v vs %#v", serve, switchResult)
	}
}
