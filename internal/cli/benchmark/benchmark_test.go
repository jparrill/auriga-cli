package benchmark

import (
	"testing"
)

func TestNewBenchmarkCmd(t *testing.T) {
	cmd := NewBenchmarkCmd()
	if cmd.Name() != "benchmark" {
		t.Errorf("expected 'benchmark', got %q", cmd.Name())
	}

	subs := make(map[string]bool)
	for _, c := range cmd.Commands() {
		subs[c.Name()] = true
	}

	for _, name := range []string{"list", "run"} {
		if !subs[name] {
			t.Errorf("expected subcommand %q", name)
		}
	}
}
