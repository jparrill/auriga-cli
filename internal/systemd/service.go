package systemd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/jparrill/auriga-cli/internal/exec"
)

type ServiceConfig struct {
	ProfileName string
	ExecStart   string
	Environment []string
}

func UnitNameForPort(port int) string {
	return fmt.Sprintf("auriga-llama-server-%d.service", port)
}

func GenerateUnit(cfg ServiceConfig) string {
	var sb strings.Builder
	sb.WriteString("[Unit]\n")
	fmt.Fprintf(&sb, "Description=Auriga llama-server (profile: %s)\n", cfg.ProfileName)
	sb.WriteString("After=network.target\n\n")

	sb.WriteString("[Service]\n")
	sb.WriteString("Type=simple\n")
	fmt.Fprintf(&sb, "ExecStart=%s\n", cfg.ExecStart)
	for _, env := range cfg.Environment {
		fmt.Fprintf(&sb, "Environment=%s\n", env)
	}
	sb.WriteString("Restart=on-failure\n")
	sb.WriteString("RestartSec=5\n")
	sb.WriteString("StandardOutput=journal\n")
	sb.WriteString("StandardError=journal\n\n")

	sb.WriteString("[Install]\n")
	sb.WriteString("WantedBy=default.target\n")

	return sb.String()
}

func UnitDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine home directory: %w", err)
	}
	return filepath.Join(home, ".config", "systemd", "user"), nil
}

func UnitPathForPort(port int) (string, error) {
	dir, err := UnitDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, UnitNameForPort(port)), nil
}

func Install(port int, content string) error {
	dir, err := UnitDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("cannot create systemd user dir: %w", err)
	}
	path := filepath.Join(dir, UnitNameForPort(port))
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return fmt.Errorf("cannot write service file: %w", err)
	}
	return DaemonReload()
}

func DaemonReload() error {
	ctx := context.Background()
	_, err := exec.RunCapture(ctx, "systemctl", []string{"--user", "daemon-reload"}, exec.RunOpts{})
	return err
}

func Enable(port int) error {
	ctx := context.Background()
	_, err := exec.RunCapture(ctx, "systemctl", []string{"--user", "enable", UnitNameForPort(port)}, exec.RunOpts{})
	return err
}

func Start(port int) error {
	ctx := context.Background()
	_, err := exec.RunCapture(ctx, "systemctl", []string{"--user", "start", UnitNameForPort(port)}, exec.RunOpts{})
	return err
}

func Stop(port int) error {
	ctx := context.Background()
	_, err := exec.RunCapture(ctx, "systemctl", []string{"--user", "stop", UnitNameForPort(port)}, exec.RunOpts{})
	return err
}

func Disable(port int) error {
	ctx := context.Background()
	_, err := exec.RunCapture(ctx, "systemctl", []string{"--user", "disable", UnitNameForPort(port)}, exec.RunOpts{})
	return err
}

func IsActiveOnPort(port int) bool {
	ctx := context.Background()
	out, err := exec.RunCapture(ctx, "systemctl", []string{"--user", "is-active", UnitNameForPort(port)}, exec.RunOpts{})
	if err != nil {
		return false
	}
	return strings.TrimSpace(out) == "active"
}

func EnableLinger() error {
	user := os.Getenv("USER")
	if user == "" {
		return fmt.Errorf("USER environment variable not set")
	}
	ctx := context.Background()
	_, err := exec.RunCapture(ctx, "loginctl", []string{"enable-linger", user}, exec.RunOpts{})
	return err
}
