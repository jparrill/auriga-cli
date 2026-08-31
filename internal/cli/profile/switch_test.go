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
	if flag.DefValue != "131072" {
		t.Errorf("When checking ctx-size default, got %q, want 131072", flag.DefValue)
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

	err := RunProfileSwitch("nonexistent", false, 131072, false)

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

	err := RunProfileSwitch("test-profile", false, 131072, false)

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

	err := RunProfileSwitch("test-profile", false, 131072, false)

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

	got := buildExecStart("/usr/bin/llama-server", "/models/model.gguf", "", nil, 131072, 8090)

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
		{"When building ExecStart, it should have ctx-size", "--ctx-size 131072"},
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

	got := buildExecStart("/usr/bin/llama-server", "/models/model.gguf", "/models/mmproj.gguf", []string{"--jinja"}, 131072, 8090)

	if !strings.Contains(got, "--mmproj /models/mmproj.gguf") {
		t.Errorf("When mmproj set, ExecStart should contain --mmproj, got: %s", got)
	}
	if !strings.Contains(got, "--jinja") {
		t.Errorf("When mmproj set, ExecStart should contain --jinja via extraFlags, got: %s", got)
	}
	if strings.Count(got, "--jinja") > 1 {
		t.Errorf("When mmproj set, --jinja should appear only once, got: %s", got)
	}
}

func TestBuildExecStart_WithExtraFlags(t *testing.T) {
	viper.Reset()
	defer viper.Reset()

	viper.Set("llama_server.bin", "/usr/bin/llama-server")

	got := buildExecStart("/usr/bin/llama-server", "/models/model.gguf", "", []string{"--threads", "16"}, 131072, 8090)

	if !strings.Contains(got, "--threads 16") {
		t.Errorf("When extra flags set, ExecStart should contain them, got: %s", got)
	}
}

func TestBuildExecStart_CustomCtxSize(t *testing.T) {
	viper.Reset()
	defer viper.Reset()

	viper.Set("llama_server.bin", "/usr/bin/llama-server")

	got := buildExecStart("/usr/bin/llama-server", "/models/model.gguf", "", nil, 131072, 8090)

	if !strings.Contains(got, "--ctx-size 131072") {
		t.Errorf("When custom ctx-size, ExecStart should use it, got: %s", got)
	}
}

func TestBuildExecStart_NoMmprojNoJinja(t *testing.T) {
	viper.Reset()
	defer viper.Reset()

	viper.Set("llama_server.bin", "/usr/bin/llama-server")

	got := buildExecStart("/usr/bin/llama-server", "/models/model.gguf", "", nil, 131072, 8090)

	if strings.Contains(got, "--mmproj") {
		t.Errorf("When no mmproj, ExecStart should not have --mmproj, got: %s", got)
	}
	if strings.Contains(got, "--jinja") {
		t.Errorf("When no mmproj, ExecStart should not have --jinja, got: %s", got)
	}
}

func TestBuildExecStart_MoePort(t *testing.T) {
	viper.Reset()
	defer viper.Reset()

	viper.Set("llama_server.bin", "/usr/bin/llama-server")

	got := buildExecStart("/usr/bin/llama-server", "/models/moe-model.gguf", "", nil, 131072, 8091)

	if !strings.Contains(got, "--port 8091") {
		t.Errorf("When MoE port, ExecStart should use port 8091, got: %s", got)
	}
}

func TestBuildExecStart_CustomPort(t *testing.T) {
	viper.Reset()
	defer viper.Reset()

	viper.Set("llama_server.bin", "/usr/bin/llama-server")

	got := buildExecStart("/usr/bin/llama-server", "/models/model.gguf", "", nil, 131072, 9000)

	if !strings.Contains(got, "--port 9000") {
		t.Errorf("When custom port override, ExecStart should use it, got: %s", got)
	}
}

