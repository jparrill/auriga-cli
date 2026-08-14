package show

import (
	"os"
	"testing"

	"github.com/jparrill/auriga-cli/internal/ui"
	"github.com/spf13/viper"
)

func TestMain(m *testing.M) {
	ui.InitLogger(false)
	os.Exit(m.Run())
}

func TestExtractPort_FromURL(t *testing.T) {
	tests := []struct {
		name     string
		hostURL  string
		fallback string
		want     string
	}{
		{"When URL has port, it should extract it", "http://localhost:8090", "11434", "8090"},
		{"When URL has different port, it should extract it", "http://auriga:11434", "8090", "11434"},
		{"When URL has no port, it should use fallback", "http://localhost", "8090", "8090"},
		{"When URL is empty, it should use fallback", "", "8090", "8090"},
		{"When URL has trailing slash, it should strip it", "http://localhost:9000/", "8090", "9000"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractPort(tt.hostURL, tt.fallback)
			if got != tt.want {
				t.Errorf("extractPort(%q, %q) = %q, want %q", tt.hostURL, tt.fallback, got, tt.want)
			}
		})
	}
}

func TestFormatURL(t *testing.T) {
	tests := []struct {
		name string
		ip   string
		port string
		want string
	}{
		{"When IP available, it should format URL", "192.168.1.143", "8090", "http://192.168.1.143:8090"},
		{"When IP is dash, it should return dash", "-", "8090", "-"},
		{"When Tailscale IP, it should format URL", "100.77.65.108", "8091", "http://100.77.65.108:8091"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatURL(tt.ip, tt.port)
			if got != tt.want {
				t.Errorf("formatURL(%q, %q) = %q, want %q", tt.ip, tt.port, got, tt.want)
			}
		})
	}
}

func TestBestRemoteIP(t *testing.T) {
	tests := []struct {
		name  string
		lanIP string
		tsIP  string
		want  string
	}{
		{"When both available, it should prefer Tailscale", "192.168.1.143", "100.77.65.108", "100.77.65.108"},
		{"When only LAN, it should use LAN", "192.168.1.143", "-", "192.168.1.143"},
		{"When neither available, it should use localhost", "-", "-", "localhost"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := bestRemoteIP(tt.lanIP, tt.tsIP)
			if got != tt.want {
				t.Errorf("bestRemoteIP(%q, %q) = %q, want %q", tt.lanIP, tt.tsIP, got, tt.want)
			}
		})
	}
}

func TestNewShowConfigCmd_Registration(t *testing.T) {
	cmd := NewShowCmd()
	found := false
	for _, sub := range cmd.Commands() {
		if sub.Name() == "config" {
			found = true
			break
		}
	}
	if !found {
		t.Error("When listing show subcommands, config should be registered")
	}
}

func TestDetectLANIP_ReturnsNonEmpty(t *testing.T) {
	ip := detectLANIP()
	if ip == "" {
		t.Error("When detecting LAN IP, it should return something (even -)")
	}
}

func TestSummarizeFlags(t *testing.T) {
	tests := []struct {
		name  string
		flags []string
		want  string
	}{
		{
			"When full flags, it should summarize",
			[]string{"--cache-type-k", "q8_0", "--cache-type-v", "q8_0", "--batch-size", "2048", "--ubatch-size", "512", "--threads", "16"},
			"kv:q8_0 batch:2048 threads:16",
		},
		{
			"When jinja flag, it should show jinja",
			[]string{"--jinja"},
			"jinja",
		},
		{
			"When empty, it should return dash",
			[]string{},
			"-",
		},
		{
			"When ctx-size present, it should show ctx",
			[]string{"--ctx-size", "131072"},
			"ctx:131072",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := summarizeFlags(tt.flags)
			if got != tt.want {
				t.Errorf("summarizeFlags() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestDetectModelTypeShow(t *testing.T) {
	if detectModelTypeShow("Qwen3.6-35B-A3B-Q8_0.gguf") != "moe" {
		t.Error("When A3B model, should detect moe")
	}
	if detectModelTypeShow("Qwen3.6-27B-UD-Q8_K_XL.gguf") != "dense" {
		t.Error("When dense model, should detect dense")
	}
}

func TestRunShowConfig_NoProfiles(t *testing.T) {
	viper.Reset()
	defer viper.Reset()

	viper.Set("ollama.host", "http://localhost:11434")
	viper.Set("llama_server.host", "http://localhost:8090")
	viper.Set("llama_server.dense_port", 8090)
	viper.Set("llama_server.moe_port", 8091)

	err := runShowConfig()
	if err != nil {
		t.Errorf("When no profiles, runShowConfig should not error, got: %v", err)
	}
}
