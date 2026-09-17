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

func validateSlot(slot int) error {
	if slot != 1 && slot != 2 {
		return fmt.Errorf("--slot must be 1 or 2, got %d", slot)
	}
	return nil
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

func profileCtxSize(name string) int {
	profileKey := fmt.Sprintf("profiles.%s", name)
	if c := viper.GetInt(profileKey + ".ctx_size"); c > 0 {
		return c
	}
	if c := viper.GetInt("llama_server.ctx_size"); c > 0 {
		return c
	}
	return 131072
}

func warnTypeMismatch(name, configuredType, modelName string) {
	if configuredType == "" {
		return
	}
	detected := detectModelType(modelName)
	if configuredType != detected {
		ui.Info(fmt.Sprintf("Profile %q type=%s (model name heuristic: %s)", name, configuredType, detected))
	}
}

func newProfileServeCmd() *cobra.Command {
	var (
		daemon  bool
		ctxSize int
		slot    int
	)

	cmd := &cobra.Command{
		Use:   "serve <profile-name>",
		Short: "Start llama-server with a profile",
		Long: `Start llama-server with the model and optional mmproj from a configured profile.
If the profile has vision (mmproj), --jinja is added automatically.

Context size resolves: --ctx-size flag > profile ctx_size > llama_server.ctx_size > 131072.

Examples:
  auriga profile serve qwen3.6-vision --slot 1            # Foreground on slot 1
  auriga profile serve qwen3.6-vision --slot 2 --daemon    # Background on slot 2
  auriga profile serve gemma4-12b-vision --slot 1 --ctx-size 65536`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateSlot(slot); err != nil {
				return err
			}
			if !cmd.Flags().Changed("ctx-size") {
				ctxSize = profileCtxSize(args[0])
			}
			return runProfileServe(args[0], daemon, ctxSize, slot)
		},
	}

	cmd.Flags().BoolVarP(&daemon, "daemon", "d", false, "Run in background")
	cmd.Flags().IntVar(&ctxSize, "ctx-size", 131072, "Context window size (default from config)")
	cmd.Flags().IntVar(&slot, "slot", 0, "Slot to run on (1 or 2, required)")
	cmd.MarkFlagRequired("slot")

	return cmd
}

func runProfileServe(name string, daemon bool, ctxSize int, slot int) error {
	profileKey := fmt.Sprintf("profiles.%s", name)
	modelFile := viper.GetString(profileKey + ".model")
	if modelFile == "" {
		return fmt.Errorf("profile %q not found — run: auriga profile list", name)
	}
	port := llamaserver.SlotPort(slot)
	pf := pidFileForPort(port)
	warnTypeMismatch(name, viper.GetString(profileKey+".type"), modelFile)

	if existingPID := readPIDForPort(port); existingPID > 0 {
		if processExists(existingPID) {
			return fmt.Errorf("llama-server already running on port %d (PID %d) — run: auriga profile stop", port, existingPID)
		}
		os.Remove(pf)
	}

	if portInUse(port) {
		return fmt.Errorf("port %d already in use — another process may be running\nRun: auriga profile stop", port)
	}

	preflight, err := resolveProfilePreflight(name, ctxSize, slot, "auriga model ensure")
	if err != nil {
		return err
	}
	if err := validateProfileBinary(preflight); err != nil {
		return err
	}

	mode := "foreground"
	if daemon {
		mode = "daemon"
	}

	pType := profileType(name)
	params := []ui.OrderedParam{
		{Key: "Profile", Value: name},
		{Key: "Model", Value: preflight.ModelFile},
		{Key: "Type", Value: pType},
		{Key: "Mode", Value: mode},
	}
	if viper.GetString(profileKey+".bin") != "" {
		params = append(params, ui.OrderedParam{Key: "Binary", Value: preflight.Binary})
	}
	if preflight.MMProjFile != "" {
		params = append(params, ui.OrderedParam{Key: "Vision", Value: preflight.MMProjFile})
	}
	if preflight.DFlashFile != "" {
		params = append(params, ui.OrderedParam{Key: "DFlash", Value: preflight.DFlashFile})
	}
	if preflight.MTPDrafterFile != "" {
		params = append(params, ui.OrderedParam{Key: "MTP Drafter", Value: preflight.MTPDrafterFile})
	}
	params = append(params, ui.OrderedParam{Key: "Port", Value: fmt.Sprintf("%d", port)})
	params = append(params, ui.OrderedParam{Key: "Context", Value: fmt.Sprintf("%d", ctxSize)})
	if profileFlags := preflight.Flags; len(profileFlags) > 0 {
		params = append(params, ui.OrderedParam{Key: "Flags", Value: formatFlagPairs(profileFlags)})
	}

	confirmed, err := ui.ConfirmOperationOrdered("Start llama-server", params, "", false)
	if err != nil || !confirmed {
		return err
	}

	extraFlags := profileFlagsWithVision(preflight.Flags, preflight.MMProjFile)
	extraFlags = injectDrafterFlags(name, preflight.GGUFDir, extraFlags)

	ctx := context.Background()
	proc, err := llamaserver.StartWithCtx(ctx, preflight.Binary, preflight.ModelPath, preflight.MMProjPath, extraFlags, ctxSize, port)
	if err != nil {
		return err
	}

	os.WriteFile(pf, []byte(strconv.Itoa(proc.Pid)), 0644)

	if daemon {
		ui.Ok(fmt.Sprintf("llama-server running in background (PID %d) on port %d", proc.Pid, port))
		ui.Info("Stop with: auriga profile stop")
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

func injectDrafterFlags(name, ggufDir string, flags []string) []string {
	if containsFlag(flags, "--model-draft") {
		return flags
	}
	profileKey := fmt.Sprintf("profiles.%s", name)
	for _, field := range []string{".mtp_drafter", ".dflash"} {
		drafter := viper.GetString(profileKey + field)
		if drafter == "" {
			continue
		}
		drafterPath := filepath.Join(ggufDir, drafter)
		if _, err := os.Stat(drafterPath); err != nil {
			ui.Warn(fmt.Sprintf("%s not found: %s — skipping --model-draft", field[1:], drafter))
			continue
		}
		flags = append(flags, "--model-draft", drafterPath)
		return flags
	}
	return flags
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
