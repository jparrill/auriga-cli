package llamaserver

import (
	"os"
	"path/filepath"
	"strings"
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

func TestBinForProfile_WithProfileBin(t *testing.T) {
	viper.Reset()
	defer viper.Reset()

	viper.Set("llama_server.bin", "~/infra/bin/llama-server")
	viper.Set("profiles.strix-test.bin", "/custom/path/llama-server-strix-halo")

	got := BinForProfile("strix-test")
	if got != "/custom/path/llama-server-strix-halo" {
		t.Errorf("expected custom bin path, got %q", got)
	}
}

func TestBinForProfile_WithoutProfileBin(t *testing.T) {
	viper.Reset()
	defer viper.Reset()

	viper.Set("llama_server.bin", "/global/llama-server")

	got := BinForProfile("some-profile")
	if got != "/global/llama-server" {
		t.Errorf("expected global bin, got %q", got)
	}
}

func TestBinForProfile_EmptyProfileName(t *testing.T) {
	viper.Reset()
	defer viper.Reset()

	viper.Set("llama_server.bin", "/global/llama-server")

	got := BinForProfile("")
	if got != "/global/llama-server" {
		t.Errorf("expected global bin for empty profile, got %q", got)
	}
}

func TestBinForProfile_ExpandsHome(t *testing.T) {
	viper.Reset()
	defer viper.Reset()

	viper.Set("llama_server.bin", "/global/llama-server")
	viper.Set("profiles.home-test.bin", "~/Projects/strix-halo-llamacpp/vulkan/llama-server-strix-halo")

	got := BinForProfile("home-test")
	if got == "~/Projects/strix-halo-llamacpp/vulkan/llama-server-strix-halo" {
		t.Error("expected expanded path, got unexpanded")
	}
	if !strings.Contains(got, "Projects/strix-halo-llamacpp/vulkan/llama-server-strix-halo") {
		t.Errorf("expanded path should contain the relative part, got %q", got)
	}
}

func TestAllBinPaths_GlobalOnly(t *testing.T) {
	viper.Reset()
	defer viper.Reset()

	viper.Set("llama_server.bin", "/global/llama-server")

	paths := AllBinPaths()
	if len(paths) != 1 {
		t.Errorf("expected 1 path, got %d", len(paths))
	}
	if paths[0] != "/global/llama-server" {
		t.Errorf("expected global bin, got %q", paths[0])
	}
}

func TestAllBinPaths_WithProfileBins(t *testing.T) {
	viper.Reset()
	defer viper.Reset()

	viper.Set("llama_server.bin", "/global/llama-server")
	viper.Set("profiles.p1.bin", "/custom/strix-halo")
	viper.Set("profiles.p1.model", "test.gguf")
	viper.Set("profiles.p2.model", "test2.gguf")
	viper.Set("profiles.p3.bin", "/custom/strix-halo")
	viper.Set("profiles.p3.model", "test3.gguf")

	paths := AllBinPaths()
	if len(paths) != 2 {
		t.Errorf("expected 2 unique paths (global + 1 custom), got %d: %v", len(paths), paths)
	}
}

func TestAllBinPaths_NoDuplicates(t *testing.T) {
	viper.Reset()
	defer viper.Reset()

	viper.Set("llama_server.bin", "/same/llama-server")
	viper.Set("profiles.p1.bin", "/same/llama-server")
	viper.Set("profiles.p1.model", "test.gguf")

	paths := AllBinPaths()
	if len(paths) != 1 {
		t.Errorf("expected 1 unique path (same as global), got %d: %v", len(paths), paths)
	}
}

func TestStartWithCtx_BinaryNotFound(t *testing.T) {
	_, err := StartWithCtx(
		t.Context(),
		"/nonexistent/llama-server",
		"/some/model.gguf",
		"",
		nil,
		65536,
		8090,
	)
	if err == nil {
		t.Fatal("expected error for missing binary")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error should mention 'not found', got: %s", err)
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
