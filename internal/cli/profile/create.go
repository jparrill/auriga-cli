package profile

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/jparrill/auriga-cli/internal/config"
	"github.com/jparrill/auriga-cli/internal/huggingface"
	"github.com/jparrill/auriga-cli/internal/ui"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

type createOpts struct {
	Repo   string
	Model  string
	Vision bool
}

func newProfileCreateCmd() *cobra.Command {
	opts := &createOpts{}

	cmd := &cobra.Command{
		Use:   "create <name>",
		Short: "Create a new llama-server profile",
		Long: `Create a profile by specifying a HuggingFace repo. The CLI auto-resolves
the GGUF (based on preferred quant) and mmproj (if --vision is set).

Examples:
  auriga profile create gemma4-12b-vision --repo unsloth/gemma-4-12b-it-GGUF --vision
  auriga profile create qwen3.6 --repo unsloth/Qwen3.6-35B-A3B-GGUF
  auriga profile create custom --repo unsloth/Qwen3-30B-A3B-GGUF --model Qwen3-30B-A3B-Q4_K_M.gguf`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runProfileCreate(args[0], opts)
		},
	}

	cmd.Flags().StringVar(&opts.Repo, "repo", "", "HuggingFace repo (e.g., unsloth/gemma-4-12b-it-GGUF)")
	cmd.Flags().StringVar(&opts.Model, "model", "", "GGUF filename (auto-resolved from repo if omitted)")
	cmd.Flags().BoolVar(&opts.Vision, "vision", false, "Enable vision (auto-discover mmproj from repo)")
	cmd.MarkFlagRequired("repo")

	return cmd
}

func runProfileCreate(name string, opts *createOpts) error {
	quant := viper.GetString("llama_server.quant")
	quantPriority := []string{quant, "Q4_K_L", "Q4_K_S", "Q4_K", "Q4"}

	existing := viper.GetStringMap(fmt.Sprintf("profiles.%s", name))
	if len(existing) > 0 {
		return fmt.Errorf("profile %q already exists — delete it first with: auriga profile delete %s", name, name)
	}

	ui.Info(fmt.Sprintf("Resolving from repo: %s", opts.Repo))

	// Resolve GGUF
	var modelFile string
	if opts.Model != "" {
		modelFile = opts.Model
		ui.Ok(fmt.Sprintf("Model (manual): %s", modelFile))
	} else {
		ui.Info(fmt.Sprintf("Resolving GGUF (quant priority: %s)...", quant))
		var err error
		var size int64
		modelFile, size, err = huggingface.ResolveGGUF(opts.Repo, quantPriority)
		if err != nil {
			return fmt.Errorf("cannot resolve GGUF: %w", err)
		}
		sizeGB := float64(size) / (1024 * 1024 * 1024)
		ui.Ok(fmt.Sprintf("Model: %s (%.1f GB)", modelFile, sizeGB))
	}

	// Resolve mmproj
	var mmprojFile string
	if opts.Vision {
		ui.Info("Resolving mmproj for vision...")
		var err error
		var size int64
		mmprojFile, size, err = huggingface.ResolveMMProj(opts.Repo)
		if err != nil {
			return fmt.Errorf("cannot resolve mmproj: %w (this model may not support vision)", err)
		}
		sizeMB := float64(size) / (1024 * 1024)
		ui.Ok(fmt.Sprintf("MMProj: %s (%.0f MB)", mmprojFile, sizeMB))
	}

	// Build profile data
	profileData := map[string]interface{}{
		"repo":  opts.Repo,
		"model": modelFile,
	}
	if mmprojFile != "" {
		profileData["mmproj"] = mmprojFile
	}

	// Show summary
	params := []ui.OrderedParam{
		{Key: "Name", Value: name},
		{Key: "Repo", Value: opts.Repo},
		{Key: "Model", Value: modelFile},
	}
	if mmprojFile != "" {
		params = append(params, ui.OrderedParam{Key: "Vision", Value: mmprojFile})
	} else {
		params = append(params, ui.OrderedParam{Key: "Vision", Value: "no"})
	}

	confirmed, err := ui.ConfirmOperationOrdered("Create Profile", params, "", false)
	if err != nil || !confirmed {
		return err
	}

	// Write to config
	viper.Set(fmt.Sprintf("profiles.%s", name), profileData)
	cfgPath := viper.ConfigFileUsed()
	if cfgPath == "" {
		cfgPath = config.ExpandHome("~/.config/auriga/config.yaml")
	}

	if err := os.MkdirAll(filepath.Dir(cfgPath), 0755); err != nil {
		return err
	}
	if err := viper.WriteConfigAs(cfgPath); err != nil {
		return fmt.Errorf("cannot write config: %w", err)
	}

	ui.Ok(fmt.Sprintf("Profile %q created in %s", name, cfgPath))

	// Check if files are downloaded
	ggufDir := config.ExpandHome(viper.GetString("llama_server.gguf_dir"))
	mmprojDir := config.ExpandHome(viper.GetString("llama_server.mmproj_dir"))

	if _, err := os.Stat(filepath.Join(ggufDir, modelFile)); err != nil {
		ui.Warn(fmt.Sprintf("GGUF not downloaded yet: %s", modelFile))
		ui.Info(fmt.Sprintf("Run: auriga model ensure --profile %s", name))
	}
	if mmprojFile != "" {
		if _, err := os.Stat(filepath.Join(mmprojDir, mmprojFile)); err != nil {
			ui.Warn(fmt.Sprintf("MMProj not downloaded yet: %s", mmprojFile))
			ui.Info(fmt.Sprintf("Run: auriga model ensure --profile %s", name))
		}
	}

	return nil
}
