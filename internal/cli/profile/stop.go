package profile

import (
	"context"
	"fmt"
	"os"
	"syscall"
	"time"

	"github.com/jparrill/auriga-cli/internal/exec"
	"github.com/jparrill/auriga-cli/internal/llamaserver"
	"github.com/jparrill/auriga-cli/internal/ui"
	"github.com/spf13/cobra"
)

func newProfileStopCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "stop",
		Short: "Stop llama-server and restart Ollama",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runProfileStop()
		},
	}
}

func runProfileStop() error {
	ctx := context.Background()
	stopped := false

	// Try PID file first
	if pid := readPID(); pid > 0 {
		if processExists(pid) {
			ui.Info(fmt.Sprintf("Stopping llama-server (PID %d)...", pid))
			proc, err := os.FindProcess(pid)
			if err == nil {
				proc.Signal(syscall.SIGTERM)
				time.Sleep(2 * time.Second)
				proc.Kill()
				stopped = true
			}
		}
		os.Remove(pidFile)
	}

	// Fallback: pkill
	if !stopped {
		ui.Info("Stopping llama-server via pkill...")
		out, err := exec.RunCapture(ctx, "pkill", []string{"-f", "llama-server"}, exec.RunOpts{})
		if err != nil {
			ui.Warn(fmt.Sprintf("pkill: %s", out))
		}
	}

	time.Sleep(2 * time.Second)
	llamaserver.StartOllama(ctx)

	return nil
}
