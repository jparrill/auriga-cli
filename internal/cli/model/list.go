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

	tbl := ui.NewTable(fmt.Sprintf("Ollama Models (%s)", host), "MODEL", "SIZE")
	for _, m := range tags.Models {
		sizeGB := float64(m.Size) / (1024 * 1024 * 1024)
		tbl.AddRow(m.Name, fmt.Sprintf("%.1f GB", sizeGB))
	}
	tbl.Print()
}

func listGGUFModels() {
	ggufDir := config.ExpandHome(viper.GetString("llama_server.gguf_dir"))
	mmprojDir := config.ExpandHome(viper.GetString("llama_server.mmproj_dir"))

	dirEntries, err := os.ReadDir(ggufDir)
	if err != nil {
		ui.Fail(fmt.Sprintf("Cannot read %s: %v", ggufDir, err))
		return
	}

	tbl := ui.NewTable(fmt.Sprintf("GGUF Models (%s)", ggufDir), "FILE", "SIZE")
	for _, e := range dirEntries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".gguf" {
			continue
		}
		info, _ := e.Info()
		sizeGB := float64(info.Size()) / (1024 * 1024 * 1024)
		tbl.AddRow(e.Name(), fmt.Sprintf("%.1f GB", sizeGB))
	}
	tbl.Print()

	projEntries, err := os.ReadDir(mmprojDir)
	if err != nil {
		ui.Warn(fmt.Sprintf("Cannot read %s: %v", mmprojDir, err))
		return
	}

	projTbl := ui.NewTable(fmt.Sprintf("Multimodal Projectors (%s)", mmprojDir), "FILE", "SIZE")
	for _, e := range projEntries {
		if e.IsDir() {
			continue
		}
		info, _ := e.Info()
		sizeMB := float64(info.Size()) / (1024 * 1024)
		projTbl.AddRow(e.Name(), fmt.Sprintf("%.0f MB", sizeMB))
	}
	projTbl.Print()
}
