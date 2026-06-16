package llamaserver

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
	"github.com/spf13/viper"
)

type chatRequest struct {
	Model    string        `json:"model"`
	Messages []chatMessage `json:"messages"`
	MaxTokens int          `json:"max_tokens"`
	Temperature float64    `json:"temperature"`
	Stream   bool          `json:"stream"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

func Host() string {
	return viper.GetString("llama_server.host")
}

func Port() int {
	return viper.GetInt("llama_server.port")
}

func Bin() string {
	return config.ExpandHome(viper.GetString("llama_server.bin"))
}

func GGUFDir() string {
	return config.ExpandHome(viper.GetString("llama_server.gguf_dir"))
}

func MMProjDir() string {
	return config.ExpandHome(viper.GetString("llama_server.mmproj_dir"))
}

func ConfiguredModels() []string {
	raw := viper.GetString("llama_server.models")
	if raw == "" {
		return nil
	}
	return strings.Fields(raw)
}

func FindLocalGGUF(hfRepo string) string {
	repoName := filepath.Base(hfRepo)
	repoName = strings.TrimSuffix(repoName, "-GGUF")
	dir := GGUFDir()

	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".gguf") {
			if strings.Contains(strings.ToLower(e.Name()), strings.ToLower(repoName)) {
				return filepath.Join(dir, e.Name())
			}
		}
	}
	return ""
}

func StopOllama(ctx context.Context) {
	ui.Info("Stopping Ollama to free GPU...")
	exec.RunCapture(ctx, "sudo", []string{"systemctl", "stop", "ollama"}, exec.RunOpts{})
	time.Sleep(2 * time.Second)
}

func StartOllama(ctx context.Context) {
	ui.Info("Restarting Ollama...")
	exec.RunCapture(ctx, "sudo", []string{"systemctl", "start", "ollama"}, exec.RunOpts{})
	ui.Ok("Ollama restarted")
}

func Start(ctx context.Context, modelPath string, mmprojPath string, extraFlags []string) (*os.Process, error) {
	bin := Bin()
	port := Port()

	if _, err := os.Stat(bin); err != nil {
		return nil, fmt.Errorf("llama-server binary not found: %s", bin)
	}

	StopOllama(ctx)

	args := []string{
		"-m", modelPath,
		"--host", "0.0.0.0",
		"--port", fmt.Sprintf("%d", port),
		"--flash-attn", "on",
		"--gpu-layers", "99",
		"--ctx-size", "65536",
	}

	if mmprojPath != "" {
		args = append(args, "--mmproj", mmprojPath, "--jinja")
	}
	args = append(args, extraFlags...)

	ui.Info(fmt.Sprintf("Starting llama-server with %s", filepath.Base(modelPath)))
	ui.Logger.Debug("cmd", "bin", bin, "args", strings.Join(args, " "))

	logFile, _ := os.Create("/tmp/llama-server-auriga.log")

	attr := &os.ProcAttr{
		Dir: "/tmp",
		Env: append(os.Environ(), "AMD_VULKAN_ICD=RADV"),
		Files: []*os.File{os.Stdin, logFile, logFile},
	}

	proc, err := os.StartProcess(bin, append([]string{bin}, args...), attr)
	if err != nil {
		logFile.Close()
		return nil, fmt.Errorf("failed to start llama-server: %w", err)
	}

	if err := WaitForHealth(90 * time.Second); err != nil {
		logFile.Close()
		proc.Kill()
		return nil, err
	}

	ui.Ok(fmt.Sprintf("llama-server ready on port %d (PID %d)", port, proc.Pid))
	return proc, nil
}

func Stop(proc *os.Process) {
	if proc != nil {
		ui.Info("Stopping llama-server...")
		proc.Kill()
		proc.Wait()
	}
	time.Sleep(2 * time.Second)
	StartOllama(context.Background())
}

func WaitForHealth(timeout time.Duration) error {
	url := fmt.Sprintf("%s/health", Host())
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		resp, err := http.Get(url)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == 200 {
				return nil
			}
		}
		time.Sleep(time.Second)
	}
	return fmt.Errorf("llama-server health check timeout after %s", timeout)
}

func Generate(prompt string, maxTokens int, timeout time.Duration) (string, error) {
	payload := chatRequest{
		Model: "local",
		Messages: []chatMessage{
			{Role: "user", Content: prompt},
		},
		MaxTokens:   maxTokens,
		Temperature: 0.3,
		Stream:      false,
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	client := &http.Client{Timeout: timeout}
	resp, err := client.Post(Host()+"/v1/chat/completions", "application/json", strings.NewReader(string(data)))
	if err != nil {
		return "", fmt.Errorf("llama-server call failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("llama-server returned %d", resp.StatusCode)
	}

	var chatResp chatResponse
	if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
		return "", fmt.Errorf("invalid response: %w", err)
	}

	if len(chatResp.Choices) == 0 {
		return "", fmt.Errorf("empty response from llama-server")
	}
	return chatResp.Choices[0].Message.Content, nil
}
