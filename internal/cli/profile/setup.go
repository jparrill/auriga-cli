package profile

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/jparrill/auriga-cli/internal/config"
	"github.com/jparrill/auriga-cli/internal/exec"
	"github.com/jparrill/auriga-cli/internal/huggingface"
	"github.com/jparrill/auriga-cli/internal/ui"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

type setupOpts struct {
	Repo   string
	Vision bool
	Quant  string
}

func newProfileSetupCmd() *cobra.Command {
	opts := &setupOpts{}

	cmd := &cobra.Command{
		Use:   "setup <name>",
		Short: "Download model + create profile in one step",
		Long: `Resolve, download, and configure a llama-server profile from a HuggingFace repo.

Combines model resolution, download, and profile creation into a single command.
If --vision is set, the mmproj is also resolved and downloaded.

Examples:
  auriga profile setup qwen3.6-vision --repo unsloth/Qwen3.6-35B-A3B-GGUF --vision --quant Q8_0
  auriga profile setup gemma4 --repo unsloth/gemma-4-26B-A4B-it-GGUF`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runProfileSetup(args[0], opts)
		},
	}

	cmd.Flags().StringVar(&opts.Repo, "repo", "", "HuggingFace repo (e.g., unsloth/Qwen3.6-35B-A3B-GGUF)")
	cmd.Flags().BoolVar(&opts.Vision, "vision", false, "Also download mmproj for vision support")
	cmd.Flags().StringVar(&opts.Quant, "quant", "", "Quantization level (default from config)")
	cmd.MarkFlagRequired("repo")

	return cmd
}

func runProfileSetup(name string, opts *setupOpts) error {
	existing := viper.GetStringMap(fmt.Sprintf("profiles.%s", name))
	if len(existing) > 0 {
		return fmt.Errorf("profile %q already exists — delete it first with: auriga profile delete %s", name, name)
	}

	quant := opts.Quant
	if quant == "" {
		quant = viper.GetString("llama_server.quant")
	}
	quantPriority := []string{quant, "Q4_K_L", "Q4_K_S", "Q4_K", "Q4"}

	ggufDir := config.ExpandHome(viper.GetString("llama_server.gguf_dir"))
	mmprojDir := config.ExpandHome(viper.GetString("llama_server.mmproj_dir"))
	ctx := context.Background()

	// Resolve GGUF
	ui.Info(fmt.Sprintf("Resolving GGUF from %s (quant: %s)...", opts.Repo, quant))
	modelFile, expectedSize, err := huggingface.ResolveGGUF(opts.Repo, quantPriority)
	if err != nil {
		return fmt.Errorf("cannot resolve GGUF: %w", err)
	}
	sizeGB := float64(expectedSize) / (1024 * 1024 * 1024)
	ui.Ok(fmt.Sprintf("Model: %s (%.1f GB)", modelFile, sizeGB))

	// Resolve mmproj
	var mmprojFile string
	var mmprojExpectedSize int64
	if opts.Vision {
		ui.Info("Resolving mmproj for vision...")
		var err error
		mmprojFile, mmprojExpectedSize, err = huggingface.ResolveMMProj(opts.Repo)
		if err != nil {
			return fmt.Errorf("cannot resolve mmproj: %w (this model may not support vision)", err)
		}
		repoBase := filepath.Base(opts.Repo)
		repoBase = strings.TrimSuffix(repoBase, "-GGUF")
		if !strings.Contains(mmprojFile, repoBase) {
			ext := filepath.Ext(mmprojFile)
			mmprojFile = repoBase + "-" + mmprojFile[:len(mmprojFile)-len(ext)] + ext
		}
		sizeMB := float64(mmprojExpectedSize) / (1024 * 1024)
		ui.Ok(fmt.Sprintf("MMProj: %s (%.0f MB)", mmprojFile, sizeMB))
	}

	// Confirmation
	params := []ui.OrderedParam{
		{Key: "Profile", Value: name},
		{Key: "Repo", Value: opts.Repo},
		{Key: "Model", Value: fmt.Sprintf("%s (%.1f GB)", modelFile, sizeGB)},
		{Key: "Quant", Value: quant},
	}
	if mmprojFile != "" {
		params = append(params, ui.OrderedParam{Key: "Vision", Value: mmprojFile})
	}
	params = append(params, ui.OrderedParam{Key: "GGUF dir", Value: ggufDir})

	confirmed, err := ui.ConfirmOperationOrdered("Setup Profile", params, "", false)
	if err != nil || !confirmed {
		return err
	}

	// Download GGUF
	modelDest := filepath.Join(ggufDir, modelFile)
	if _, err := os.Stat(modelDest); err == nil {
		info, _ := os.Stat(modelDest)
		if expectedSize > 0 && info.Size() != expectedSize {
			ui.Warn(fmt.Sprintf("Existing file size mismatch — re-downloading"))
		} else {
			ui.Ok(fmt.Sprintf("Already downloaded: %s", modelFile))
			goto skipModelDownload
		}
	}

	{
		url := huggingface.DownloadURL(opts.Repo, modelFile)
		err = exec.DownloadFile(ctx, url, modelDest, modelFile, exec.DownloadOpts{Resume: true})
		if err != nil {
			return fmt.Errorf("download failed: %w", err)
		}
		info, _ := os.Stat(modelDest)
		if expectedSize > 0 && info.Size() != expectedSize {
			ui.Warn(fmt.Sprintf("Size mismatch: got %d, expected %d", info.Size(), expectedSize))
		} else {
			ui.Ok(fmt.Sprintf("Downloaded: %s (%.1f GB)", modelFile, float64(info.Size())/(1024*1024*1024)))
		}
	}

skipModelDownload:

	// Download mmproj
	if mmprojFile != "" {
		mmprojDest := filepath.Join(mmprojDir, mmprojFile)
		if _, err := os.Stat(mmprojDest); err == nil {
			ui.Ok(fmt.Sprintf("Already downloaded: %s", mmprojFile))
		} else {
			originalMmproj := repoFilename(mmprojFile, opts.Repo)

			url := huggingface.DownloadURL(opts.Repo, originalMmproj)
			err = exec.DownloadFile(ctx, url, mmprojDest, mmprojFile, exec.DownloadOpts{Resume: true})
			if err != nil {
				os.Remove(mmprojDest)
				url = huggingface.DownloadURL(opts.Repo, mmprojFile)
				err = exec.DownloadFile(ctx, url, mmprojDest, mmprojFile, exec.DownloadOpts{Resume: true})
				if err != nil {
					return fmt.Errorf("mmproj download failed: %w", err)
				}
			}
			if mmprojExpectedSize > 0 {
				info, _ := os.Stat(mmprojDest)
				if info != nil && info.Size() != mmprojExpectedSize {
					ui.Warn(fmt.Sprintf("mmproj size mismatch: got %d, expected %d", info.Size(), mmprojExpectedSize))
				}
			}
			ui.Ok(fmt.Sprintf("Downloaded: %s", mmprojFile))
		}
	}

	// Create profile
	pc := ProfileConfig{
		Repo:   opts.Repo,
		Model:  modelFile,
		MMProj: mmprojFile,
	}
	if mmprojFile != "" {
		pc.Flags = []string{"--jinja"}
	}

	if err := addProfileToConfig(name, pc); err != nil {
		return fmt.Errorf("cannot write config: %w", err)
	}

	ui.Ok(fmt.Sprintf("Profile %q created in %s", name, configPath()))
	ui.Info("To start: auriga profile serve " + name)

	return nil
}
