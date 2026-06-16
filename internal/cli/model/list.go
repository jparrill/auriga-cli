package model

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/jparrill/auriga-cli/internal/config"
	"github.com/jparrill/auriga-cli/internal/ui"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

type ollamaTagsResponse struct {
	Models []struct {
		Name       string `json:"name"`
		Size       int64  `json:"size"`
		ModifiedAt string `json:"modified_at"`
	} `json:"models"`
}

func newModelListCmd() *cobra.Command {
	var backend string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List available models",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runModelList(backend)
		},
	}

	cmd.Flags().StringVar(&backend, "backend", "all", "Backend to list (ollama, llama-server, all)")

	return cmd
}

func runModelList(backend string) error {
	if backend == "all" || backend == "ollama" {
		listOllamaModels()
	}
	if backend == "all" || backend == "llama-server" {
		listGGUFModels()
	}
	return nil
}

func listOllamaModels() {
	host := viper.GetString("ollama.host")
	fmt.Printf("\n  %s\n", ui.BoldStyle.Render("Ollama Models ("+host+")"))

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(host + "/api/tags")
	if err != nil {
		ui.Fail(fmt.Sprintf("Cannot reach Ollama: %v", err))
		return
	}
	defer resp.Body.Close()

	var tags ollamaTagsResponse
	if err := json.NewDecoder(resp.Body).Decode(&tags); err != nil {
		ui.Fail(fmt.Sprintf("Invalid response: %v", err))
		return
	}

	fmt.Printf("  %-55s %10s\n", "MODEL", "SIZE")
	fmt.Printf("  %s\n", "──────────────────────────────────────────────────────────────────")
	for _, m := range tags.Models {
		sizeGB := float64(m.Size) / (1024 * 1024 * 1024)
		fmt.Printf("  %-55s %8.1f GB\n", m.Name, sizeGB)
	}
	fmt.Println()
}

func listGGUFModels() {
	ggufDir := config.ExpandHome(viper.GetString("llama_server.gguf_dir"))
	mmprojDir := config.ExpandHome(viper.GetString("llama_server.mmproj_dir"))

	fmt.Printf("  %s\n", ui.BoldStyle.Render("GGUF Models ("+ggufDir+")"))

	entries, err := os.ReadDir(ggufDir)
	if err != nil {
		ui.Fail(fmt.Sprintf("Cannot read %s: %v", ggufDir, err))
		return
	}

	fmt.Printf("  %-55s %10s\n", "FILE", "SIZE")
	fmt.Printf("  %s\n", "──────────────────────────────────────────────────────────────────")
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".gguf" {
			continue
		}
		info, _ := e.Info()
		sizeGB := float64(info.Size()) / (1024 * 1024 * 1024)
		fmt.Printf("  %-55s %8.1f GB\n", e.Name(), sizeGB)
	}
	fmt.Println()

	fmt.Printf("  %s\n", ui.BoldStyle.Render("Multimodal Projectors ("+mmprojDir+")"))
	projEntries, err := os.ReadDir(mmprojDir)
	if err != nil {
		ui.Warn(fmt.Sprintf("Cannot read %s: %v", mmprojDir, err))
		return
	}

	for _, e := range projEntries {
		if e.IsDir() {
			continue
		}
		info, _ := e.Info()
		sizeMB := float64(info.Size()) / (1024 * 1024)
		fmt.Printf("  %-55s %8.0f MB\n", e.Name(), sizeMB)
	}
	fmt.Println()
}
