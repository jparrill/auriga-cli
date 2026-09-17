package cli

import (
	"bytes"
	"strings"
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
	for _, want := range []string{
		"auriga profile serve qwen3.6-vision --slot 1",
		"auriga profile stop                           # Stop llama-server instances",
	} {
		if !strings.Contains(output, want) {
			t.Errorf("When root help is shown, it should contain %q", want)
		}
	}
	if strings.Contains(output, "restart Ollama") {
		t.Error("When root help is shown, it should not claim profile stop restarts Ollama")
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
	expected := []string{"version", "profile", "model", "benchmark", "ps", "show", "sweep"}

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

func TestRootCmd_ObsoleteCommandsAreNotRegistered(t *testing.T) {
	cmd := NewRootCmd()
	for _, name := range []string{"fix", "serve"} {
		t.Run("When obsolete command "+name+" is requested, it should be unavailable", func(t *testing.T) {
			for _, candidate := range cmd.Commands() {
				if candidate.Name() == name {
					t.Fatalf("obsolete command %q is registered", name)
				}
			}
		})
	}
}

func TestProfileLifecycleCommands_RequireSlot(t *testing.T) {
	root := NewRootCmd()
	for _, name := range []string{"serve", "switch"} {
		t.Run("When profile "+name+" has no slot, it should reject execution", func(t *testing.T) {
			cmd, _, err := root.Find([]string{"profile", name})
			if err != nil {
				t.Fatal(err)
			}
			if cmd.Flags().Lookup("slot") == nil {
				t.Fatalf("profile %s should define slot flag", name)
			}
			if err := cmd.ValidateRequiredFlags(); err == nil || !strings.Contains(err.Error(), "slot") {
				t.Fatalf("expected required slot error, got %v", err)
			}
		})
	}
}
