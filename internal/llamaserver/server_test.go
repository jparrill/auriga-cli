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

func TestDensePort_FromConfig(t *testing.T) {
	viper.Reset()
	defer viper.Reset()

	viper.Set("llama_server.dense_port", 8090)
	viper.Set("llama_server.host", "http://localhost:8090")

	p := DensePort()
	if p != 8090 {
		t.Errorf("When dense_port set, DensePort should return it, got %d", p)
	}
}

func TestDensePort_FallbackToPort(t *testing.T) {
	viper.Reset()
	defer viper.Reset()

	viper.Set("llama_server.host", "http://localhost:9090")

	p := DensePort()
	if p != 9090 {
		t.Errorf("When dense_port not set, DensePort should fallback to Port(), got %d", p)
	}
}

func TestMoePort_FromConfig(t *testing.T) {
	viper.Reset()
	defer viper.Reset()

	viper.Set("llama_server.moe_port", 8091)

	p := MoePort()
	if p != 8091 {
		t.Errorf("When moe_port set, MoePort should return it, got %d", p)
	}
}

func TestMoePort_FallbackDefault(t *testing.T) {
	viper.Reset()
	defer viper.Reset()

	p := MoePort()
	if p != 8091 {
		t.Errorf("When moe_port not set, MoePort should return 8091, got %d", p)
	}
}

func TestHostForPort_ReplacesPort(t *testing.T) {
	viper.Reset()
	defer viper.Reset()

	viper.Set("llama_server.host", "http://localhost:8090")

	got := HostForPort(8091)
	if got != "http://localhost:8091" {
		t.Errorf("When replacing port, HostForPort should update it, got %q", got)
	}
}

func TestHostForPort_DifferentHost(t *testing.T) {
	viper.Reset()
	defer viper.Reset()

	viper.Set("llama_server.host", "http://auriga:8090")

	got := HostForPort(9000)
	if got != "http://auriga:9000" {
		t.Errorf("When custom host, HostForPort should keep hostname, got %q", got)
	}
}

func TestHostForPort_NoPortInHost(t *testing.T) {
	viper.Reset()
	defer viper.Reset()

	viper.Set("llama_server.host", "http://localhost")

	got := HostForPort(8091)
	if got != "http://localhost:8091" {
		t.Errorf("When no port in host, HostForPort should construct URL, got %q", got)
	}
}

func TestHostForPort_SamePort(t *testing.T) {
	viper.Reset()
	defer viper.Reset()

	viper.Set("llama_server.host", "http://localhost:8090")

	got := HostForPort(8090)
	if got != "http://localhost:8090" {
		t.Errorf("When same port, HostForPort should return unchanged, got %q", got)
	}
}
