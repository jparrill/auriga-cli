package profile

import (
	"context"
	"fmt"
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

	ui.Info("Stopping llama-server...")
	out, err := exec.RunCapture(ctx, "pkill", []string{"-f", "llama-server"}, exec.RunOpts{})
	if err != nil {
		ui.Warn(fmt.Sprintf("pkill: %s", out))
	}

	time.Sleep(2 * time.Second)
	llamaserver.StartOllama(ctx)

	return nil
}
