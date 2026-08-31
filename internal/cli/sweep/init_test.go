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
		t.Error("cache-type-k should NOT be in standalone params (alias target)")
	}
	if _, ok := params["batch-size"]; ok {
		t.Error("batch-size should NOT be in standalone params (alias target)")
	}
	if _, ok := params["threads"]; !ok {
		t.Error("expected threads in params")
	}
	if _, ok := params["unknown-flag"]; ok {
		t.Error("unknown-flag should not be in params")
	}
}

func TestBuildParameters_BatchAlias(t *testing.T) {
	t.Run("reads from batch-size flag", func(t *testing.T) {
		flagMap := map[string]string{"batch-size": "4096"}
		params := buildParameters(flagMap)
		batch := params["batch"]
		if len(batch) == 0 {
			t.Fatal("expected batch in params")
		}
		if batch[0] != "4096" {
			t.Errorf("batch first value should be current 4096, got %q", batch[0])
		}
	})

	t.Run("no current value uses defaults", func(t *testing.T) {
		flagMap := map[string]string{}
		params := buildParameters(flagMap)
		batch := params["batch"]
		if len(batch) < 2 {
			t.Error("should have at least 2 batch alternatives")
		}
	})

	t.Run("batch-size and ubatch-size not in standalone params", func(t *testing.T) {
		flagMap := map[string]string{"batch-size": "4096", "ubatch-size": "1024"}
		params := buildParameters(flagMap)
		if _, ok := params["batch-size"]; ok {
			t.Error("batch-size should not be a standalone param")
		}
		if _, ok := params["ubatch-size"]; ok {
			t.Error("ubatch-size should not be a standalone param")
		}
	})
}

func TestBuildParameters_CacheTypeAlias(t *testing.T) {
	t.Run("reads from cache-type-k flag", func(t *testing.T) {
		flagMap := map[string]string{"cache-type-k": "q8_0"}
		params := buildParameters(flagMap)
		ct := params["cache-type"]
		if len(ct) == 0 {
			t.Fatal("expected cache-type in params")
		}
		if ct[0] != "q8_0" {
			t.Errorf("cache-type first value should be current q8_0, got %q", ct[0])
		}
	})

	t.Run("no current value uses defaults", func(t *testing.T) {
		flagMap := map[string]string{}
		params := buildParameters(flagMap)
		ct := params["cache-type"]
		if len(ct) < 2 {
			t.Error("should have at least 2 cache type alternatives")
		}
	})

	t.Run("cache-type-k and cache-type-v not in standalone params", func(t *testing.T) {
		flagMap := map[string]string{"cache-type-k": "q4_0", "cache-type-v": "q4_0"}
		params := buildParameters(flagMap)
		if _, ok := params["cache-type-k"]; ok {
			t.Error("cache-type-k should not be a standalone param")
		}
		if _, ok := params["cache-type-v"]; ok {
			t.Error("cache-type-v should not be a standalone param")
		}
	})
}

func TestIsAliasTarget(t *testing.T) {
	alias, ok := isAliasTarget("cache-type-k")
	if !ok || alias != "cache-type" {
		t.Errorf("cache-type-k should be alias target of cache-type, got %q %v", alias, ok)
	}

	_, ok = isAliasTarget("threads")
	if ok {
		t.Error("threads should not be an alias target")
	}
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
