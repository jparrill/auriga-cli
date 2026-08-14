package profile

import (
	"context"
	"fmt"
	"os"
	"syscall"
	"time"

	"github.com/jparrill/auriga-cli/internal/exec"
	"github.com/jparrill/auriga-cli/internal/llamaserver"
	"github.com/jparrill/auriga-cli/internal/systemd"
	"github.com/jparrill/auriga-cli/internal/ui"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func newProfileStopCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "stop [profile-name]",
		Short: "Stop llama-server (all ports or specific profile)",
		Long: `Stop llama-server instances. Without arguments, stops all instances
on all known ports. With a profile name, stops only the instance on that
profile's port.

Examples:
  auriga profile stop                    # Stop all llama-server instances
  auriga profile stop qwen3.6-vision     # Stop only the MoE instance`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 1 {
				port := profilePort(args[0])
				return stopOnPort(port)
			}
			return runProfileStopAll()
		},
	}
}

func runProfileStopAll() error {
	ports := allProfilePorts()
	stopped := false

	for _, port := range ports {
		if err := stopOnPort(port); err == nil {
			stopped = true
		}
	}

	if !stopped {
		ui.Info("Stopping llama-server via pkill (fallback)...")
		ctx := context.Background()
		exec.RunCapture(ctx, "pkill", []string{"-f", "llama-server"}, exec.RunOpts{})
	}

	time.Sleep(2 * time.Second)
	return nil
}

func stopOnPort(port int) error {
	stopped := false

	if systemd.IsActiveOnPort(port) {
		ui.Info(fmt.Sprintf("Stopping systemd-managed llama-server on port %d...", port))
		if err := systemd.Stop(port); err != nil {
			ui.Warn(fmt.Sprintf("systemctl stop failed on port %d: %v", port, err))
		} else {
			stopped = true
		}
	}

	if !stopped {
		pf := pidFileForPort(port)
		if pid := readPIDForPort(port); pid > 0 {
			if processExists(pid) {
				ui.Info(fmt.Sprintf("Stopping llama-server (PID %d) on port %d...", pid, port))
				proc, err := os.FindProcess(pid)
				if err == nil {
					proc.Signal(syscall.SIGTERM)
					time.Sleep(2 * time.Second)
					proc.Kill()
					stopped = true
				}
			}
			os.Remove(pf)
		}
	}

	if stopped {
		ui.Ok(fmt.Sprintf("llama-server stopped on port %d", port))
	}
	return nil
}

func allProfilePorts() []int {
	seen := map[int]bool{}
	seen[llamaserver.DensePort()] = true
	seen[llamaserver.MoePort()] = true

	profiles := viper.GetStringMap("profiles")
	for name := range profiles {
		seen[profilePort(name)] = true
	}

	ports := make([]int, 0, len(seen))
	for p := range seen {
		ports = append(ports, p)
	}
	return ports
}
