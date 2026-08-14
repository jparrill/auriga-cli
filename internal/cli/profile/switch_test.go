package profile

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/viper"
)

func TestNewProfileSwitchCmd_Registration(t *testing.T) {
	cmd := newProfileSwitchCmd()

	if cmd.Use != "switch <profile-name>" {
		t.Errorf("When creating switch command, Use should be 'switch <profile-name>', got %q", cmd.Use)
	}
	if cmd.Short == "" {
		t.Error("When creating switch command, it should have a short description")
	}
}

func TestNewProfileSwitchCmd_Flags(t *testing.T) {
	cmd := newProfileSwitchCmd()

	tests := []struct {
		name     string
		flagName string
	}{
		{"When checking switch flags, it should have persistent flag", "persistent"},
		{"When checking switch flags, it should have ctx-size flag", "ctx-size"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			flag := cmd.Flags().Lookup(tt.flagName)
			if flag == nil {
				t.Errorf("flag %q not found", tt.flagName)
			}
		})
	}
}

func TestNewProfileSwitchCmd_PersistentDefault(t *testing.T) {
	cmd := newProfileSwitchCmd()

	flag := cmd.Flags().Lookup("persistent")
	if flag.DefValue != "false" {
		t.Errorf("When checking persistent default, got %q, want false", flag.DefValue)
	}
}

func TestNewProfileSwitchCmd_CtxSizeDefault(t *testing.T) {
	cmd := newProfileSwitchCmd()

	flag := cmd.Flags().Lookup("ctx-size")
	if flag.DefValue != "65536" {
		t.Errorf("When checking ctx-size default, got %q, want 65536", flag.DefValue)
	}
}

func TestNewProfileSwitchCmd_RequiresArgs(t *testing.T) {
	cmd := newProfileSwitchCmd()
	cmd.SetArgs([]string{})
	err := cmd.Execute()
	if err == nil {
		t.Error("When no args provided, switch should error")
	}
}

func TestRunProfileSwitch_ProfileNotFound(t *testing.T) {
	viper.Reset()
	defer viper.Reset()

	err := runProfileSwitch("nonexistent", false, 65536)

	if err == nil {
		t.Error("When profile not found, it should return error")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("When profile not found, error should mention 'not found', got: %v", err)
	}
}

func TestRunProfileSwitch_ModelFileMissing(t *testing.T) {
	viper.Reset()
	defer viper.Reset()

	viper.Set("profiles.test-profile.model", "nonexistent.gguf")
	viper.Set("llama_server.gguf_dir", "/tmp/auriga-test-nonexistent")

	err := runProfileSwitch("test-profile", false, 65536)

	if err == nil {
		t.Error("When model file missing, it should return error")
	}
	if !strings.Contains(err.Error(), "model not found") {
		t.Errorf("When model missing, error should mention model, got: %v", err)
	}
	if !strings.Contains(err.Error(), "profile sync") {
		t.Errorf("When model missing, error should suggest sync, got: %v", err)
	}
}

func TestRunProfileSwitch_MmprojFileMissing(t *testing.T) {
	viper.Reset()
	defer viper.Reset()

	tmpDir := t.TempDir()
	modelFile := filepath.Join(tmpDir, "model.gguf")
	os.WriteFile(modelFile, []byte("fake model"), 0644)

	viper.Set("profiles.test-profile.model", "model.gguf")
	viper.Set("profiles.test-profile.mmproj", "mmproj.gguf")
	viper.Set("llama_server.gguf_dir", tmpDir)
	viper.Set("llama_server.mmproj_dir", "/tmp/auriga-test-nonexistent")

	err := runProfileSwitch("test-profile", false, 65536)

	if err == nil {
		t.Error("When mmproj file missing, it should return error")
	}
	if !strings.Contains(err.Error(), "mmproj not found") {
		t.Errorf("When mmproj missing, error should mention mmproj, got: %v", err)
	}
}

func TestBuildExecStart_BasicModel(t *testing.T) {
	viper.Reset()
	defer viper.Reset()

	viper.Set("llama_server.bin", "/usr/bin/llama-server")
	viper.Set("llama_server.host", "http://localhost:8090")

	got := buildExecStart("/models/model.gguf", "", nil, 65536)

	checks := []struct {
		name string
		want string
	}{
		{"When building ExecStart, it should have binary path", "/usr/bin/llama-server"},
		{"When building ExecStart, it should have model flag", "-m /models/model.gguf"},
		{"When building ExecStart, it should have host", "--host 0.0.0.0"},
		{"When building ExecStart, it should have port", "--port 8090"},
		{"When building ExecStart, it should have flash-attn", "--flash-attn on"},
		{"When building ExecStart, it should have gpu-layers", "--gpu-layers 99"},
		{"When building ExecStart, it should have ctx-size", "--ctx-size 65536"},
	}

	for _, c := range checks {
		t.Run(c.name, func(t *testing.T) {
			if !strings.Contains(got, c.want) {
				t.Errorf("expected %q in %q", c.want, got)
			}
		})
	}
}

func TestBuildExecStart_WithMmproj(t *testing.T) {
	viper.Reset()
	defer viper.Reset()

	viper.Set("llama_server.bin", "/usr/bin/llama-server")
	viper.Set("llama_server.host", "http://localhost:8090")

	got := buildExecStart("/models/model.gguf", "/models/mmproj.gguf", nil, 65536)

	if !strings.Contains(got, "--mmproj /models/mmproj.gguf") {
		t.Errorf("When mmproj set, ExecStart should contain --mmproj, got: %s", got)
	}
	if !strings.Contains(got, "--jinja") {
		t.Errorf("When mmproj set, ExecStart should contain --jinja, got: %s", got)
	}
}

func TestBuildExecStart_WithExtraFlags(t *testing.T) {
	viper.Reset()
	defer viper.Reset()

	viper.Set("llama_server.bin", "/usr/bin/llama-server")
	viper.Set("llama_server.host", "http://localhost:8090")

	got := buildExecStart("/models/model.gguf", "", []string{"--threads", "16"}, 65536)

	if !strings.Contains(got, "--threads 16") {
		t.Errorf("When extra flags set, ExecStart should contain them, got: %s", got)
	}
}

func TestBuildExecStart_CustomCtxSize(t *testing.T) {
	viper.Reset()
	defer viper.Reset()

	viper.Set("llama_server.bin", "/usr/bin/llama-server")
	viper.Set("llama_server.host", "http://localhost:8090")

	got := buildExecStart("/models/model.gguf", "", nil, 131072)

	if !strings.Contains(got, "--ctx-size 131072") {
		t.Errorf("When custom ctx-size, ExecStart should use it, got: %s", got)
	}
}

func TestBuildExecStart_NoMmprojNoJinja(t *testing.T) {
	viper.Reset()
	defer viper.Reset()

	viper.Set("llama_server.bin", "/usr/bin/llama-server")
	viper.Set("llama_server.host", "http://localhost:8090")

	got := buildExecStart("/models/model.gguf", "", nil, 65536)

	if strings.Contains(got, "--mmproj") {
		t.Errorf("When no mmproj, ExecStart should not have --mmproj, got: %s", got)
	}
	if strings.Contains(got, "--jinja") {
		t.Errorf("When no mmproj, ExecStart should not have --jinja, got: %s", got)
	}
}

func TestProfileCmd_HasSwitchSubcommand(t *testing.T) {
	cmd := NewProfileCmd()

	found := false
	for _, sub := range cmd.Commands() {
		if sub.Name() == "switch" {
			found = true
			break
		}
	}
	if !found {
		t.Error("When listing profile subcommands, switch should be registered")
	}
}