func TestBuildExecStart_CustomBin(t *testing.T) {
	viper.Reset()
	defer viper.Reset()

	got := buildExecStart("/custom/llama-server-strix-halo", "/models/model.gguf", "", nil, 131072, 8090)

	if !strings.Contains(got, "/custom/llama-server-strix-halo") {
		t.Errorf("When custom bin, ExecStart should use custom path, got: %s", got)
	}
	if strings.Contains(got, "llama-server ") && !strings.Contains(got, "strix-halo") {
		t.Errorf("When custom bin, should not use default llama-server, got: %s", got)
	}
}

func TestRunProfileSwitch_BinaryNotFound(t *testing.T) {
	viper.Reset()
	defer viper.Reset()

	tmpDir := t.TempDir()
	modelFile := filepath.Join(tmpDir, "model.gguf")
	os.WriteFile(modelFile, []byte("fake model"), 0644)

	viper.Set("profiles.test-profile.model", "model.gguf")
	viper.Set("profiles.test-profile.bin", "/nonexistent/llama-server-custom")
	viper.Set("llama_server.gguf_dir", tmpDir)
	viper.Set("llama_server.bin", "/also-nonexistent/llama-server")

	err := RunProfileSwitch("test-profile", false, 131072, false)

	if err == nil {
		t.Error("When profile binary not found, it should return error")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("When binary not found, error should mention 'not found', got: %v", err)
	}
}

