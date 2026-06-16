package llamaserver

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/viper"
)

func TestFindLocalGGUF_Found(t *testing.T) {
	dir := t.TempDir()
	testFile := filepath.Join(dir, "Qwen3-Coder-Next-UD-Q4_K_M.gguf")
	os.WriteFile(testFile, []byte("test"), 0644)

	viper.Set("llama_server.gguf_dir", dir)

	result := FindLocalGGUF("unsloth/Qwen3-Coder-Next-GGUF")
	if result == "" {
		t.Error("expected to find GGUF, got empty")
	}
	if filepath.Base(result) != "Qwen3-Coder-Next-UD-Q4_K_M.gguf" {
		t.Errorf("unexpected filename: %s", result)
	}
}

func TestFindLocalGGUF_NotFound(t *testing.T) {
	dir := t.TempDir()
	viper.Set("llama_server.gguf_dir", dir)

	result := FindLocalGGUF("nonexistent/model-GGUF")
	if result != "" {
		t.Errorf("expected empty, got %s", result)
	}
}

func TestFindLocalGGUF_StripsGGUFSuffix(t *testing.T) {
	dir := t.TempDir()
	testFile := filepath.Join(dir, "gemma-4-12b-it-Q4_K_M.gguf")
	os.WriteFile(testFile, []byte("test"), 0644)

	viper.Set("llama_server.gguf_dir", dir)

	result := FindLocalGGUF("unsloth/gemma-4-12b-it-GGUF")
	if result == "" {
		t.Error("expected to find GGUF after stripping -GGUF suffix")
	}
}

func TestPort_FromHost(t *testing.T) {
	viper.Set("llama_server.host", "http://localhost:9090")
	p := Port()
	if p != 9090 {
		t.Errorf("expected 9090, got %d", p)
	}
}

func TestPort_Default(t *testing.T) {
	viper.Set("llama_server.host", "http://localhost")
	p := Port()
	if p != 8090 {
		t.Errorf("expected default 8090, got %d", p)
	}
}

func TestHost(t *testing.T) {
	viper.Set("llama_server.host", "http://test:1234")
	h := Host()
	if h != "http://test:1234" {
		t.Errorf("expected http://test:1234, got %s", h)
	}
}

func TestBin(t *testing.T) {
	viper.Set("llama_server.bin", "~/infra/bin/llama-server")
	b := Bin()
	if b == "~/infra/bin/llama-server" {
		t.Error("expected expanded path, got unexpanded")
	}
}
