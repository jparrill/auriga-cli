package show

import (
	"context"
	"fmt"
	"net"
	"strings"

	"github.com/jparrill/auriga-cli/internal/exec"
	"github.com/jparrill/auriga-cli/internal/llamaserver"
	"github.com/jparrill/auriga-cli/internal/ui"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func newShowConfigCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "config",
		Short: "Show connection URLs for all services",
		Long: `Display connection details for Ollama and llama-server instances,
including LAN, Tailscale, and localhost URLs. Useful for configuring
OpenCode or other clients.

Examples:
  auriga show config`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runShowConfig()
		},
	}
}

type endpoint struct {
	Service string
	Type    string
	Port    string
	Local   string
	LAN     string
	TS      string
}

func runShowConfig() error {
	lanIP := detectLANIP()
	tsIP := detectTailscaleIP()

	var endpoints []endpoint

	ollamaPort := extractPort(viper.GetString("ollama.host"), "11434")
	endpoints = append(endpoints, endpoint{
		Service: "ollama",
		Type:    "-",
		Port:    ollamaPort,
		Local:   fmt.Sprintf("http://localhost:%s", ollamaPort),
		LAN:     formatURL(lanIP, ollamaPort),
		TS:      formatURL(tsIP, ollamaPort),
	})

	slotPorts := llamaserver.AllSlotPorts()
	customPorts := map[string]bool{}
	for i, sp := range slotPorts {
		ps := fmt.Sprintf("%d", sp)
		customPorts[ps] = true
		endpoints = append(endpoints, endpoint{
			Service: "llama-server",
			Type:    fmt.Sprintf("slot-%d", i+1),
			Port:    ps,
			Local:   fmt.Sprintf("http://localhost:%s", ps),
			LAN:     formatURL(lanIP, ps),
			TS:      formatURL(tsIP, ps),
		})
	}

	slot1Port := fmt.Sprintf("%d", slotPorts[0])
	slot2Port := fmt.Sprintf("%d", slotPorts[1])

	profiles := viper.GetStringMap("profiles")
	for name := range profiles {
		p := viper.GetInt(fmt.Sprintf("profiles.%s.port", name))
		if p > 0 {
			ps := fmt.Sprintf("%d", p)
			if !customPorts[ps] {
				customPorts[ps] = true
				endpoints = append(endpoints, endpoint{
					Service: fmt.Sprintf("llama-server (%s)", name),
					Type:    "custom",
					Port:    ps,
					Local:   fmt.Sprintf("http://localhost:%s", ps),
					LAN:     formatURL(lanIP, ps),
					TS:      formatURL(tsIP, ps),
				})
			}
		}
	}

	tbl := ui.NewTable("Connection URLs", "SERVICE", "TYPE", "PORT", "LOCALHOST", "LAN", "TAILSCALE")
	for _, e := range endpoints {
		tbl.AddRow(e.Service, e.Type, e.Port, e.Local, e.LAN, e.TS)
	}
	tbl.Print()

	fmt.Println()
	printProfileDetails()
	fmt.Println()
	printAPIEndpoints(slot1Port, slot2Port, lanIP, tsIP)
	fmt.Println()
	printClientExamples(slot1Port, slot2Port, lanIP, tsIP)

	return nil
}

func printProfileDetails() {
	profiles := viper.GetStringMap("profiles")
	if len(profiles) == 0 {
		return
	}

	tbl := ui.NewTable("Profile Configuration", "PROFILE", "TYPE", "PORT", "MODEL", "VISION", "FLAGS")
	for name := range profiles {
		key := fmt.Sprintf("profiles.%s", name)
		model := viper.GetString(key + ".model")
		mmproj := viper.GetString(key + ".mmproj")
		pType := viper.GetString(key + ".type")
		if pType == "" {
			pType = detectModelTypeShow(model)
		}

		port := viper.GetInt(key + ".port")
		if port == 0 {
			port = llamaserver.Slot1Port()
		}

		vision := "no"
		if mmproj != "" {
			vision = "yes"
		}

		flags := viper.GetStringSlice(key + ".flags")
		flagSummary := "-"
		if len(flags) > 0 {
			flagSummary = summarizeFlags(flags)
		}

		tbl.AddRow(name, pType, fmt.Sprintf("%d", port), model, vision, flagSummary)
	}
	tbl.Print()
}

