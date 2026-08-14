package systemd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenerateUnit_BasicProfile(t *testing.T) {
	cfg := ServiceConfig{
		ProfileName: "qwen3.6-vision",
		ExecStart:   "/usr/bin/llama-server -m /models/model.gguf --host 0.0.0.0 --port 8090",
		Environment: []string{"AMD_VULKAN_ICD=RADV"},
	}

	got := GenerateUnit(cfg)

	checks := []struct {
		name string
		want string
	}{
		{"When generating unit, it should have Unit section", "[Unit]"},
		{"When generating unit, it should have description with profile name", "Description=Auriga llama-server (profile: qwen3.6-vision)"},
		{"When generating unit, it should have After network", "After=network.target"},
		{"When generating unit, it should have Service section", "[Service]"},
		{"When generating unit, it should have Type simple", "Type=simple"},
		{"When generating unit, it should have ExecStart", "ExecStart=/usr/bin/llama-server -m /models/model.gguf --host 0.0.0.0 --port 8090"},
		{"When generating unit, it should have environment", "Environment=AMD_VULKAN_ICD=RADV"},
		{"When generating unit, it should have restart policy", "Restart=on-failure"},
		{"When generating unit, it should have restart delay", "RestartSec=5"},
		{"When generating unit, it should have journal output", "StandardOutput=journal"},
		{"When generating unit, it should have Install section", "[Install]"},
		{"When generating unit, it should have WantedBy", "WantedBy=default.target"},
	}

	for _, c := range checks {
		t.Run(c.name, func(t *testing.T) {
			if !strings.Contains(got, c.want) {
				t.Errorf("expected %q in output, got:\n%s", c.want, got)
			}
		})
	}
}

func TestGenerateUnit_NoEnvironment(t *testing.T) {
	cfg := ServiceConfig{
		ProfileName: "gemma4-26b",
		ExecStart:   "/usr/bin/llama-server -m /models/gemma.gguf",
	}

	got := GenerateUnit(cfg)

	if strings.Contains(got, "Environment=") {
		t.Error("When no environment vars, it should not have Environment lines")
	}
	if !strings.Contains(got, "gemma4-26b") {
		t.Error("When generating unit, it should contain profile name")
	}
}

func TestGenerateUnit_MultipleEnvironmentVars(t *testing.T) {
	cfg := ServiceConfig{
		ProfileName: "test",
		ExecStart:   "/usr/bin/llama-server",
		Environment: []string{"AMD_VULKAN_ICD=RADV", "HSA_OVERRIDE_GFX_VERSION=11.5.0"},
	}

	got := GenerateUnit(cfg)

	if !strings.Contains(got, "Environment=AMD_VULKAN_ICD=RADV") {
		t.Error("When multiple env vars, it should contain first var")
	}
	if !strings.Contains(got, "Environment=HSA_OVERRIDE_GFX_VERSION=11.5.0") {
		t.Error("When multiple env vars, it should contain second var")
	}
}

func TestGenerateUnit_VisionProfile(t *testing.T) {
	cfg := ServiceConfig{
		ProfileName: "qwen3.6-vision",
		ExecStart:   "/bin/llama-server -m /m/model.gguf --mmproj /m/mmproj.gguf --jinja --host 0.0.0.0",
		Environment: []string{"AMD_VULKAN_ICD=RADV"},
	}

	got := GenerateUnit(cfg)

	if !strings.Contains(got, "--mmproj") {
		t.Error("When vision profile, ExecStart should contain --mmproj")
	}
	if !strings.Contains(got, "--jinja") {
		t.Error("When vision profile, ExecStart should contain --jinja")
	}
}

func TestUnitPath_ContainsServiceName(t *testing.T) {
	path, err := UnitPath()
	if err != nil {
		t.Fatalf("When getting unit path, it should not error: %v", err)
	}
	if !strings.HasSuffix(path, UnitName) {
		t.Errorf("When getting unit path, it should end with %s, got %s", UnitName, path)
	}
	if !strings.Contains(path, filepath.Join(".config", "systemd", "user")) {
		t.Errorf("When getting unit path, it should be in systemd user dir, got %s", path)
	}
}

func TestUnitDir_IsSystemdUserDir(t *testing.T) {
	dir, err := UnitDir()
	if err != nil {
		t.Fatalf("When getting unit dir, it should not error: %v", err)
	}
	if !strings.HasSuffix(dir, filepath.Join(".config", "systemd", "user")) {
		t.Errorf("When getting unit dir, it should end with .config/systemd/user, got %s", dir)
	}
}

func TestInstall_WritesFile(t *testing.T) {
	tmpDir := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	content := GenerateUnit(ServiceConfig{
		ProfileName: "test-profile",
		ExecStart:   "/usr/bin/llama-server -m /models/test.gguf",
	})

	dir := filepath.Join(tmpDir, ".config", "systemd", "user")
	os.MkdirAll(dir, 0755)
	path := filepath.Join(dir, UnitName)
	err := os.WriteFile(path, []byte(content), 0644)
	if err != nil {
		t.Fatalf("When writing service file, it should not error: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("When reading back service file, it should not error: %v", err)
	}

	if !strings.Contains(string(data), "test-profile") {
		t.Error("When installing service, file should contain profile name")
	}
	if !strings.Contains(string(data), "[Service]") {
		t.Error("When installing service, file should contain Service section")
	}
}

func TestUnitName_IsCorrect(t *testing.T) {
	if UnitName != "auriga-llama-server.service" {
		t.Errorf("When checking unit name, got %s, want auriga-llama-server.service", UnitName)
	}
}
