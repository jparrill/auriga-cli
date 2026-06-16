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
	if !strings.Contains(prompt, `"model":"test"`) {
		t.Error("missing benchmarks")
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

	// Source should be truncated to 50000
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
	violations := []Violation{
		{Description: "Server IP", FilePath: "config.js"},
		{Description: "Email", FilePath: "about.md"},
	}
	result := BuildSensitiveRetryPrompt("original", violations)
	if !strings.Contains(result, "FIX REQUIRED") {
		t.Error("missing fix header")
	}
	if !strings.Contains(result, "Server IP in config.js") {
		t.Error("missing violation detail")
	}
}

func TestBuildBuildRetryPrompt(t *testing.T) {
	result := BuildBuildRetryPrompt("original", "npm ERR! missing dep")
	if !strings.Contains(result, "npm ERR! missing dep") {
		t.Error("missing build error")
	}
	if !strings.Contains(result, "@astrojs/node") {
		t.Error("missing common problems hint")
	}
}
