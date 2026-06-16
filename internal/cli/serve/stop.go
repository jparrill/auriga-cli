package serve

import (
	"context"
	"fmt"
	"time"

	"github.com/jparrill/auriga-cli/internal/exec"
	"github.com/jparrill/auriga-cli/internal/ui"
	"github.com/spf13/cobra"
)

func newServeStopCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "stop",
		Short: "Stop llama-server and restart Ollama",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runServeStop()
		},
	}
}

func runServeStop() error {
	ctx := context.Background()

	ui.Info("Stopping llama-server...")
	out, err := exec.RunCapture(ctx, "pkill", []string{"-f", "llama-server"}, exec.RunOpts{})
	if err != nil {
		ui.Warn(fmt.Sprintf("pkill: %s", out))
	}

	time.Sleep(2 * time.Second)

	ui.Info("Restarting Ollama...")
	_, err = exec.RunCapture(ctx, "sudo", []string{"systemctl", "start", "ollama"}, exec.RunOpts{})
	if err != nil {
		return fmt.Errorf("failed to restart Ollama: %w", err)
	}

	ui.Ok("Ollama restarted")
	return nil
}
