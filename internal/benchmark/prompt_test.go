package benchmark

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildPrompt(t *testing.T) {
	dir := t.TempDir()
	planFile := filepath.Join(dir, "PLAN.md")
	sourceHTML := filepath.Join(dir, "source.html")
	benchmarks := filepath.Join(dir, "benchmarks.json")

	os.WriteFile(planFile, []byte("# Test Plan"), 0644)
	os.WriteFile(sourceHTML, []byte("<html>test</html>"), 0644)
	os.WriteFile(benchmarks, []byte(`[{"model":"test"}]`), 0644)

	prompt, err := BuildPrompt(planFile, sourceHTML, benchmarks)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(prompt, "expert frontend developer") {
		t.Error("missing system prompt")
	}
	if !strings.Contains(prompt, "# Test Plan") {
		t.Error("missing plan content")
	}
	if !strings.Contains(prompt, "<html>test</html>") {
		t.Error("missing source HTML")
	}
}

func TestBuildPrompt_MissingFile(t *testing.T) {
	_, err := BuildPrompt("/nonexistent/plan.md", "/nonexistent/source.html", "/nonexistent/bench.json")
	if err == nil {
		t.Error("expected error for missing file")
	}
}

func TestBuildPrompt_TruncatesLargeSource(t *testing.T) {
	dir := t.TempDir()
	planFile := filepath.Join(dir, "PLAN.md")
	sourceHTML := filepath.Join(dir, "source.html")
	benchmarks := filepath.Join(dir, "benchmarks.json")

	os.WriteFile(planFile, []byte("plan"), 0644)
	os.WriteFile(sourceHTML, []byte(strings.Repeat("x", 100000)), 0644)
	os.WriteFile(benchmarks, []byte("[]"), 0644)

	prompt, err := BuildPrompt(planFile, sourceHTML, benchmarks)
	if err != nil {
		t.Fatal(err)
	}

	if len(prompt) > 60000 {
		t.Errorf("prompt too large (%d chars), source should be truncated", len(prompt))
	}
}

func TestBuildFormatRetryPrompt(t *testing.T) {
	result := BuildFormatRetryPrompt("original prompt")
	if !strings.Contains(result, "CRITICAL FORMAT REQUIREMENT") {
		t.Error("missing format fix header")
	}
	if !strings.HasSuffix(result, "original prompt") {
		t.Error("missing original prompt at end")
	}
}

func TestBuildSensitiveRetryPrompt(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "config.js"), []byte("const ip = '192.168.1.143';"), 0644)

	violations := []Violation{
		{Description: "Server IP", FilePath: "config.js"},
	}

	result, err := BuildSensitiveRetryPrompt(dir, violations)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "sensitive data") {
		t.Error("missing sensitive data mention")
	}
	if !strings.Contains(result, "Server IP") {
		t.Error("missing violation detail")
	}
	if !strings.Contains(result, "192.168.1.143") {
		t.Error("missing affected file content")
	}
}

func TestBuildBuildRetryPrompt(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"name":"test"}`), 0644)
	os.WriteFile(filepath.Join(dir, "astro.config.mjs"), []byte(`export default {}`), 0644)

	result, err := BuildBuildRetryPrompt(dir, "Cannot find module '@astrojs/node'")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "@astrojs/node") {
		t.Error("missing build error")
	}
	if !strings.Contains(result, "package.json") {
		t.Error("missing affected file")
	}
}

func TestLoadPromptTemplate_Embedded(t *testing.T) {
	content, err := loadPromptTemplate("system.md")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(content, "expert frontend developer") {
		t.Error("embedded system prompt not loaded correctly")
	}
}
