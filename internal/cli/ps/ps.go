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
	Profile   string
	ModelType string
	Managed   string
	Health    string
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

	printLlamaServerDetail(procs)
	printDiskUsage()
	printGPUMemory()
}

func printLlamaServerDetail(procs []processInfo) {
	var servers []processInfo
	for _, p := range procs {
		if strings.HasPrefix(p.Component, "llama-server") && p.Status == "active" {
			servers = append(servers, p)
		}
	}
	if len(servers) == 0 {
		return
	}

	tbl := ui.NewTable("llama-server instances", "PROFILE", "TYPE", "PORT", "HEALTH", "MANAGED", "DETAILS")
	for _, s := range servers {
		health := ui.ErrorStyle.Render(s.Health)
		if s.Health == "healthy" {
			health = ui.SuccessStyle.Render(s.Health)
		}
		tbl.AddRow(s.Profile, s.ModelType, s.Port, health, s.Managed, s.Extra)
	}
	tbl.Print()
}

func gatherStatus() []processInfo {
	var procs []processInfo
	procs = append(procs, checkOllama())
	procs = append(procs, checkLlamaServers()...)
	procs = append(procs, checkPi())
	procs = append(procs, checkContainer("hermes", "Hermes gateway")...)
	procs = append(procs, checkContainer("hermes-dashboard", "Hermes dashboard")...)
	procs = append(procs, checkContainer("hermes-searxng", "SearXNG")...)
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

func checkLlamaServers() []processInfo {
	llamaBin := config.ExpandHome(viper.GetString("llama_server.bin"))

	ctx := context.Background()
	out, err := exec.RunCapture(ctx, "pgrep", []string{"-a", "llama-server"}, exec.RunOpts{})
	if err != nil || strings.TrimSpace(out) == "" {
		return []processInfo{{Component: "llama-server", Status: "stopped", PID: "-", Port: "-", Model: "-", Extra: "-"}}
	}

	var procs []processInfo
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		if !strings.Contains(line, llamaBin) {
			continue
		}

		p := processInfo{
			Component: "llama-server",
			Status:    "active",
			PID:       "-",
			Port:      "-",
			Model:     "-",
			Profile:   "-",
			ModelType: "-",
			Managed:   "process",
			Health:    "unknown",
			Extra:     "-",
		}
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

			p.Profile, p.ModelType = resolveProfile(p.Model)

			p.Managed = detectManagement(p.Port)

			var details []string
			mmproj := extractFlag(args, "--mmproj")
			if mmproj != "" {
				details = append(details, "vision")
			}
			ctxSize := extractFlag(args, "--ctx-size")
			if ctxSize != "" {
				details = append(details, "ctx:"+ctxSize)
			}
			cacheK := extractFlag(args, "--cache-type-k")
			if cacheK != "" {
				details = append(details, "kv:"+cacheK)
			}
			if len(details) > 0 {
				p.Extra = strings.Join(details, " ")
			}

			if port != "" {
				p.Health = checkHealth(port)
			}
		}
		procs = append(procs, p)
	}

	if len(procs) == 0 {
		return []processInfo{{Component: "llama-server", Status: "stopped", PID: "-", Port: "-", Model: "-", Extra: "-"}}
	}
	return procs
}

func resolveProfile(modelFilename string) (profile, modelType string) {
	if modelFilename == "" || modelFilename == "-" {
		return "-", "-"
	}
	profiles := viper.GetStringMap("profiles")
	for name := range profiles {
		m := viper.GetString(fmt.Sprintf("profiles.%s.model", name))
		if m == modelFilename {
			t := viper.GetString(fmt.Sprintf("profiles.%s.type", name))
			if t == "" {
				t = detectType(modelFilename)
			}
			return name, t
		}
	}
	return "-", detectType(modelFilename)
}

