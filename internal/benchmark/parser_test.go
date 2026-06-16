package benchmark

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseFiles_Strict(t *testing.T) {
	raw := `--- FILE: package.json ---
{"name": "test"}
--- END FILE ---
--- FILE: src/index.js ---
console.log("hello");
--- END FILE ---`

	dir := t.TempDir()
	count, err := ParseFiles(raw, dir)
	if err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Errorf("ParseFiles() = %d files, want 2", count)
	}

	content, _ := os.ReadFile(filepath.Join(dir, "package.json"))
	if string(content) != "{\"name\": \"test\"}\n" {
		t.Errorf("unexpected content: %q", string(content))
	}

	content, _ = os.ReadFile(filepath.Join(dir, "src", "index.js"))
	if string(content) != "console.log(\"hello\");\n" {
		t.Errorf("unexpected content: %q", string(content))
	}
}

func TestParseFiles_Backtick(t *testing.T) {
	raw := "--- FILE: config.mjs ---\n```javascript\nexport default {};\n```\n--- END FILE ---\n"

	dir := t.TempDir()
	count, err := ParseFiles(raw, dir)
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Errorf("ParseFiles() = %d files, want 1", count)
	}

	content, _ := os.ReadFile(filepath.Join(dir, "config.mjs"))
	if string(content) != "export default {};\n" {
		t.Errorf("unexpected content: %q", string(content))
	}
}

func TestParseFiles_NoMarkers(t *testing.T) {
	raw := "just some random text without markers"

	dir := t.TempDir()
	count, err := ParseFiles(raw, dir)
	if err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Errorf("ParseFiles() = %d files, want 0", count)
	}

	_, err = os.Stat(filepath.Join(dir, "_raw_output.txt"))
	if err != nil {
		t.Error("expected _raw_output.txt to be created")
	}
}

func TestParseFiles_NestedDirs(t *testing.T) {
	raw := `--- FILE: src/pages/index.astro ---
<h1>Hello</h1>
--- END FILE ---
--- FILE: src/components/Layout.astro ---
<html><body><slot/></body></html>
--- END FILE ---`

	dir := t.TempDir()
	count, _ := ParseFiles(raw, dir)
	if count != 2 {
		t.Errorf("ParseFiles() = %d files, want 2", count)
	}

	if _, err := os.Stat(filepath.Join(dir, "src", "pages", "index.astro")); err != nil {
		t.Error("expected src/pages/index.astro")
	}
	if _, err := os.Stat(filepath.Join(dir, "src", "components", "Layout.astro")); err != nil {
		t.Error("expected src/components/Layout.astro")
	}
}
