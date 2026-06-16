package ps

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
	"github.com/jparrill/auriga-cli/internal/ui"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

type processInfo struct {
	Component string
	Status    string
	PID       string
	Port      string
	Model     string
	Extra     string
}

func NewPsCmd() *cobra.Command {
	var watch int

	cmd := &cobra.Command{
		Use:   "ps",
		Short: "Show running auriga components",
		Long: `Show status of Ollama, llama-server, Pi, and system resources.

Examples:
  auriga ps              # One-shot status
  auriga ps --watch 5    # Refresh every 5 seconds`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if watch > 0 {
				return runWatch(watch)
			}
			printStatus()
			return nil
		},
	}

	cmd.Flags().IntVar(&watch, "watch", 0, "Refresh interval in seconds (0 = one-shot)")

	return cmd
}

func runWatch(interval int) error {
	for {
		fmt.Print("\033[2J\033[H") // Clear screen
		printStatus()
		fmt.Printf("\n  %s", ui.MutedStyle.Render(fmt.Sprintf("Refreshing every %ds — Ctrl+C to stop", interval)))
		time.Sleep(time.Duration(interval) * time.Second)
	}
}

func printStatus() {
	procs := gatherStatus()

	tbl := ui.NewTable("auriga ps", "COMPONENT", "STATUS", "PID", "PORT", "MODEL", "DETAILS")
	for _, p := range procs {
		status := ui.ErrorStyle.Render("stopped")
		if p.Status == "active" {
			status = ui.SuccessStyle.Render("active")
		}
		tbl.AddRow(p.Component, status, p.PID, p.Port, p.Model, p.Extra)
	}
	tbl.Print()

	printDiskUsage()
	printGPUMemory()
}

func gatherStatus() []processInfo {
	var procs []processInfo
	procs = append(procs, checkOllama())
	procs = append(procs, checkLlamaServer())
	procs = append(procs, checkPi())
	return procs
}

func checkOllama() processInfo {
	p := processInfo{Component: "ollama", Status: "stopped", PID: "-", Port: "-", Model: "-", Extra: "-"}

	ctx := context.Background()
	out, err := exec.RunCapture(ctx, "systemctl", []string{"is-active", "ollama"}, exec.RunOpts{})
	if err == nil && strings.TrimSpace(out) == "active" {
		p.Status = "active"
		p.Port = "11434"

		// Get PID
		pidOut, _ := exec.RunCapture(ctx, "pgrep", []string{"-f", "ollama serve"}, exec.RunOpts{})
		p.PID = strings.TrimSpace(strings.Split(pidOut, "\n")[0])

		// Get loaded model
		p.Model = getOllamaRunningModel()
	}

	return p
}

func getOllamaRunningModel() string {
	host := viper.GetString("ollama.host")
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get(host + "/api/ps")
	if err != nil {
		return "-"
	}
	defer resp.Body.Close()

	var data struct {
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil || len(data.Models) == 0 {
		return "(idle)"
	}

	var names []string
	for _, m := range data.Models {
		names = append(names, m.Name)
	}
	return strings.Join(names, ", ")
}

func checkLlamaServer() processInfo {
	p := processInfo{Component: "llama-server", Status: "stopped", PID: "-", Port: "-", Model: "-", Extra: "-"}

	ctx := context.Background()
	out, err := exec.RunCapture(ctx, "pgrep", []string{"-a", "llama-server"}, exec.RunOpts{})
	if err != nil || strings.TrimSpace(out) == "" {
		return p
	}

	p.Status = "active"
	line := strings.TrimSpace(strings.Split(out, "\n")[0])
	parts := strings.SplitN(line, " ", 2)
	p.PID = parts[0]

	if len(parts) > 1 {
		args := parts[1]
		p.Model = extractFlag(args, "-m")
		if p.Model != "" {
			p.Model = filepath.Base(p.Model)
		}

		port := extractFlag(args, "--port")
		if port != "" {
			p.Port = port
		}

		mmproj := extractFlag(args, "--mmproj")
		if mmproj != "" {
			p.Extra = "vision: " + filepath.Base(mmproj)
		}
	}

	return p
}

func checkPi() processInfo {
	p := processInfo{Component: "pi", Status: "stopped", PID: "-", Port: "-", Model: "-", Extra: "-"}

	ctx := context.Background()
	out, err := exec.RunCapture(ctx, "pgrep", []string{"-a", "pi"}, exec.RunOpts{})
	if err != nil || strings.TrimSpace(out) == "" {
		return p
	}

	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if !strings.Contains(line, "pi --model") && !strings.Contains(line, "pi-coding-agent") {
			continue
		}

		p.Status = "active"
		parts := strings.SplitN(line, " ", 2)
		p.PID = parts[0]

		if len(parts) > 1 {
			model := extractFlag(parts[1], "--model")
			if model != "" {
				p.Model = model
			}
		}
		break
	}

	return p
}