func detectType(modelName string) string {
	if strings.Contains(modelName, "-A") {
		for i := strings.Index(modelName, "-A") + 2; i < len(modelName); i++ {
			if modelName[i] >= '0' && modelName[i] <= '9' {
				continue
			}
			if modelName[i] == 'B' {
				return "moe"
			}
			break
		}
	}
	return "dense"
}

func detectManagement(port string) string {
	if port == "" || port == "-" {
		return "process"
	}
	unitName := fmt.Sprintf("auriga-llama-server-%s.service", port)
	ctx := context.Background()
	out, err := exec.RunCapture(ctx, "systemctl", []string{"--user", "is-active", unitName}, exec.RunOpts{})
	if err == nil && strings.TrimSpace(out) == "active" {
		return "systemd"
	}

	pidFile := fmt.Sprintf("/tmp/auriga-llama-server-%s.pid", port)
	if _, err := os.Stat(pidFile); err == nil {
		return "pid-file"
	}
	return "process"
}

func checkHealth(port string) string {
	host := viper.GetString("llama_server.host")
	parts := strings.Split(host, ":")
	var url string
	if len(parts) >= 3 {
		parts[len(parts)-1] = port
		url = strings.Join(parts, ":") + "/health"
	} else {
		url = fmt.Sprintf("http://localhost:%s/health", port)
	}

	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return "unreachable"
	}
	resp.Body.Close()
	if resp.StatusCode == 200 {
		return "healthy"
	}
	return fmt.Sprintf("http-%d", resp.StatusCode)
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

func checkContainer(name, label string) []processInfo {
	ctx := context.Background()
	// Try podman first, then docker
	for _, runtime := range []string{"podman", "docker"} {
		out, err := exec.RunCapture(ctx, runtime, []string{
			"inspect", "--format", "{{.State.Status}}:{{.State.Pid}}", name,
		}, exec.RunOpts{})
		if err != nil {
			continue
		}
		parts := strings.SplitN(strings.TrimSpace(out), ":", 2)
		status := parts[0]
		pid := "-"
		if len(parts) > 1 && parts[1] != "0" {
			pid = parts[1]
		}

		p := processInfo{
			Component: label,
			Status:    "stopped",
			PID:       pid,
			Port:      "-",
			Model:     "-",
			Extra:     runtime,
		}
		if status == "running" {
			p.Status = "active"
		}
		return []processInfo{p}
	}
	return nil
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
	if v := os.Getenv("OLLAMA_MODELS_DIR"); v != "" {
		return v
	}
	dir := viper.GetString("ollama.models_dir")
	if dir != "" {
		return config.ExpandHome(dir)
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".ollama", "models")
}

func findAMDCard() string {
	entries, err := filepath.Glob("/sys/class/drm/card*/device/vendor")
	if err != nil {
		return ""
	}
	for _, vendorPath := range entries {
		data, err := os.ReadFile(vendorPath)
		if err != nil {
			continue
		}
		if strings.TrimSpace(string(data)) == "0x1002" {
			return filepath.Dir(vendorPath)
		}
	}
	return ""
}

func printGPUMemory() {
	ctx := context.Background()

	cardDevice := findAMDCard()
	if cardDevice != "" {
		vramUsed, err := os.ReadFile(filepath.Join(cardDevice, "mem_info_vram_used"))
		vramTotal, err2 := os.ReadFile(filepath.Join(cardDevice, "mem_info_vram_total"))
		gttUsed, err3 := os.ReadFile(filepath.Join(cardDevice, "mem_info_gtt_used"))
		gttTotal, err4 := os.ReadFile(filepath.Join(cardDevice, "mem_info_gtt_total"))

		if err == nil && err2 == nil && err3 == nil && err4 == nil {
			tbl := ui.NewTable("GPU Memory", "TYPE", "USED", "TOTAL")
			tbl.AddRow("VRAM", formatBytesStr(vramUsed), formatBytesStr(vramTotal))
			tbl.AddRow("GTT", formatBytesStr(gttUsed), formatBytesStr(gttTotal))
			tbl.Print()
			return
		}
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
