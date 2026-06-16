package profile

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/jparrill/auriga-cli/internal/config"
	"github.com/jparrill/auriga-cli/internal/llamaserver"
	"github.com/jparrill/auriga-cli/internal/ui"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func newProfileServeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "serve <profile-name>",
		Short: "Start llama-server with a profile",
		Long: `Start llama-server with the model and optional mmproj from a configured profile.
If the profile has vision (mmproj), --jinja is added automatically.

Examples:
  auriga profile serve qwen3.6-vision
  auriga profile serve gemma4-12b-vision`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runProfileServe(args[0])
		},
	}
}

func runProfileServe(name string) error {
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
		return fmt.Errorf("model not found: %s\nRun: auriga model ensure --profile %s", modelPath, name)
	}

	var mmprojPath string
	if mmprojFile != "" {
		mmprojPath = filepath.Join(mmprojDir, mmprojFile)
		if _, err := os.Stat(mmprojPath); err != nil {
			return fmt.Errorf("mmproj not found: %s\nRun: auriga model ensure --profile %s", mmprojPath, name)
		}
	}

	params := []ui.OrderedParam{
		{Key: "Profile", Value: name},
		{Key: "Model", Value: modelFile},
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

	ui.Info("Press Ctrl+C to stop")
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	fmt.Println()
	llamaserver.Stop(proc)

	return nil
}
