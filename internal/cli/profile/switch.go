package profile

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/jparrill/auriga-cli/internal/config"
	"github.com/jparrill/auriga-cli/internal/exec"
	"github.com/jparrill/auriga-cli/internal/llamaserver"
	"github.com/jparrill/auriga-cli/internal/systemd"
	"github.com/jparrill/auriga-cli/internal/ui"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func newProfileSwitchCmd() *cobra.Command {
	var (
		persistent bool
		ctxSize    int
	)

	cmd := &cobra.Command{
		Use:   "switch <profile-name>",
		Short: "Switch to a different llama-server profile",
		Long: `Stop the running llama-server and start it with a new profile.
Files are verified before switching. Use --persistent to create a systemd
user service that survives reboots.

Examples:
  auriga profile switch qwen3.6-vision
  auriga profile switch gemma4-26b --persistent
  auriga profile switch qwen3-coder --ctx-size 131072 --persistent`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runProfileSwitch(args[0], persistent, ctxSize)
		},
	}

	cmd.Flags().BoolVar(&persistent, "persistent", false, "Create systemd user service for reboot persistence")
	cmd.Flags().IntVar(&ctxSize, "ctx-size", 65536, "Context window size for llama-server")

	return cmd
}

func runProfileSwitch(name string, persistent bool, ctxSize int) error {
	profileKey := fmt.Sprintf("profiles.%s", name)
	modelFile := viper.GetString(profileKey + ".model")
	if modelFile == "" {
		return fmt.Errorf("profile %q not found — run: auriga profile list", name)
	}

	mmprojFile := viper.GetString(profileKey + ".mmproj")
	ggufDir := config.ExpandHome(viper.GetString("llama_server.gguf_dir"))
	mmprojDir := config.ExpandHome(viper.GetString("llama_server.mmproj_dir"))

	modelPath := filepath.Join(ggufDir, modelFile)
	if _, err := os.Stat(modelPath); err != nil {
		return fmt.Errorf("model not found: %s\nRun: auriga profile sync --name %s", modelPath, name)
	}

	var mmprojPath string
	if mmprojFile != "" {
		mmprojPath = filepath.Join(mmprojDir, mmprojFile)
		if _, err := os.Stat(mmprojPath); err != nil {
			return fmt.Errorf("mmproj not found: %s\nRun: auriga profile sync --name %s", mmprojPath, name)
		}
	}

	repo := viper.GetString(profileKey + ".repo")
	if repo != "" {
		if !verifyFile(name, modelFile, modelPath, repo, modelFile) {
			return fmt.Errorf("model verification failed — run: auriga profile sync --name %s", name)
		}
		if mmprojFile != "" {
			originalName := repoFilename(mmprojFile, repo)
			if !verifyFile(name, mmprojFile, mmprojPath, repo, originalName) {
				return fmt.Errorf("mmproj verification failed — run: auriga profile sync --name %s", name)
			}
		}
	}

	port := profilePort(name)

	configuredType := viper.GetString(profileKey + ".type")
	warnTypeMismatch(name, configuredType, modelFile)

	mode := "daemon"
	if persistent {
		mode = "persistent (systemd)"
	}
	pType := profileType(name)
	params := []ui.OrderedParam{
		{Key: "Profile", Value: name},
		{Key: "Model", Value: modelFile},
		{Key: "Type", Value: pType},
		{Key: "Mode", Value: mode},
	}
	if mmprojFile != "" {
		params = append(params, ui.OrderedParam{Key: "Vision", Value: mmprojFile})
	}
	params = append(params, ui.OrderedParam{Key: "Port", Value: fmt.Sprintf("%d", port)})

	confirmed, err := ui.ConfirmOperationOrdered("Switch llama-server profile", params, "", false)
	if err != nil || !confirmed {
		return err
	}

	stopRunningServer(port)

	extraFlags := viper.GetStringSlice(profileKey + ".flags")
	if mmprojFile != "" && !containsFlag(extraFlags, "--jinja") {
		extraFlags = append(extraFlags, "--jinja")
	}

	if persistent {
		return switchPersistent(name, modelPath, mmprojPath, extraFlags, ctxSize, port)
	}
	return switchDaemon(name, modelPath, mmprojPath, extraFlags, ctxSize, port)
}