func extractFlag(args, flag string) string {
	fields := strings.Fields(args)
	for i, f := range fields {
		if f == flag && i+1 < len(fields) {
			return fields[i+1]
		}
	}
	return ""
}

func printDiskUsage() {
	ggufDir := config.ExpandHome(viper.GetString("llama_server.gguf_dir"))
	mmprojDir := config.ExpandHome(viper.GetString("llama_server.mmproj_dir"))
	resultsDir := config.ExpandHome(viper.GetString("benchmark.results_dir"))
	ollamaDir := resolveOllamaModelsDir()

	ggufSize := dirSize(ggufDir)
	mmprojSize := dirSize(mmprojDir)
	resultsSize := dirSize(resultsDir)
	ollamaSize := dirSize(ollamaDir)
	total := ggufSize + mmprojSize + resultsSize + ollamaSize

	tbl := ui.NewTable("Disk Usage", "COMPONENT", "PATH", "SIZE")
	tbl.AddRow("Ollama models", shortenPath(ollamaDir), formatGB(ollamaSize))
	tbl.AddRow("GGUF models", shortenPath(ggufDir), formatGB(ggufSize))
	tbl.AddRow("MM Projectors", shortenPath(mmprojDir), formatGB(mmprojSize))
	tbl.AddRow("Bench results", shortenPath(resultsDir), formatGB(resultsSize))
	tbl.AddRow("TOTAL", "", formatGB(total))
	tbl.Print()
}

func shortenPath(path string) string {
	home, _ := os.UserHomeDir()
	if strings.HasPrefix(path, home) {
		return "~" + path[len(home):]
	}
	return path
}

func resolveOllamaModelsDir() string {
	if v := os.Getenv("OLLAMA_MODELS"); v != "" {
		return v
	}
	return config.ExpandHome(viper.GetString("ollama.models_dir"))
}

func printGPUMemory() {
	ctx := context.Background()
	// Try reading from sysfs (AMD)
	vramUsed, err := os.ReadFile("/sys/class/drm/card1/device/mem_info_vram_used")
	vramTotal, err2 := os.ReadFile("/sys/class/drm/card1/device/mem_info_vram_total")
	gttUsed, err3 := os.ReadFile("/sys/class/drm/card1/device/mem_info_gtt_used")
	gttTotal, err4 := os.ReadFile("/sys/class/drm/card1/device/mem_info_gtt_total")

	if err == nil && err2 == nil && err3 == nil && err4 == nil {
		tbl := ui.NewTable("GPU Memory", "TYPE", "USED", "TOTAL")
		tbl.AddRow("VRAM", formatBytesStr(vramUsed), formatBytesStr(vramTotal))
		tbl.AddRow("GTT", formatBytesStr(gttUsed), formatBytesStr(gttTotal))
		tbl.Print()
		return
	}

	// Fallback: try rocm-smi
	out, err := exec.RunCapture(ctx, "rocm-smi", []string{"--showmeminfo", "vram"}, exec.RunOpts{})
	if err == nil {
		fmt.Printf("  %s\n", ui.BoldStyle.Render("GPU Memory"))
		for _, line := range strings.Split(out, "\n") {
			if strings.Contains(line, "Used") || strings.Contains(line, "Total") {
				fmt.Printf("  %s\n", strings.TrimSpace(line))
			}
		}
		fmt.Println()
	}
}

func dirSize(path string) int64 {
	var size int64
	filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() {
			size += info.Size()
		}
		return nil
	})
	return size
}

func formatGB(bytes int64) string {
	gb := float64(bytes) / (1024 * 1024 * 1024)
	return fmt.Sprintf("%.1f GB", gb)
}

func formatBytesStr(raw []byte) string {
	s := strings.TrimSpace(string(raw))
	var n int64
	fmt.Sscanf(s, "%d", &n)
	return formatGB(n)
}
