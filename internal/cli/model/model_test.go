package model

import (
	"testing"
)

func TestNewModelCmd(t *testing.T) {
	cmd := NewModelCmd()
	if cmd.Name() != "model" {
		t.Errorf("expected 'model', got %q", cmd.Name())
	}

	subs := make(map[string]bool)
	for _, c := range cmd.Commands() {
		subs[c.Name()] = true
	}

	for _, name := range []string{"list", "ensure", "create"} {
		if !subs[name] {
			t.Errorf("expected subcommand %q", name)
		}
	}
}
