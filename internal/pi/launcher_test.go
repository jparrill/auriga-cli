package pi

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/viper"
)

func TestWriteSystemMD(t *testing.T) {
	dir := t.TempDir()
	projectDir := filepath.Join(dir, "project")
	os.MkdirAll(projectDir, 0755)

	err := WriteSystemMD(projectDir, "gemma4:26b", "ollama", "FAIL", 15)
	if err != nil {
		t.Fatal(err)
	}

	content, err := os.ReadFile(filepath.Join(projectDir, ".pi", "SYSTEM.md"))
	if err != nil {
		t.Fatal("SYSTEM.md not created")
	}

	s := string(content)
	if !strings.Contains(s, "gemma4:26b") {
		t.Error("missing model name")
	}
	if !strings.Contains(s, "ollama") {
		t.Error("missing backend")
	}
	if !strings.Contains(s, "FAIL") {
		t.Error("missing status")
	}
	if !strings.Contains(s, "npm install") {
		t.Error("missing workflow instructions")
	}
	if !strings.Contains(s, "@astrojs/node") {
		t.Error("missing common errors")
	}
}

func TestWriteSystemMD_CreatesDir(t *testing.T) {
	dir := t.TempDir()
	projectDir := filepath.Join(dir, "deep", "nested", "project")

	err := WriteSystemMD(projectDir, "test", "ollama", "PASS", 0)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(filepath.Join(projectDir, ".pi", "SYSTEM.md")); err != nil {
		t.Error("SYSTEM.md not created in nested dir")
	}
}

func TestBin(t *testing.T) {
	viper.Set("pi.bin", "~/test/pi")
	b := Bin()
	if strings.HasPrefix(b, "~") {
		t.Error("expected expanded path")
	}
}
