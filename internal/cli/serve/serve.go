package serve

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/jparrill/auriga-cli/internal/config"
	"github.com/jparrill/auriga-cli/internal/exec"
	"github.com/jparrill/auriga-cli/internal/ui"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func NewServeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Manage llama-server for local inference",
		Long: `Start llama-server with a named profile (model + optional mmproj for vision), or stop it.

Examples:
  auriga serve start qwen3.6-vision                    # Vision-enabled (with mmproj)
  auriga serve start qwen3.6-uncensored-vision         # Uncensored + vision
  auriga serve start --model Qwen3-30B-A3B-Q4_K_M.gguf # Custom GGUF, no vision
  auriga serve list                                    # Show configured profiles
  auriga serve stop                                    # Stop and restart Ollama`,
	}

	cmd.AddCommand(newServeStartCmd())
	cmd.AddCommand(newServeStopCmd())
	cmd.AddCommand(newServeListCmd())

	return cmd
}

type serveStartOpts struct {
	Profile string
	Model   string
	MMProj  string
	Port    int
	CtxSize int
	Vision  bool
}

func newServeStartCmd() *cobra.Command {
	opts := &serveStartOpts{}

	cmd := &cobra.Command{
		Use:   "start [profile]",
		Short: "Start llama-server with a profile or custom model",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				opts.Profile = args[0]
			}
			return runServeStart(opts)
		},
	}

	cmd.Flags().StringVar(&opts.Model, "model", "", "GGUF model filename (overrides profile)")
	cmd.Flags().StringVar(&opts.MMProj, "mmproj", "", "Multimodal projector filename (overrides profile)")
	cmd.Flags().IntVar(&opts.Port, "port", 0, "Server port (default from config)")
	cmd.Flags().IntVar(&opts.CtxSize, "ctx-size", 65536, "Context window size")
	cmd.Flags().BoolVar(&opts.Vision, "vision", false, "Enable vision (auto-detect mmproj from profile)")

	return cmd
}

func runServeStart(opts *serveStartOpts) error {
	ctx := context.Background()

	ggufDir := config.ExpandHome(viper.GetString("llama_server.gguf_dir"))
	mmprojDir := config.ExpandHome(viper.GetString("llama_server.mmproj_dir"))
	bin := config.ExpandHome(viper.GetString("llama_server.bin"))
	port := viper.GetInt("llama_server.port")
	if opts.Port > 0 {
		port = opts.Port
	}

	var modelPath, mmprojPath string

	if opts.Profile != "" {
		profileKey := fmt.Sprintf("profiles.%s", opts.Profile)
		modelFile := viper.GetString(profileKey + ".model")
		mmprojFile := viper.GetString(profileKey + ".mmproj")

		if modelFile == "" {
			return fmt.Errorf("profile %q not found in config (key: %s.model)", opts.Profile, profileKey)
		}

		modelPath = filepath.Join(ggufDir, modelFile)
		if mmprojFile != "" {
			mmprojPath = filepath.Join(mmprojDir, mmprojFile)
		}
	}

	if opts.Model != "" {
		modelPath = filepath.Join(ggufDir, opts.Model)
	}
	if opts.MMProj != "" {
		mmprojPath = filepath.Join(mmprojDir, opts.MMProj)
	}

	if modelPath == "" {
		return fmt.Errorf("no model specified — use a profile name or --model flag")
	}

	if _, err := os.Stat(modelPath); err != nil {
		return fmt.Errorf("model not found: %s", modelPath)
	}
	if mmprojPath != "" {
		if _, err := os.Stat(mmprojPath); err != nil {
			return fmt.Errorf("mmproj not found: %s", mmprojPath)
		}
	}

	params := []ui.OrderedParam{
		{Key: "Model", Value: filepath.Base(modelPath)},
		{Key: "Port", Value: fmt.Sprintf("%d", port)},
		{Key: "Context", Value: fmt.Sprintf("%d", opts.CtxSize)},
	}
	if mmprojPath != "" {
		params = append(params, ui.OrderedParam{Key: "Vision", Value: filepath.Base(mmprojPath)})
	}

	confirmed, err := ui.ConfirmOperationOrdered("Start llama-server", params, "", false)
	if err != nil || !confirmed {
		return err
	}

	ui.Info("Stopping Ollama to free GPU...")
	exec.RunCapture(ctx, "sudo", []string{"systemctl", "stop", "ollama"}, exec.RunOpts{})
	time.Sleep(2 * time.Second)

	args := []string{
		"-m", modelPath,
		"--host", "0.0.0.0",
		"--port", fmt.Sprintf("%d", port),
		"--flash-attn", "on",
		"--gpu-layers", "99",
		"--ctx-size", fmt.Sprintf("%d", opts.CtxSize),
	}

	if mmprojPath != "" {
		args = append(args, "--mmproj", mmprojPath, "--jinja")
	}

	extraFlags := viper.GetStringSlice(fmt.Sprintf("profiles.%s.flags", opts.Profile))
	args = append(args, extraFlags...)

	ui.Info(fmt.Sprintf("CMD: %s %s", bin, strings.Join(args, " ")))

	logFile, _ := os.Create("/tmp/llama-server-auriga.log")
	defer logFile.Close()

	cmd := fmt.Sprintf("%s %s", bin, strings.Join(args, " "))
	_ = cmd

	proc := &os.Process{}
	err = ui.WithSpinner("Starting llama-server...", func() error {
		var startErr error
		proc, startErr = startProcess(bin, args, logFile)
		if startErr != nil {
			return startErr
		}
		return waitForHealth(fmt.Sprintf("http://localhost:%d/health", port), 90*time.Second)
	})
	if err != nil {
		return fmt.Errorf("llama-server failed to start: %w", err)
	}

	ui.Ok(fmt.Sprintf("llama-server ready on port %d (PID %d)", port, proc.Pid))
	ui.Info("Press Ctrl+C to stop")

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	fmt.Println()
	ui.Info("Stopping llama-server...")
	proc.Kill()
	proc.Wait()
	time.Sleep(2 * time.Second)

	ui.Info("Restarting Ollama...")
	exec.RunCapture(context.Background(), "sudo", []string{"systemctl", "start", "ollama"}, exec.RunOpts{})
	ui.Ok("Ollama restarted")

	return nil
}

func startProcess(bin string, args []string, logFile *os.File) (*os.Process, error) {
	attr := &os.ProcAttr{
		Dir: "/tmp",
		Env: append(os.Environ(), "AMD_VULKAN_ICD=RADV"),
		Files: []*os.File{
			os.Stdin,
			logFile,
			logFile,
		},
	}
	argv := append([]string{bin}, args...)
	proc, err := os.StartProcess(bin, argv, attr)
	if err != nil {
		return nil, fmt.Errorf("failed to start %s: %w", bin, err)
	}
	return proc, nil
}

func waitForHealth(url string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		resp, err := http.Get(url)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == 200 {
				return nil
			}
		}
		time.Sleep(time.Second)
	}
	return fmt.Errorf("health check timeout after %s", timeout)
}
