package model

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jparrill/auriga-cli/internal/config"
	"github.com/jparrill/auriga-cli/internal/exec"
	"github.com/jparrill/auriga-cli/internal/llamaserver"
	"github.com/jparrill/auriga-cli/internal/ollama"
	"github.com/jparrill/auriga-cli/internal/ui"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func newModelEnsureCmd() *cobra.Command {
	var backend string

	cmd := &cobra.Command{
		Use:   "ensure",
		Short: "Download missing models",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runModelEnsure(backend)
		},
	}

	cmd.Flags().StringVar(&backend, "backend", "all", "Backend (ollama, llama-server, all)")

	return cmd
}

func runModelEnsure(backend string) error {
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
			for _, repo := range models {
				local := llamaserver.FindLocalGGUF(repo)
				if local != "" {
					info, _ := os.Stat(local)
					sizeGB := float64(info.Size()) / (1024 * 1024 * 1024)
					ui.Ok(fmt.Sprintf("Already available: %s (%.1f GB)", filepath.Base(local), sizeGB))
				} else {
					ui.Info(fmt.Sprintf("Resolving GGUF from %s (quant: %s)...", repo, quant))
					filename, err := resolveGGUFFilename(repo, quant)
					if err != nil {
						ui.Fail(fmt.Sprintf("Cannot resolve: %v", err))
						continue
					}
					ui.Info(fmt.Sprintf("Found: %s", filename))

					ggufDir := config.ExpandHome(viper.GetString("llama_server.gguf_dir"))
					url := fmt.Sprintf("https://huggingface.co/%s/resolve/main/%s", repo, filename)
					dest := filepath.Join(ggufDir, filename)

					ui.Info(fmt.Sprintf("Downloading %s...", filename))
					_, err = exec.Run(ctx, "wget", []string{"-c", url, "-O", dest}, exec.RunOpts{})
					if err != nil {
						ui.Fail(fmt.Sprintf("Download failed: %v", err))
					} else {
						info, _ := os.Stat(dest)
						sizeGB := float64(info.Size()) / (1024 * 1024 * 1024)
						ui.Ok(fmt.Sprintf("Downloaded: %s (%.1f GB)", filename, sizeGB))
					}
				}
			}
		}
	}

	return nil
}

func resolveGGUFFilename(hfRepo, preferredQuant string) (string, error) {
	quantPriority := []string{preferredQuant, "Q4_K_L", "Q4_K_S", "Q4_K", "Q4"}

	url := fmt.Sprintf("https://huggingface.co/api/models/%s/tree/main", hfRepo)
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return "", fmt.Errorf("HF API error: %w", err)
	}
	defer resp.Body.Close()

	var files []struct {
		Path string `json:"path"`
		Size int64  `json:"size"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&files); err != nil {
		return "", fmt.Errorf("invalid HF response: %w", err)
	}

	var ggufFiles []string
	for _, f := range files {
		if strings.HasSuffix(f.Path, ".gguf") {
			ggufFiles = append(ggufFiles, f.Path)
		}
	}

	for _, q := range quantPriority {
		for _, gf := range ggufFiles {
			if strings.Contains(gf, q) {
				return gf, nil
			}
		}
	}

	for _, f := range files {
		if strings.HasSuffix(f.Path, ".gguf") && f.Size > 1_000_000_000 {
			return f.Path, nil
		}
	}

	if len(ggufFiles) > 0 {
		return ggufFiles[0], nil
	}
	return "", fmt.Errorf("no GGUF found in %s", hfRepo)
}