func TestRunProfileServe_BinaryNotFound(t *testing.T) {
	viper.Reset()
	defer viper.Reset()

	tmpDir := t.TempDir()
	modelFile := filepath.Join(tmpDir, "model.gguf")
	os.WriteFile(modelFile, []byte("fake model"), 0644)

	viper.Set("profiles.test-profile.model", "model.gguf")
	viper.Set("profiles.test-profile.bin", "/nonexistent/llama-server-custom")
	viper.Set("llama_server.gguf_dir", tmpDir)
	viper.Set("llama_server.bin", "/also-nonexistent/llama-server")
	viper.Set("llama_server.host", "http://localhost:8090")
	viper.Set("llama_server.dense_port", 8090)

	err := runProfileServe("test-profile", false, 131072)

	if err == nil {
		t.Error("When profile binary not found, serve should return error")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("When binary not found, error should mention 'not found', got: %v", err)
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

func TestDetectModelType(t *testing.T) {
	tests := []struct {
		name      string
		modelName string
		want      string
	}{
		{"When model has A3B suffix, it should detect MoE", "Qwen3.6-35B-A3B-Q8_0.gguf", "moe"},
		{"When model has A4B suffix, it should detect MoE", "gemma-4-26B-A4B-it-UD-Q4_K_M.gguf", "moe"},
		{"When model is dense, it should detect dense", "Qwen3.6-27B-UD-Q8_K_XL.gguf", "dense"},
		{"When model is DeepSeek distill, it should detect dense", "DeepSeek-R1-Distill-Qwen-32B-Q8_0.gguf", "dense"},
		{"When model has A10B, it should detect MoE", "SomeModel-A10B-Q4.gguf", "moe"},
		{"When model name is empty, it should default dense", "", "dense"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := detectModelType(tt.modelName)
			if got != tt.want {
				t.Errorf("detectModelType(%q) = %q, want %q", tt.modelName, got, tt.want)
			}
		})
	}
}

func TestProfilePort_DenseDefault(t *testing.T) {
	viper.Reset()
	defer viper.Reset()

	viper.Set("profiles.test-dense.model", "Qwen3.6-27B-UD-Q8_K_XL.gguf")
	viper.Set("llama_server.dense_port", 8090)
	viper.Set("llama_server.host", "http://localhost:8090")

	port := profilePort("test-dense")
	if port != 8090 {
		t.Errorf("When dense profile, port should be 8090, got %d", port)
	}
}

func TestProfilePort_MoeDefault(t *testing.T) {
	viper.Reset()
	defer viper.Reset()

	viper.Set("profiles.test-moe.model", "Qwen3.6-35B-A3B-Q8_0.gguf")
	viper.Set("llama_server.moe_port", 8091)

	port := profilePort("test-moe")
	if port != 8091 {
		t.Errorf("When MoE profile, port should be 8091, got %d", port)
	}
}

func TestProfilePort_ExplicitType(t *testing.T) {
	viper.Reset()
	defer viper.Reset()

	viper.Set("profiles.override.model", "Qwen3.6-27B-UD-Q8_K_XL.gguf")
	viper.Set("profiles.override.type", "moe")
	viper.Set("llama_server.moe_port", 8091)

	port := profilePort("override")
	if port != 8091 {
		t.Errorf("When type explicitly set to moe, port should be 8091, got %d", port)
	}
}

func TestProfilePort_ExplicitPortOverride(t *testing.T) {
	viper.Reset()
	defer viper.Reset()

	viper.Set("profiles.custom.model", "model.gguf")
	viper.Set("profiles.custom.port", 9000)

	port := profilePort("custom")
	if port != 9000 {
		t.Errorf("When port explicitly set, it should override, got %d", port)
	}
}

func TestProfileType_ExplicitOverridesDetection(t *testing.T) {
	viper.Reset()
	defer viper.Reset()

	viper.Set("profiles.test.model", "Qwen3.6-27B-UD-Q8_K_XL.gguf")
	viper.Set("profiles.test.type", "moe")

	pType := profileType("test")
	if pType != "moe" {
		t.Errorf("When type explicitly set, it should override detection, got %q", pType)
	}
}

func TestProfileType_AutoDetection(t *testing.T) {
	viper.Reset()
	defer viper.Reset()

	viper.Set("profiles.auto.model", "gemma-4-26B-A4B-it-UD-Q4_K_M.gguf")

	pType := profileType("auto")
	if pType != "moe" {
		t.Errorf("When type not set and model is MoE, it should auto-detect moe, got %q", pType)
	}
}

func TestPidFileForPort(t *testing.T) {
	tests := []struct {
		name string
		port int
		want string
	}{
		{"When dense port, PID file uses port", 8090, "/tmp/auriga-llama-server-8090.pid"},
		{"When MoE port, PID file uses port", 8091, "/tmp/auriga-llama-server-8091.pid"},
		{"When custom port, PID file uses port", 9000, "/tmp/auriga-llama-server-9000.pid"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := pidFileForPort(tt.port)
			if got != tt.want {
				t.Errorf("pidFileForPort(%d) = %q, want %q", tt.port, got, tt.want)
			}
		})
	}
}

func TestReadPIDForPort_NoFile(t *testing.T) {
	pid := readPIDForPort(59999)
	if pid != 0 {
		t.Errorf("When no PID file, readPIDForPort should return 0, got %d", pid)
	}
}

func TestReadPIDForPort_ValidFile(t *testing.T) {
	os.WriteFile("/tmp/auriga-llama-server-59998.pid", []byte("12345"), 0644)
	defer os.Remove("/tmp/auriga-llama-server-59998.pid")

	pid := readPIDForPort(59998)
	if pid != 12345 {
		t.Errorf("When PID file has valid PID, should return it, got %d", pid)
	}
}

func TestReadPIDForPort_InvalidContent(t *testing.T) {
	os.WriteFile("/tmp/auriga-llama-server-59997.pid", []byte("not-a-number"), 0644)
	defer os.Remove("/tmp/auriga-llama-server-59997.pid")

	pid := readPIDForPort(59997)
	if pid != 0 {
		t.Errorf("When PID file has invalid content, should return 0, got %d", pid)
	}
}

