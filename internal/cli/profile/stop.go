package profile

import (
	"context"
	"fmt"
	"os"
	"strings"
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
				port := portForProfileName(args[0])
				if port == 0 {
					return fmt.Errorf("profile %q not found — use 'auriga profile stop' to stop all", args[0])
				}
				return stopOnPort(port)
			}
			return runProfileStopAll()
		},
	}
}

// portForProfileName resolves the port a profile would run on: an explicit
// port wins, otherwise it is derived from the type (dense -> slot 1, moe -> slot 2).
func portForProfileName(name string) int {
	profileKey := fmt.Sprintf("profiles.%s", name)
	if p := viper.GetInt(profileKey + ".port"); p > 0 {
		return p
	}
	if viper.Get(profileKey+".model") == nil && viper.Get(profileKey+".model_dir") == nil {
		return 0
	}
	slot := 1
	if viper.GetString(profileKey+".type") == "moe" {
		slot = 2
	}
	return llamaserver.SlotPort(slot)
}

func runProfileStopAll() error {
	ports := allProfilePorts()

	for _, port := range ports {
		stopOnPort(port)
	}

	ctx := context.Background()
	out, _ := exec.RunCapture(ctx, "pgrep", []string{"-f", "llama-server"}, exec.RunOpts{})
	if strings.TrimSpace(out) != "" {
		ui.Info("Cleaning up remaining llama-server processes...")
		exec.RunCapture(ctx, "pkill", []string{"-f", "llama-server"}, exec.RunOpts{})
	}

	for _, port := range ports {
		llamaserver.StopVLLMOnPort(port)
	}

	time.Sleep(2 * time.Second)
	return nil
}

func stopOnPort(port int) error {
	stopped := false

	if systemd.IsActiveOnPort(port) {
		ui.Info(fmt.Sprintf("Stopping systemd-managed server on port %d...", port))
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
				ui.Info(fmt.Sprintf("Stopping server (PID %d) on port %d...", pid, port))
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

	// vLLM may run natively or in the podman adapter; neither shows up in the
	// pid file when it was started outside auriga.
	if llamaserver.StopVLLMOnPort(port) {
		stopped = true
	}

	llamaserver.RemoveActiveProfile(port)

	if stopped {
		ui.Ok(fmt.Sprintf("stopped on port %d", port))
	}
	return nil
}

func allProfilePorts() []int {
	seen := map[int]bool{}
	for _, port := range llamaserver.AllSlotPorts() {
		seen[port] = true
	}

	profiles := viper.GetStringMap("profiles")
	for name := range profiles {
		if p := viper.GetInt(fmt.Sprintf("profiles.%s.port", name)); p > 0 {
			seen[p] = true
		}
	}

	ports := make([]int, 0, len(seen))
	for p := range seen {
		ports = append(ports, p)
	}
	return ports
}
