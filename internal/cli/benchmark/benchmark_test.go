package benchmark

import (
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

func TestRunBenchmarkRun_SlotResolvesHost(t *testing.T) {
	viper.Reset()
	viper.Set("llama_server.slot_1_port", 8090)
	viper.Set("llama_server.slot_2_port", 8091)
	viper.Set("llama_server.host", "http://localhost:8090")
	defer viper.Reset()

	tests := []struct {
		name         string
		slot         int
		wantHost     string
		wantBackend  string
		wantErr      bool
	}{
		{"slot 1", 1, "http://localhost:8090", "llama-server", false},
		{"slot 2", 2, "http://localhost:8091", "llama-server", false},
		{"invalid slot", 3, "", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := &runOpts{
				Backend: "all",
				Slot:    tt.slot,
			}
			err := runBenchmarkRun(opts)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error for invalid slot")
				}
				return
			}
			// runBenchmarkRun will fail later (no results dir, etc.) but
			// slot resolution happens first — verify opts were mutated
			if opts.Host != tt.wantHost {
				t.Errorf("host: got %q, want %q", opts.Host, tt.wantHost)
			}
			if opts.Backend != tt.wantBackend {
				t.Errorf("backend: got %q, want %q", opts.Backend, tt.wantBackend)
			}
		})
	}
}

func TestRunBenchmarkRun_NoSlotKeepsOriginal(t *testing.T) {
	opts := &runOpts{
		Backend: "ollama",
		Host:    "http://custom:9999",
		Slot:    0,
	}
	// Slot 0 means not set — should not override host/backend
	_ = runBenchmarkRun(opts)
	if opts.Host != "http://custom:9999" {
		t.Errorf("host changed unexpectedly: %q", opts.Host)
	}
	if opts.Backend != "ollama" {
		t.Errorf("backend changed unexpectedly: %q", opts.Backend)
	}
}
