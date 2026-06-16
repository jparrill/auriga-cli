package benchmark

import (
	"context"
	"fmt"
	"os"
	goexec "os/exec"
	"path/filepath"
	"time"

	"github.com/jparrill/auriga-cli/internal/ui"
)

func ValidateBuild(projectDir string) (bool, string) {
	pkg := filepath.Join(projectDir, "package.json")
	if _, err := os.Stat(pkg); err != nil {
		return false, "No package.json found"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	ui.Info("Running npm install...")
	out, err := captureCmd(ctx, projectDir, "npm", "install", "--legacy-peer-deps")
	if err != nil {
		return false, fmt.Sprintf("npm install failed:\n%s", truncate(out, 1000))
	}

	ui.Info("Running npm run build...")
	out, err = captureCmd(ctx, projectDir, "npm", "run", "build")
	if err != nil {
		return false, fmt.Sprintf("npm run build failed:\n%s", truncate(out, 1500))
	}

	ui.Ok("Build passed")
	return true, ""
}

func captureCmd(ctx context.Context, dir, name string, args ...string) (string, error) {
	cmd := goexec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[len(s)-max:]
}