func TestAllProfilePorts_IncludesDefaults(t *testing.T) {
	viper.Reset()
	defer viper.Reset()

	viper.Set("llama_server.dense_port", 8090)
	viper.Set("llama_server.moe_port", 8091)
	viper.Set("llama_server.host", "http://localhost:8090")

	ports := allProfilePorts()

	hasDense := false
	hasMoe := false
	for _, p := range ports {
		if p == 8090 {
			hasDense = true
		}
		if p == 8091 {
			hasMoe = true
		}
	}
	if !hasDense {
		t.Error("When listing all ports, it should include dense port")
	}
	if !hasMoe {
		t.Error("When listing all ports, it should include MoE port")
	}
}

func TestAllProfilePorts_IncludesCustomPorts(t *testing.T) {
	viper.Reset()
	defer viper.Reset()

	viper.Set("llama_server.dense_port", 8090)
	viper.Set("llama_server.moe_port", 8091)
	viper.Set("llama_server.host", "http://localhost:8090")
	viper.Set("profiles.custom.model", "model.gguf")
	viper.Set("profiles.custom.port", 9000)

	ports := allProfilePorts()

	hasCustom := false
	for _, p := range ports {
		if p == 9000 {
			hasCustom = true
		}
	}
	if !hasCustom {
		t.Error("When profile has custom port, allProfilePorts should include it")
	}
}

func TestNewProfileStopCmd_AcceptsOptionalArg(t *testing.T) {
	cmd := newProfileStopCmd()

	if cmd.Use != "stop [profile-name]" {
		t.Errorf("When creating stop command, Use should be 'stop [profile-name]', got %q", cmd.Use)
	}
}

func TestWarnTypeMismatch_NoWarningWhenMatch(t *testing.T) {
	warnTypeMismatch("test", "moe", "Qwen3.6-35B-A3B-Q8_0.gguf")
}

func TestWarnTypeMismatch_NoWarningWhenTypeEmpty(t *testing.T) {
	warnTypeMismatch("test", "", "Qwen3.6-35B-A3B-Q8_0.gguf")
}

func TestProfileCtxSize_Default(t *testing.T) {
	viper.Reset()
	defer viper.Reset()

	viper.Set("profiles.test.model", "model.gguf")

	got := profileCtxSize("test")
	if got != 131072 {
		t.Errorf("When no ctx_size configured, should return 131072, got %d", got)
	}
}

func TestProfileCtxSize_GlobalOverride(t *testing.T) {
	viper.Reset()
	defer viper.Reset()

	viper.Set("profiles.test.model", "model.gguf")
	viper.Set("llama_server.ctx_size", 65536)

	got := profileCtxSize("test")
	if got != 65536 {
		t.Errorf("When global ctx_size set, should use it, got %d", got)
	}
}

func TestProfileCtxSize_ProfileOverride(t *testing.T) {
	viper.Reset()
	defer viper.Reset()

	viper.Set("profiles.test.model", "model.gguf")
	viper.Set("profiles.test.ctx_size", 32768)
	viper.Set("llama_server.ctx_size", 65536)

	got := profileCtxSize("test")
	if got != 32768 {
		t.Errorf("When profile ctx_size set, should override global, got %d", got)
	}
}

func TestPrintHermesTip_MoeUsesLocalProfile(t *testing.T) {
	viper.Reset()
	defer viper.Reset()

	viper.Set("hermes.moe_profile", "local")
	viper.Set("hermes.dense_profile", "planning")

	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	printHermesTip("Qwen3.6-35B-A3B-Q8_0.gguf", "moe", 8091)

	w.Close()
	os.Stdout = old
	buf := make([]byte, 4096)
	n, _ := r.Read(buf)
	output := string(buf[:n])

	if !strings.Contains(output, "local") {
		t.Error("When MoE model, tip should reference local profile")
	}
	if !strings.Contains(output, "8091") {
		t.Error("When MoE model, tip should show MoE port")
	}
}

