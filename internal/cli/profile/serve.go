package profile

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"regexp"
	"strconv"
	"syscall"
	"time"

	"github.com/jparrill/auriga-cli/internal/config"
	"github.com/jparrill/auriga-cli/internal/llamaserver"
	"github.com/jparrill/auriga-cli/internal/ui"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var moePattern = regexp.MustCompile(`-A\d+B`)

func pidFileForPort(port int) string {
	return fmt.Sprintf("/tmp/auriga-llama-server-%d.pid", port)
}

func readPIDForPort(port int) int {
	data, err := os.ReadFile(pidFileForPort(port))
	if err != nil {
		return 0
	}
	pid, err := strconv.Atoi(string(data))
	if err != nil {
		return 0
	}
	return pid
}

func profilePort(name string) int {
	profileKey := fmt.Sprintf("profiles.%s", name)
	if p := viper.GetInt(profileKey + ".port"); p > 0 {
		return p
	}
	t := profileType(name)
	if t == "moe" {
		return llamaserver.MoePort()
	}
	return llamaserver.DensePort()
}

func profileType(name string) string {
	profileKey := fmt.Sprintf("profiles.%s", name)
	if t := viper.GetString(profileKey + ".type"); t != "" {
		return t
	}
	return detectModelType(viper.GetString(profileKey + ".model"))
}

func detectModelType(modelName string) string {
	if moePattern.MatchString(modelName) {
		return "moe"
	}
	return "dense"
}

func warnTypeMismatch(name, configuredType, modelName string) {
	detected := detectModelType(modelName)
	if configuredType != "" && configuredType != detected {
		ui.Warn(fmt.Sprintf("Profile %q has type=%s but model name suggests %s", name, configuredType, detected))
	}
}

func newProfileServeCmd() *cobra.Command {
	var (
		daemon bool
		ctxSize int
	)

	cmd := &cobra.Command{
		Use:   "serve <profile-name>",
		Short: "Start llama-server with a profile",
		Long: `Start llama-server with the model and optional mmproj from a configured profile.
If the profile has vision (mmproj), --jinja is added automatically.

Examples:
  auriga profile serve qwen3.6-vision            # Foreground (Ctrl+C to stop)
  auriga profile serve qwen3.6-vision --daemon    # Background (use 'auriga profile stop' to stop)
  auriga profile serve gemma4-12b-vision --ctx-size 131072`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runProfileServe(args[0], daemon, ctxSize)
		},
	}

	cmd.Flags().BoolVarP(&daemon, "daemon", "d", false, "Run in background")
	cmd.Flags().IntVar(&ctxSize, "ctx-size", 65536, "Context window size for llama-server")

	return cmd
}

func runProfileServe(name string, daemon bool, ctxSize int) error {
	profileKey := fmt.Sprintf("profiles.%s", name)
	modelFile := viper.GetString(profileKey + ".model")
	if modelFile == "" {
		return fmt.Errorf("profile %q not found — run: auriga profile list", name)
	}

	port := profilePort(name)
	pf := pidFileForPort(port)

	configuredType := viper.GetString(profileKey + ".type")
	warnTypeMismatch(name, configuredType, modelFile)

	if existingPID := readPIDForPort(port); existingPID > 0 {
		if processExists(existingPID) {
			return fmt.Errorf("llama-server already running on port %d (PID %d) — run: auriga profile stop", port, existingPID)
		}
		os.Remove(pf)
	}

	if portInUse(port) {
		return fmt.Errorf("port %d already in use — another process may be running\nRun: auriga profile stop", port)
	}

	mmprojFile := viper.GetString(profileKey + ".mmproj")
	ggufDir := config.ExpandHome(viper.GetString("llama_server.gguf_dir"))
	mmprojDir := config.ExpandHome(viper.GetString("llama_server.mmproj_dir"))

	modelPath := filepath.Join(ggufDir, modelFile)
	if _, err := os.Stat(modelPath); err != nil {
		return fmt.Errorf("model not found: %s\nRun: auriga model ensure --profile %s", modelPath, name)
	}

	var mmprojPath string
	if mmprojFile != "" {
		mmprojPath = filepath.Join(mmprojDir, mmprojFile)
		if _, err := os.Stat(mmprojPath); err != nil {
			return fmt.Errorf("mmproj not found: %s\nRun: auriga model ensure --profile %s", mmprojPath, name)
		}
	}

	mode := "foreground"
	if daemon {
		mode = "daemon"
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

	confirmed, err := ui.ConfirmOperationOrdered("Start llama-server", params, "", false)
	if err != nil || !confirmed {
		return err
	}

	extraFlags := viper.GetStringSlice(profileKey + ".flags")
	if mmprojFile != "" && !containsFlag(extraFlags, "--jinja") {
		extraFlags = append(extraFlags, "--jinja")
	}

	ctx := context.Background()
	proc, err := llamaserver.StartWithCtx(ctx, modelPath, mmprojPath, extraFlags, ctxSize, port)
	if err != nil {
		return err
	}

	os.WriteFile(pf, []byte(strconv.Itoa(proc.Pid)), 0644)

	if daemon {
		ui.Ok(fmt.Sprintf("llama-server running in background (PID %d) on port %d", proc.Pid, port))
		ui.Info("Stop with: auriga profile stop")
		printHermesTip(modelFile, pType, port)
		proc.Release()
		return nil
	}

	ui.Info("Press Ctrl+C to stop")
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	fmt.Println()
	os.Remove(pf)
	llamaserver.Stop(proc)

	return nil
}

func containsFlag(flags []string, target string) bool {
	for _, f := range flags {
		if f == target {
			return true
		}
	}
	return false
}

func portInUse(port int) bool {
	host := llamaserver.HostForPort(port)
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(host + "/health")
	if err != nil {
		return false
	}
	resp.Body.Close()
	return true
}

func processExists(pid int) bool {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	err = proc.Signal(syscall.Signal(0))
	return err == nil
}

func printHermesTip(modelFile, modelType string, port int) {
	moeProfile := viper.GetString("hermes.moe_profile")
	denseProfile := viper.GetString("hermes.dense_profile")

	hermesProfile := moeProfile
	if modelType == "dense" {
		hermesProfile = denseProfile
	}
	if hermesProfile == "" {
		return
	}

	fmt.Println()
	ui.Info(fmt.Sprintf("Hermes: update %q profile for this %s model", hermesProfile, modelType))
	fmt.Printf("  hermes profile use %s\n", hermesProfile)
	fmt.Printf("  hermes config set model.base_url http://localhost:%d/v1\n", port)
	fmt.Printf("  hermes config set model.default %s\n", modelFile)

	if modelType == "dense" && moeProfile != "" {
		fmt.Printf("\n  # Also update fallback in %q profile:\n", moeProfile)
		fmt.Printf("  hermes profile use %s\n", moeProfile)
		fmt.Printf("  hermes config set fallback_providers.0.model %s\n", modelFile)
		fmt.Printf("  systemctl --user restart hermes-gateway.service\n")
		fmt.Printf("\n  # Create %q profile first if it doesn't exist:\n", denseProfile)
		fmt.Printf("  hermes profile create %s --clone-from %s --description \"Deep planning with dense models\"\n", denseProfile, moeProfile)
	}
}