func switchDaemon(name, modelPath, mmprojPath string, extraFlags []string, ctxSize, port int) error {
	ctx := context.Background()
	proc, err := llamaserver.StartWithCtx(ctx, modelPath, mmprojPath, extraFlags, ctxSize, port)
	if err != nil {
		return err
	}

	os.WriteFile(pidFileForPort(port), fmt.Appendf(nil, "%d", proc.Pid), 0644)
	proc.Release()

	ui.Ok(fmt.Sprintf("Switched to %s (PID %d) on port %d", name, proc.Pid, port))
	ui.Info("Stop with: auriga profile stop")
	return nil
}

func switchPersistent(name, modelPath, mmprojPath string, extraFlags []string, ctxSize, port int) error {
	execStart := buildExecStart(modelPath, mmprojPath, extraFlags, ctxSize, port)

	cfg := systemd.ServiceConfig{
		ProfileName: name,
		ExecStart:   execStart,
		Environment: []string{"AMD_VULKAN_ICD=RADV"},
	}

	content := systemd.GenerateUnit(cfg)
	unitName := systemd.UnitNameForPort(port)

	if err := systemd.Install(port, content); err != nil {
		return fmt.Errorf("failed to install service: %w", err)
	}

	if err := systemd.Enable(port); err != nil {
		return fmt.Errorf("failed to enable service: %w", err)
	}

	if err := systemd.Start(port); err != nil {
		return fmt.Errorf("failed to start service: %w", err)
	}

	if err := systemd.EnableLinger(); err != nil {
		ui.Warn(fmt.Sprintf("Could not enable linger: %v", err))
		ui.Info("Service may not survive logout — run: loginctl enable-linger")
	}

	if err := llamaserver.WaitForHealthOnPort(port, 90*time.Second); err != nil {
		return fmt.Errorf("service started but health check failed: %w", err)
	}

	path, _ := systemd.UnitPathForPort(port)
	ui.Ok(fmt.Sprintf("Switched to %s (systemd persistent) on port %d", name, port))
	ui.Info(fmt.Sprintf("Service: %s", path))
	ui.Info(fmt.Sprintf("Stop with: systemctl --user stop %s", unitName))
	ui.Info(fmt.Sprintf("Logs with: journalctl --user -u %s -f", unitName))
	return nil
}

func buildExecStart(modelPath, mmprojPath string, extraFlags []string, ctxSize, port int) string {
	bin := llamaserver.Bin()

	args := []string{
		bin,
		"-m", modelPath,
		"--host", "0.0.0.0",
		"--port", fmt.Sprintf("%d", port),
		"--flash-attn", "on",
		"--gpu-layers", "99",
		"--ctx-size", fmt.Sprintf("%d", ctxSize),
	}

	if mmprojPath != "" {
		args = append(args, "--mmproj", mmprojPath, "--jinja")
	}
	args = append(args, extraFlags...)

	return strings.Join(args, " ")
}

func stopRunningServer(port int) {
	if systemd.IsActiveOnPort(port) {
		ui.Info(fmt.Sprintf("Stopping systemd-managed llama-server on port %d...", port))
		systemd.Stop(port)
		time.Sleep(2 * time.Second)
		return
	}

	pf := pidFileForPort(port)
	if pid := readPIDForPort(port); pid > 0 {
		if processExists(pid) {
			ui.Info(fmt.Sprintf("Stopping llama-server (PID %d) on port %d...", pid, port))
			proc, err := os.FindProcess(pid)
			if err == nil {
				proc.Signal(syscall.SIGTERM)
				time.Sleep(2 * time.Second)
				proc.Kill()
			}
		}
		os.Remove(pf)
		return
	}

	ctx := context.Background()
	exec.RunCapture(ctx, "pkill", []string{"-f", fmt.Sprintf("llama-server.*--port %d", port)}, exec.RunOpts{})
	time.Sleep(1 * time.Second)
}