func TestPrintHermesTip_DenseUsesPlanningProfile(t *testing.T) {
	viper.Reset()
	defer viper.Reset()

	viper.Set("hermes.moe_profile", "local")
	viper.Set("hermes.dense_profile", "planning")

	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	printHermesTip("Qwen3.6-27B-UD-Q8_K_XL.gguf", "dense", 8090)

	w.Close()
	os.Stdout = old
	buf := make([]byte, 4096)
	n, _ := r.Read(buf)
	output := string(buf[:n])

	if !strings.Contains(output, "planning") {
		t.Error("When dense model, tip should reference planning profile")
	}
	if !strings.Contains(output, "hermes profile create planning") {
		t.Error("When dense model, tip should show create command for planning profile")
	}
	if !strings.Contains(output, "fallback") {
		t.Error("When dense model, tip should mention updating fallback in moe profile")
	}
	if !strings.Contains(output, "hermes gateway restart") {
		t.Error("When dense model, tip should remind to restart gateway")
	}
}

func TestPrintHermesTip_NoOutputWhenUnconfigured(t *testing.T) {
	viper.Reset()
	defer viper.Reset()

	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	printHermesTip("model.gguf", "moe", 8091)

	w.Close()
	os.Stdout = old
	buf := make([]byte, 4096)
	n, _ := r.Read(buf)
	output := string(buf[:n])

	if strings.Contains(output, "hermes") {
		t.Error("When hermes profiles not configured, should not print tip")
	}
}

func TestInjectDrafterFlags_MTPDrafter(t *testing.T) {
	viper.Reset()
	defer viper.Reset()

	ggufDir := t.TempDir()
	drafterPath := filepath.Join(ggufDir, "drafter.gguf")
	os.WriteFile(drafterPath, []byte("drafter"), 0644)

	viper.Set("profiles.test.mtp_drafter", "drafter.gguf")

	flags := []string{"--spec-type", "draft-mtp"}
	result := injectDrafterFlags("test", ggufDir, flags)

	if !containsFlag(result, "--model-draft") {
		t.Errorf("When mtp_drafter on disk, should inject --model-draft, got %v", result)
	}
	if result[len(result)-1] != drafterPath {
		t.Errorf("--model-draft should point to full path, got %v", result)
	}
}

func TestInjectDrafterFlags_DFlash(t *testing.T) {
	viper.Reset()
	defer viper.Reset()

	ggufDir := t.TempDir()
	dflashPath := filepath.Join(ggufDir, "dflash.gguf")
	os.WriteFile(dflashPath, []byte("dflash"), 0644)

	viper.Set("profiles.test.dflash", "dflash.gguf")

	flags := []string{"--jinja"}
	result := injectDrafterFlags("test", ggufDir, flags)

	if !containsFlag(result, "--model-draft") {
		t.Errorf("When dflash on disk, should inject --model-draft, got %v", result)
	}
}

func TestInjectDrafterFlags_AlreadyPresent(t *testing.T) {
	viper.Reset()
	defer viper.Reset()

	ggufDir := t.TempDir()
	os.WriteFile(filepath.Join(ggufDir, "drafter.gguf"), []byte("drafter"), 0644)

	viper.Set("profiles.test.mtp_drafter", "drafter.gguf")

	flags := []string{"--model-draft", "/custom/path.gguf"}
	result := injectDrafterFlags("test", ggufDir, flags)

	count := 0
	for _, f := range result {
		if f == "--model-draft" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("When --model-draft already in flags, should not duplicate, got %d occurrences", count)
	}
}

func TestInjectDrafterFlags_DrafterMissing(t *testing.T) {
	viper.Reset()
	defer viper.Reset()

	ggufDir := t.TempDir()
	viper.Set("profiles.test.mtp_drafter", "missing.gguf")

	flags := []string{"--spec-type", "draft-mtp"}
	result := injectDrafterFlags("test", ggufDir, flags)

	if containsFlag(result, "--model-draft") {
		t.Errorf("When drafter not on disk, should not inject --model-draft, got %v", result)
	}
}

