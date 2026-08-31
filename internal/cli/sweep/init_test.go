package sweep

import (
	"testing"

	"github.com/spf13/viper"
)

func TestParseFlagPairs(t *testing.T) {
	tests := []struct {
		name  string
		flags []string
		want  map[string]string
	}{
		{
			name:  "standard flags",
			flags: []string{"--cache-type-k", "q4_0", "--threads", "8", "-np", "1"},
			want:  map[string]string{"cache-type-k": "q4_0", "threads": "8", "np": "1"},
		},
		{
			name:  "empty flags",
			flags: []string{},
			want:  map[string]string{},
		},
		{
			name:  "single flag pair",
			flags: []string{"--batch-size", "2048"},
			want:  map[string]string{"batch-size": "2048"},
		},
		{
			name:  "trailing flag without value",
			flags: []string{"--threads", "8", "--verbose"},
			want:  map[string]string{"threads": "8"},
		},
		{
			name:  "equals syntax",
			flags: []string{"--cache-type-k=q4_0", "--threads", "8"},
			want:  map[string]string{"cache-type-k": "q4_0", "threads": "8"},
		},
		{
			name:  "mixed equals and pairs",
			flags: []string{"--batch-size=2048", "-np", "1", "--load-mode=mlock"},
			want:  map[string]string{"batch-size": "2048", "np": "1", "load-mode": "mlock"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseFlagPairs(tt.flags)
			if len(got) != len(tt.want) {
				t.Fatalf("got %d entries, want %d: %v", len(got), len(tt.want), got)
			}
			for k, wantV := range tt.want {
				if got[k] != wantV {
					t.Errorf("key %q: got %q, want %q", k, got[k], wantV)
				}
			}
		})
	}
}

func TestBuildProfileFields(t *testing.T) {
	viper.Reset()

	t.Run("profile bin and global bin differ", func(t *testing.T) {
		viper.Set("profiles.test.bin", "/usr/bin/llama-server-fork")
		viper.Set("llama_server.bin", "/usr/bin/llama-server")
		defer viper.Reset()

		fields := buildProfileFields("test")
		bins := fields["bin"]
		if len(bins) != 2 {
			t.Fatalf("expected 2 bin entries, got %d: %v", len(bins), bins)
		}
		if bins[0] != "/usr/bin/llama-server-fork" {
			t.Errorf("first bin should be profile override, got %q", bins[0])
		}
		if bins[1] != "/usr/bin/llama-server" {
			t.Errorf("second bin should be global, got %q", bins[1])
		}
	})

	t.Run("only global bin", func(t *testing.T) {
		viper.Set("llama_server.bin", "/usr/bin/llama-server")
		defer viper.Reset()

		fields := buildProfileFields("test")
		bins := fields["bin"]
		if len(bins) != 1 {
			t.Fatalf("expected 1 bin entry, got %d", len(bins))
		}
	})

	t.Run("same bin in profile and global", func(t *testing.T) {
		viper.Set("profiles.test.bin", "/usr/bin/llama-server")
		viper.Set("llama_server.bin", "/usr/bin/llama-server")
		defer viper.Reset()

		fields := buildProfileFields("test")
		bins := fields["bin"]
		if len(bins) != 1 {
			t.Fatalf("expected 1 bin entry when same, got %d", len(bins))
		}
	})

	t.Run("no bin configured", func(t *testing.T) {
		viper.Reset()
		fields := buildProfileFields("test")
		if len(fields) != 0 {
			t.Fatalf("expected empty fields, got %v", fields)
		}
	})
}

func TestBuildParameters(t *testing.T) {
	flagMap := map[string]string{
		"cache-type-k": "q4_0",
		"threads":      "8",
		"unknown-flag": "value",
	}

	params := buildParameters(flagMap)

	if _, ok := params["cache-type-k"]; ok {
		t.Error("cache-type-k should NOT be in standalone params (it belongs in linked)")
	}
	if _, ok := params["batch-size"]; ok {
		t.Error("batch-size should NOT be in standalone params (it belongs in linked)")
	}
	if _, ok := params["threads"]; !ok {
		t.Error("expected threads in params")
	}
	if _, ok := params["unknown-flag"]; ok {
		t.Error("unknown-flag should not be in params")
	}
}

func TestBuildLinkedParameters(t *testing.T) {
	t.Run("with current cache and batch values", func(t *testing.T) {
		flagMap := map[string]string{
			"cache-type-k": "q8_0",
			"cache-type-v": "q8_0",
			"batch-size":   "4096",
			"ubatch-size":  "1024",
		}
		linked := buildLinkedParameters(flagMap)

		ct := linked["cache-type"]
		if ct["cache-type-k"][0] != "q8_0" {
			t.Errorf("cache-type-k first value should be current q8_0, got %q", ct["cache-type-k"][0])
		}
		if len(ct["cache-type-k"]) != len(ct["cache-type-v"]) {
			t.Error("cache-type-k and cache-type-v must have same length")
		}
		for i := range ct["cache-type-k"] {
			if ct["cache-type-k"][i] != ct["cache-type-v"][i] {
				t.Errorf("index %d: k=%s v=%s — linked values must match", i, ct["cache-type-k"][i], ct["cache-type-v"][i])
			}
		}

		batch := linked["batch"]
		if batch["batch-size"][0] != "4096" || batch["ubatch-size"][0] != "1024" {
			t.Errorf("first batch pair should be current (4096,1024), got (%s,%s)", batch["batch-size"][0], batch["ubatch-size"][0])
		}
		if len(batch["batch-size"]) != len(batch["ubatch-size"]) {
			t.Error("batch-size and ubatch-size must have same length")
		}
	})

	t.Run("without current values uses defaults", func(t *testing.T) {
		flagMap := map[string]string{}
		linked := buildLinkedParameters(flagMap)

		ct := linked["cache-type"]
		if len(ct["cache-type-k"]) < 2 {
			t.Error("should have at least 2 cache type alternatives")
		}

		batch := linked["batch"]
		if len(batch["batch-size"]) < 2 {
			t.Error("should have at least 2 batch presets")
		}
	})

	t.Run("cache type k and v always identical", func(t *testing.T) {
		flagMap := map[string]string{"cache-type-k": "f16"}
		linked := buildLinkedParameters(flagMap)

		ct := linked["cache-type"]
		for i := range ct["cache-type-k"] {
			if ct["cache-type-k"][i] != ct["cache-type-v"][i] {
				t.Errorf("index %d: k=%s v=%s — must always match", i, ct["cache-type-k"][i], ct["cache-type-v"][i])
			}
		}
	})
}

func TestBuildToggles(t *testing.T) {
	t.Run("np present", func(t *testing.T) {
		flagMap := map[string]string{"np": "1"}
		toggles := buildToggles(flagMap)
		np := toggles["np"]
		if len(np) != 2 || np[0] != "on" || np[1] != "off" {
			t.Errorf("unexpected np toggle: %v", np)
		}
	})

	t.Run("load-mode present", func(t *testing.T) {
		flagMap := map[string]string{"load-mode": "mlock"}
		toggles := buildToggles(flagMap)
		lm := toggles["load-mode"]
		if len(lm) != 2 || lm[0] != "mlock" || lm[1] != "off" {
			t.Errorf("unexpected load-mode toggle: %v", lm)
		}
	})

	t.Run("load-mode absent not included", func(t *testing.T) {
		flagMap := map[string]string{}
		toggles := buildToggles(flagMap)
		if _, ok := toggles["load-mode"]; ok {
			t.Error("load-mode should not be included when absent from flags")
		}
	})
}
