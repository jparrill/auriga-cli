package profile

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"

	"github.com/jparrill/auriga-cli/internal/config"
	"github.com/jparrill/auriga-cli/internal/llamaserver"
	"github.com/jparrill/auriga-cli/internal/ui"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

const pidFile = "/tmp/auriga-llama-server.pid"

func newProfileServeCmd() *cobra.Command {
	var daemon bool

	cmd := &cobra.Command{
		Use:   "serve <profile-name>",
		Short: "Start llama-server with a profile",
		Long: `Start llama-server with the model and optional mmproj from a configured profile.
If the profile has vision (mmproj), --jinja is added automatically.

Examples:
  auriga profile serve qwen3.6-vision            # Foreground (Ctrl+C to stop)
  auriga profile serve qwen3.6-vision --daemon    # Background (use 'auriga profile stop' to stop)
  auriga profile serve gemma4-12b-vision --daemon`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runProfileServe(args[0], daemon)
		},
	}

	cmd.Flags().BoolVarP(&daemon, "daemon", "d", false, "Run in background")

	return cmd
}

func runProfileServe(name string, daemon bool) error {
	profileKey := fmt.Sprintf("profiles.%s", name)
	modelFile := viper.GetString(profileKey + ".model")
	if modelFile == "" {
		return fmt.Errorf("profile %q not found — run: auriga profile list", name)
	}

	// Check if already running
	if existingPID := readPID(); existingPID > 0 {
		if processExists(existingPID) {
			return fmt.Errorf("llama-server already running (PID %d) — run: auriga profile stop", existingPID)
		}
		os.Remove(pidFile)
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

	params := []ui.OrderedParam{
		{Key: "Profile", Value: name},
		{Key: "Model", Value: modelFile},
		{Key: "Mode", Value: mode},
	}
	if mmprojFile != "" {
		params = append(params, ui.OrderedParam{Key: "Vision", Value: mmprojFile})
	}
	params = append(params, ui.OrderedParam{Key: "Port", Value: fmt.Sprintf("%d", llamaserver.Port())})

	confirmed, err := ui.ConfirmOperationOrdered("Start llama-server", params, "", false)
	if err != nil || !confirmed {
		return err
	}

	var extraFlags []string
	if mmprojFile != "" {
		extraFlags = append(extraFlags, "--jinja")
	}

	ctx := context.Background()
	proc, err := llamaserver.Start(ctx, modelPath, mmprojPath, extraFlags)
	if err != nil {
		return err
	}

	// Save PID
	os.WriteFile(pidFile, []byte(strconv.Itoa(proc.Pid)), 0644)

	if daemon {
		ui.Ok(fmt.Sprintf("llama-server running in background (PID %d)", proc.Pid))
		ui.Info("Stop with: auriga profile stop")
		// Release the process so it survives after CLI exits
		proc.Release()
		return nil
	}

	ui.Info("Press Ctrl+C to stop")
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	fmt.Println()
	os.Remove(pidFile)
	llamaserver.Stop(proc)

	return nil
}

func readPID() int {
	data, err := os.ReadFile(pidFile)
	if err != nil {
		return 0
	}
	pid, err := strconv.Atoi(string(data))
	if err != nil {
		return 0
	}
	return pid
}

func processExists(pid int) bool {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	err = proc.Signal(syscall.Signal(0))
	return err == nil
}
