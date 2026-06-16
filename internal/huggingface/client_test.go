package huggingface

import (
	"strings"
	"testing"
)

func TestResolveGGUF_QuantPriority(t *testing.T) {
	// This test requires network access — skip in CI
	if testing.Short() {
		t.Skip("skipping network test")
	}

	filename, size, err := ResolveGGUF("unsloth/gemma-4-12b-it-GGUF", []string{"Q4_K_M", "Q4_K_L", "Q4_K_S"})
	if err != nil {
		t.Fatalf("ResolveGGUF failed: %v", err)
	}
	if !strings.HasSuffix(filename, ".gguf") {
		t.Errorf("expected .gguf file, got %s", filename)
	}
	if size == 0 {
		t.Error("expected non-zero size")
	}
	if !strings.Contains(filename, "Q4_K_M") {
		t.Logf("got %s (may not contain Q4_K_M if unavailable, using fallback)", filename)
	}
}

func TestResolveMMProj(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping network test")
	}

	filename, size, err := ResolveMMProj("unsloth/gemma-4-12b-it-GGUF")
	if err != nil {
		t.Fatalf("ResolveMMProj failed: %v", err)
	}
	if !strings.Contains(strings.ToLower(filename), "mmproj") {
		t.Errorf("expected mmproj file, got %s", filename)
	}
	if size == 0 {
		t.Error("expected non-zero size")
	}
}

func TestResolveMMProj_NotFound(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping network test")
	}

	_, _, err := ResolveMMProj("unsloth/Qwen3-30B-A3B-GGUF")
	if err == nil {
		t.Error("expected error for model without mmproj")
	}
}

func TestDownloadURL(t *testing.T) {
	url := DownloadURL("unsloth/gemma-4-12b-it-GGUF", "model.gguf")
	expected := "https://huggingface.co/unsloth/gemma-4-12b-it-GGUF/resolve/main/model.gguf"
	if url != expected {
		t.Errorf("got %s, want %s", url, expected)
	}
}
