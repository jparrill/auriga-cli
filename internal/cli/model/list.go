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

	var entries []entry
	for _, m := range tags.Models {
		sizeGB := float64(m.Size) / (1024 * 1024 * 1024)
		entries = append(entries, entry{m.Name, fmt.Sprintf("%.1f GB", sizeGB)})
	}
	printTable("MODEL", "SIZE", entries)
}

func listGGUFModels() {
	ggufDir := config.ExpandHome(viper.GetString("llama_server.gguf_dir"))
	mmprojDir := config.ExpandHome(viper.GetString("llama_server.mmproj_dir"))

	fmt.Printf("\n  %s\n", ui.BoldStyle.Render("GGUF Models ("+ggufDir+")"))

	dirEntries, err := os.ReadDir(ggufDir)
	if err != nil {
		ui.Fail(fmt.Sprintf("Cannot read %s: %v", ggufDir, err))
		return
	}

	var files []entry
	for _, e := range dirEntries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".gguf" {
			continue
		}
		info, _ := e.Info()
		sizeGB := float64(info.Size()) / (1024 * 1024 * 1024)
		files = append(files, entry{e.Name(), fmt.Sprintf("%.1f GB", sizeGB)})
	}
	printTable("FILE", "SIZE", files)

	fmt.Printf("  %s\n", ui.BoldStyle.Render("Multimodal Projectors ("+mmprojDir+")"))
	projEntries, err := os.ReadDir(mmprojDir)
	if err != nil {
		ui.Warn(fmt.Sprintf("Cannot read %s: %v", mmprojDir, err))
		return
	}

	var projs []entry
	for _, e := range projEntries {
		if e.IsDir() {
			continue
		}
		info, _ := e.Info()
		sizeMB := float64(info.Size()) / (1024 * 1024)
		projs = append(projs, entry{e.Name(), fmt.Sprintf("%.0f MB", sizeMB)})
	}
	printTable("FILE", "SIZE", projs)
}

type entry struct {
	name string
	size string
}

func printTable(nameHeader, sizeHeader string, rows []entry) {
	maxName := len(nameHeader)
	for _, r := range rows {
		if len(r.name) > maxName {
			maxName = len(r.name)
		}
	}

	fmtStr := fmt.Sprintf("  %%-%ds  %%10s\n", maxName)
	fmt.Printf(fmtStr, nameHeader, sizeHeader)
	fmt.Printf("  %s\n", repeatChar('─', maxName+12))
	for _, r := range rows {
		fmt.Printf(fmtStr, r.name, r.size)
	}
	fmt.Println()
}

func repeatChar(c rune, n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = byte(c)
	}
	return string(b)
}
