package cli

import (
	"bytes"
	"testing"
)

func TestRootCmd_Help(t *testing.T) {
	cmd := NewRootCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--help"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("root --help failed: %v", err)
	}

	output := buf.String()
	if output == "" {
		t.Error("expected help output, got empty")
	}
}

func TestVersionCmd(t *testing.T) {
	cmd := NewRootCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"version"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("version failed: %v", err)
	}
}

func TestRootCmd_SubcommandRegistration(t *testing.T) {
	cmd := NewRootCmd()
	expected := []string{"version", "profile", "model", "fix", "benchmark"}

	cmds := make(map[string]bool)
	for _, c := range cmd.Commands() {
		cmds[c.Name()] = true
	}

	for _, name := range expected {
		if !cmds[name] {
			t.Errorf("expected subcommand %q not found", name)
		}
	}
}
