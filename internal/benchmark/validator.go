package benchmark

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/jparrill/auriga-cli/internal/exec"
	"github.com/jparrill/auriga-cli/internal/ui"
)

func ValidateBuild(projectDir string) (bool, string) {
	pkg := filepath.Join(projectDir, "package.json")
	if _, err := os.Stat(pkg); err != nil {
		return false, "No package.json found"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	sandbox := exec.SandboxOpts{Dir: projectDir, Image: exec.ImageNode}

	ui.Info("Running npm install (sandboxed)...")
	out, err := exec.RunSandboxed(ctx, "npm", []string{"install", "--legacy-peer-deps"}, sandbox)
	if err != nil {
		return false, fmt.Sprintf("npm install failed:\n%s", truncate(out, 1000))
	}

	ui.Info("Running npm run build (sandboxed)...")
	out, err = exec.RunSandboxed(ctx, "npm", []string{"run", "build"}, sandbox)
	if err != nil {
		return false, fmt.Sprintf("npm run build failed:\n%s", truncate(out, 1500))
	}

	ui.Ok("Build passed")
	return true, ""
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[len(s)-max:]
}
