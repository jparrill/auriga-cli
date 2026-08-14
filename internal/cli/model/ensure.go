package model

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/jparrill/auriga-cli/internal/cli/profile"
	"github.com/jparrill/auriga-cli/internal/config"
	"github.com/jparrill/auriga-cli/internal/exec"
	"github.com/jparrill/auriga-cli/internal/huggingface"
	"github.com/jparrill/auriga-cli/internal/llamaserver"
	"github.com/jparrill/auriga-cli/internal/ollama"
	"github.com/jparrill/auriga-cli/internal/ui"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func newModelEnsureCmd() *cobra.Command {
	var backend string
	var profileName string

	cmd := &cobra.Command{
		Use:   "ensure",
		Short: "Download missing models",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runModelEnsure(backend, profileName)
		},
	}

	cmd.Flags().StringVar(&backend, "backend", "all", "Backend (ollama, llama-server, all)")
	cmd.Flags().StringVar(&profileName, "profile", "", "Ensure only this profile (skip ollama/llama-server)")

	return cmd
}

func runModelEnsure(backend, profileName string) error {
	if profileName != "" {
		existing := viper.GetStringMap(fmt.Sprintf("profiles.%s", profileName))
		if len(existing) == 0 {
			return fmt.Errorf("profile %q not found", profileName)
		}
		fmt.Printf("\n  %s\n", ui.BoldStyle.Render("Ensuring profile:"))
		result := profile.SyncProfile(profileName)
		profile.PrintSyncSummary([]profile.SyncResult{result})
		return nil
	}

	ctx := context.Background()

	if backend == "all" || backend == "ollama" {
		models := ollama.ConfiguredModels()
		if len(models) > 0 {
			fmt.Printf("\n  %s\n", ui.BoldStyle.Render("Ensuring Ollama models:"))
			for _, m := range models {
				if ollama.HasModel(m) {
					ui.Ok(fmt.Sprintf("Already available: %s", m))
				} else {
					ui.Info(fmt.Sprintf("Pulling %s...", m))
					err := ui.WithSpinner(fmt.Sprintf("Pulling %s", m), func() error {
						_, e := exec.RunCapture(ctx, "ollama", []string{"pull", m}, exec.RunOpts{})
						return e
					})
					if err != nil {
						ui.Fail(fmt.Sprintf("Failed to pull %s: %v", m, err))
					} else {
						ui.Ok(fmt.Sprintf("Downloaded: %s", m))
					}
				}
			}
		}
	}

	if backend == "all" || backend == "llama-server" {
		models := llamaserver.ConfiguredModels()
		if len(models) > 0 {
			fmt.Printf("\n  %s\n", ui.BoldStyle.Render("Ensuring llama-server GGUFs:"))
			quant := viper.GetString("llama_server.quant")
			quantPriority := []string{quant, "Q4_K_L", "Q4_K_S", "Q4_K", "Q4"}
			for _, repo := range models {
				local := llamaserver.FindLocalGGUF(repo)
				if local != "" {
					info, _ := os.Stat(local)
					localSize := info.Size()
					sizeGB := float64(localSize) / (1024 * 1024 * 1024)
					ui.Ok(fmt.Sprintf("Already available: %s (%.1f GB)", filepath.Base(local), sizeGB))

					expectedSize, err := huggingface.ExpectedSize(repo, filepath.Base(local))
					if err == nil && expectedSize > 0 && localSize != expectedSize {
						ui.Warn(fmt.Sprintf("Size mismatch: local=%d bytes, expected=%d bytes — possible incomplete download", localSize, expectedSize))
						ui.Info(fmt.Sprintf("Re-download with: wget -c %s -O %s", huggingface.DownloadURL(repo, filepath.Base(local)), local))
					}
				} else {
					ui.Info(fmt.Sprintf("Resolving GGUF from %s (quant: %s)...", repo, quant))
					filename, expectedSize, err := huggingface.ResolveGGUF(repo, quantPriority)
					if err != nil {
						ui.Fail(fmt.Sprintf("Cannot resolve: %v", err))
						continue
					}
					ui.Info(fmt.Sprintf("Found: %s", filename))

					ggufDir := config.ExpandHome(viper.GetString("llama_server.gguf_dir"))
					url := huggingface.DownloadURL(repo, filename)
					dest := filepath.Join(ggufDir, filename)

					ui.Info(fmt.Sprintf("Downloading %s...", filename))
					_, err = exec.Run(ctx, "wget", []string{"-c", url, "-O", dest}, exec.RunOpts{})
					if err != nil {
						ui.Fail(fmt.Sprintf("Download failed: %v", err))
					} else {
						info, _ := os.Stat(dest)
						localSize := info.Size()
						sizeGB := float64(localSize) / (1024 * 1024 * 1024)
						if expectedSize > 0 && localSize != expectedSize {
							ui.Warn(fmt.Sprintf("Downloaded %s (%.1f GB) — SIZE MISMATCH: expected %d bytes, got %d", filename, sizeGB, expectedSize, localSize))
						} else {
							ui.Ok(fmt.Sprintf("Downloaded: %s (%.1f GB)", filename, sizeGB))
						}
					}
				}
			}
		}
	}

	if backend == "all" || backend == "llama-server" {
		profiles := viper.GetStringMap("profiles")
		if len(profiles) > 0 {
			results := profile.SyncAllProfiles()
			profile.PrintSyncSummary(results)
		}
	}

	return nil
}