func TestRunProfileSwitch_DFlashFileMissing(t *testing.T) {
	viper.Reset()
	defer viper.Reset()

	tmpDir := t.TempDir()
	modelFile := filepath.Join(tmpDir, "model.gguf")
	os.WriteFile(modelFile, []byte("fake model"), 0644)

	viper.Set("profiles.test-profile.model", "model.gguf")
	viper.Set("profiles.test-profile.dflash", "dflash-missing.gguf")
	viper.Set("llama_server.gguf_dir", tmpDir)

	err := RunProfileSwitch("test-profile", false, 131072, false)

	if err == nil {
		t.Error("When dflash file missing, it should return error")
	}
	if !strings.Contains(err.Error(), "dflash drafter not found") {
		t.Errorf("When dflash missing, error should mention dflash, got: %v", err)
	}
}

func TestRunProfileSwitch_MTPDrafterFileMissing(t *testing.T) {
	viper.Reset()
	defer viper.Reset()

	tmpDir := t.TempDir()
	modelFile := filepath.Join(tmpDir, "model.gguf")
	os.WriteFile(modelFile, []byte("fake model"), 0644)

	viper.Set("profiles.test-profile.model", "model.gguf")
	viper.Set("profiles.test-profile.mtp_drafter", "mtp-missing.gguf")
	viper.Set("llama_server.gguf_dir", tmpDir)

	err := RunProfileSwitch("test-profile", false, 131072, false)

	if err == nil {
		t.Error("When mtp_drafter file missing, it should return error")
	}
	if !strings.Contains(err.Error(), "mtp_drafter not found") {
		t.Errorf("When mtp_drafter missing, error should mention mtp_drafter, got: %v", err)
	}
}

func TestRunProfileServe_DFlashFileMissing(t *testing.T) {
	viper.Reset()
	defer viper.Reset()

	tmpDir := t.TempDir()
	modelFile := filepath.Join(tmpDir, "model.gguf")
	os.WriteFile(modelFile, []byte("fake model"), 0644)

	viper.Set("profiles.test-profile.model", "model.gguf")
	viper.Set("profiles.test-profile.dflash", "dflash-missing.gguf")
	viper.Set("llama_server.gguf_dir", tmpDir)
	viper.Set("llama_server.host", "http://localhost:8090")
	viper.Set("llama_server.dense_port", 8090)

	err := runProfileServe("test-profile", false, 131072)

	if err == nil {
		t.Error("When dflash file missing, serve should return error")
	}
	if !strings.Contains(err.Error(), "dflash drafter not found") {
		t.Errorf("When dflash missing, error should mention dflash, got: %v", err)
	}
}

func TestRunProfileServe_MTPDrafterFileMissing(t *testing.T) {
	viper.Reset()
	defer viper.Reset()

	tmpDir := t.TempDir()
	modelFile := filepath.Join(tmpDir, "model.gguf")
	os.WriteFile(modelFile, []byte("fake model"), 0644)

	viper.Set("profiles.test-profile.model", "model.gguf")
	viper.Set("profiles.test-profile.mtp_drafter", "mtp-missing.gguf")
	viper.Set("llama_server.gguf_dir", tmpDir)
	viper.Set("llama_server.host", "http://localhost:8090")
	viper.Set("llama_server.dense_port", 8090)

	err := runProfileServe("test-profile", false, 131072)

	if err == nil {
		t.Error("When mtp_drafter file missing, serve should return error")
	}
	if !strings.Contains(err.Error(), "mtp_drafter not found") {
		t.Errorf("When mtp_drafter missing, error should mention mtp_drafter, got: %v", err)
	}
}

func TestInjectDrafterFlags_NoDrafter(t *testing.T) {
	viper.Reset()
	defer viper.Reset()

	flags := []string{"--jinja"}
	result := injectDrafterFlags("test", t.TempDir(), flags)

	if len(result) != 1 || result[0] != "--jinja" {
		t.Errorf("When no drafter configured, flags should be unchanged, got %v", result)
	}
}
