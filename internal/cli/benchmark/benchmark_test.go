package benchmark

import (
	"fmt"
	"testing"

	"github.com/spf13/viper"
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

func TestRunBenchmarkRun_SlotValidation(t *testing.T) {
	viper.Reset()
	viper.Set("llama_server.slot_1_port", 8090)
	viper.Set("llama_server.slot_2_port", 8091)
	viper.Set("llama_server.host", "http://localhost:8090")
	defer viper.Reset()

	tests := []struct {
		name    string
		slot    int
		wantErr bool
	}{
		{"valid slot 1", 1, false},
		{"valid slot 2", 2, false},
		{"invalid slot 3", 3, true},
		{"invalid slot 0", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := &runOpts{
				Slot: tt.slot,
			}
			err := runBenchmarkRun(opts)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error for invalid slot")
				}
				return
			}
			// Valid slots will fail later (no server running) but slot
			// validation and host resolution happen first — no error at
			// the validation stage means slot resolution worked
			if err != nil && err.Error() == fmt.Sprintf("--slot must be 1 or 2, got %d", tt.slot) {
				t.Fatalf("unexpected slot validation error: %v", err)
			}
		})
	}
}

func TestRunBenchmarkRunCmd_SlotRequired(t *testing.T) {
	cmd := newBenchmarkRunCmd()
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error when --slot not provided")
	}
}
