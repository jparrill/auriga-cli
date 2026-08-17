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

func TestResolveDFlash(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping network test")
	}

	filename, size, err := ResolveDFlash("meta-models/Muse-Glimmer-30B-GGUF")
	if err != nil {
		t.Fatalf("ResolveDFlash failed: %v", err)
	}
	if !strings.Contains(strings.ToLower(filename), "dflash") {
		t.Errorf("expected dflash file, got %s", filename)
	}
	if size == 0 {
		t.Error("expected non-zero size")
	}
}

func TestResolveDFlash_NotFound(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping network test")
	}

	_, _, err := ResolveDFlash("unsloth/gemma-4-12b-it-GGUF")
	if err == nil {
		t.Error("expected error for model without dflash")
	}
}

func TestDownloadURL(t *testing.T) {
	url := DownloadURL("unsloth/gemma-4-12b-it-GGUF", "model.gguf")
	expected := "https://huggingface.co/unsloth/gemma-4-12b-it-GGUF/resolve/main/model.gguf"
	if url != expected {
		t.Errorf("got %s, want %s", url, expected)
	}
}

func TestExpectedHash(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping network test")
	}

	hash, size, err := ExpectedHash("unsloth/gemma-4-26B-A4B-it-GGUF", "gemma-4-26B-A4B-it-UD-Q4_K_M.gguf")
	if err != nil {
		t.Fatalf("ExpectedHash failed: %v", err)
	}
	if hash == "" {
		t.Error("When file has LFS, it should return a SHA256 hash")
	}
	if len(hash) != 64 {
		t.Errorf("When returning SHA256, hash should be 64 hex chars, got %d: %s", len(hash), hash)
	}
	if size == 0 {
		t.Error("When file exists, it should return non-zero size")
	}
}

func TestExpectedHash_FileNotFound(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping network test")
	}

	_, _, err := ExpectedHash("unsloth/gemma-4-26B-A4B-it-GGUF", "nonexistent.gguf")
	if err == nil {
		t.Error("When file not in repo, it should return an error")
	}
}

func TestLFSInfo_ParsedFromAPI(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping network test")
	}

	files, err := ListFiles("unsloth/gemma-4-26B-A4B-it-GGUF")
	if err != nil {
		t.Fatalf("ListFiles failed: %v", err)
	}

	foundLFS := false
	for _, f := range files {
		if strings.HasSuffix(f.Path, ".gguf") && f.LFS != nil {
			foundLFS = true
			if f.LFS.OID == "" {
				t.Errorf("When GGUF has LFS, OID should not be empty for %s", f.Path)
			}
			if f.LFS.Size == 0 {
				t.Errorf("When GGUF has LFS, Size should not be zero for %s", f.Path)
			}
			break
		}
	}
	if !foundLFS {
		t.Error("When listing GGUF repo, at least one file should have LFS info")
	}
}