func summarizeFlags(flags []string) string {
	var parts []string
	for i := 0; i < len(flags); i++ {
		switch flags[i] {
		case "--cache-type-k":
			if i+1 < len(flags) {
				parts = append(parts, "kv:"+flags[i+1])
				i++
			}
		case "--cache-type-v":
			i++ // skip value, already shown via cache-type-k
		case "--batch-size":
			if i+1 < len(flags) {
				parts = append(parts, "batch:"+flags[i+1])
				i++
			}
		case "--ubatch-size":
			i++ // skip, implied by batch
		case "--threads":
			if i+1 < len(flags) {
				parts = append(parts, "threads:"+flags[i+1])
				i++
			}
		case "--jinja":
			parts = append(parts, "jinja")
		case "--ctx-size":
			if i+1 < len(flags) {
				parts = append(parts, "ctx:"+flags[i+1])
				i++
			}
		default:
			parts = append(parts, flags[i])
		}
	}
	if len(parts) == 0 {
		return "-"
	}
	return strings.Join(parts, " ")
}

func detectModelTypeShow(modelName string) string {
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

func printAPIEndpoints(slot1Port, slot2Port, lanIP, tsIP string) {
	host := bestRemoteIP(lanIP, tsIP)

	tbl := ui.NewTable("OpenAI-Compatible API", "SERVICE", "ENDPOINT")
	tbl.AddRow("Ollama", fmt.Sprintf("http://%s:11434/v1", host))
	tbl.AddRow("llama-server (slot-1)", fmt.Sprintf("http://%s:%s/v1", host, slot1Port))
	if slot1Port != slot2Port {
		tbl.AddRow("llama-server (slot-2)", fmt.Sprintf("http://%s:%s/v1", host, slot2Port))
	}
	tbl.Print()
}

func printClientExamples(slot1Port, slot2Port, lanIP, tsIP string) {
	host := bestRemoteIP(lanIP, tsIP)

	ui.Info("OpenCode / other clients:")
	fmt.Printf("  # Ollama (OpenAI-compatible)\n")
	fmt.Printf("  OPENAI_API_BASE=http://%s:11434/v1 OPENAI_API_KEY=unused\n", host)
	fmt.Printf("  # llama-server slot-1\n")
	fmt.Printf("  OPENAI_API_BASE=http://%s:%s/v1 OPENAI_API_KEY=unused\n", host, slot1Port)
	if slot1Port != slot2Port {
		fmt.Printf("  # llama-server slot-2\n")
		fmt.Printf("  OPENAI_API_BASE=http://%s:%s/v1 OPENAI_API_KEY=unused\n", host, slot2Port)
	}
}

func detectLANIP() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "-"
	}
	for _, addr := range addrs {
		if ipNet, ok := addr.(*net.IPNet); ok && !ipNet.IP.IsLoopback() && ipNet.IP.To4() != nil {
			ip := ipNet.IP.String()
			if strings.HasPrefix(ip, "192.168.") || strings.HasPrefix(ip, "10.") || strings.HasPrefix(ip, "172.") {
				return ip
			}
		}
	}
	return "-"
}

func detectTailscaleIP() string {
	ctx := context.Background()
	out, err := exec.RunCapture(ctx, "tailscale", []string{"ip", "-4"}, exec.RunOpts{})
	if err != nil {
		return "-"
	}
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "Warning") {
			continue
		}
		if net.ParseIP(line) != nil {
			return line
		}
	}
	return "-"
}

func extractPort(hostURL, fallback string) string {
	parts := strings.Split(hostURL, ":")
	if len(parts) >= 3 {
		port := strings.TrimRight(parts[len(parts)-1], "/")
		if port != "" {
			return port
		}
	}
	return fallback
}

func formatURL(ip, port string) string {
	if ip == "-" {
		return "-"
	}
	return fmt.Sprintf("http://%s:%s", ip, port)
}

func bestRemoteIP(lanIP, tsIP string) string {
	if tsIP != "-" {
		return tsIP
	}
	if lanIP != "-" {
		return lanIP
	}
	return "localhost"
}
