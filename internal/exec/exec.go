package exec

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/jparrill/auriga-cli/internal/config"
	"github.com/jparrill/auriga-cli/internal/ui"
)

type RunOpts struct {
	Dir    string
	Env    map[string]string
	DryRun bool
}

func Run(ctx context.Context, name string, args []string, opts RunOpts) (string, error) {
	cmdStr := name + " " + strings.Join(args, " ")
	dryRun := opts.DryRun || config.DryRun

	if dryRun {
		fmt.Println(ui.MutedStyle.Render("[dry-run]"), cmdStr)
		return "", nil
	}

	ui.Logger.Debug("exec", "cmd", cmdStr)

	cmd := exec.CommandContext(ctx, name, args...)
	if opts.Dir != "" {
		cmd.Dir = opts.Dir
	}
	cmd.Env = buildEnv(opts.Env)

	var stdout bytes.Buffer
	cmd.Stdout = io.MultiWriter(os.Stdout, &stdout)
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return stdout.String(), fmt.Errorf("command failed: %s: %w", cmdStr, err)
	}
	return stdout.String(), nil
}

func RunCapture(ctx context.Context, name string, args []string, opts RunOpts) (string, error) {
	cmdStr := name + " " + strings.Join(args, " ")
	dryRun := opts.DryRun || config.DryRun

	if dryRun {
		fmt.Println(ui.MutedStyle.Render("[dry-run]"), cmdStr)
		return "", nil
	}

	ui.Logger.Debug("exec", "cmd", cmdStr)

	cmd := exec.CommandContext(ctx, name, args...)
	if opts.Dir != "" {
		cmd.Dir = opts.Dir
	}
	cmd.Env = buildEnv(opts.Env)

	out, err := cmd.CombinedOutput()
	if err != nil {
		return string(out), fmt.Errorf("command failed: %s: %w\n%s", cmdStr, err, string(out))
	}
	return string(out), nil
}

func RunStreaming(ctx context.Context, name string, args []string, opts RunOpts) error {
	cmdStr := name + " " + strings.Join(args, " ")
	dryRun := opts.DryRun || config.DryRun

	if dryRun {
		fmt.Println(ui.MutedStyle.Render("[dry-run]"), cmdStr)
		return nil
	}

	ui.Logger.Debug("exec", "cmd", cmdStr)

	cmd := exec.CommandContext(ctx, name, args...)
	if opts.Dir != "" {
		cmd.Dir = opts.Dir
	}
	cmd.Env = buildEnv(opts.Env)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

func buildEnv(extra map[string]string) []string {
	if len(extra) == 0 {
		return nil
	}
	env := os.Environ()
	for k, v := range extra {
		env = append(env, k+"="+v)
	}
	return env
}
