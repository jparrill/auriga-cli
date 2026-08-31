package sweep

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

func TestCartesianProduct(t *testing.T) {
	t.Run("empty config", func(t *testing.T) {
		cfg := SweepConfig{}
		combos := cartesianProduct(cfg)
		if len(combos) != 0 {
			t.Errorf("expected 0 combos, got %d", len(combos))
		}
	})

	t.Run("single dimension single value", func(t *testing.T) {
		cfg := SweepConfig{
			Parameters: map[string][]string{"cache-type-k": {"q4_0"}},
		}
		combos := cartesianProduct(cfg)
		if len(combos) != 1 {
			t.Fatalf("expected 1 combo, got %d", len(combos))
		}
		if combos[0].Parameters["cache-type-k"] != "q4_0" {
			t.Errorf("unexpected value: %v", combos[0].Parameters)
		}
	})

	t.Run("single dimension multiple values", func(t *testing.T) {
		cfg := SweepConfig{
			Parameters: map[string][]string{"cache-type-k": {"q4_0", "q8_0"}},
		}
		combos := cartesianProduct(cfg)
		if len(combos) != 2 {
			t.Fatalf("expected 2 combos, got %d", len(combos))
		}
	})

	t.Run("two dimensions", func(t *testing.T) {
		cfg := SweepConfig{
			Parameters: map[string][]string{
				"cache-type-k": {"q4_0", "q8_0"},
				"threads":      {"8", "12"},
			},
		}
		combos := cartesianProduct(cfg)
		if len(combos) != 4 {
			t.Fatalf("expected 4 combos, got %d", len(combos))
		}
	})

	t.Run("three categories", func(t *testing.T) {
		cfg := SweepConfig{
			ProfileFields: map[string][]string{"bin": {"/bin/a", "/bin/b"}},
			Parameters:    map[string][]string{"cache-type-k": {"q4_0", "q8_0"}},
			Toggles:       map[string][]string{"np": {"on", "off"}},
		}
		combos := cartesianProduct(cfg)
		if len(combos) != 8 {
			t.Fatalf("expected 8 combos (2*2*2), got %d", len(combos))
		}
	})

	t.Run("empty values returns nil", func(t *testing.T) {
		cfg := SweepConfig{
			Parameters: map[string][]string{"cache-type-k": {}},
		}
		combos := cartesianProduct(cfg)
		if combos != nil {
			t.Errorf("expected nil for empty values, got %v", combos)
		}
	})

	t.Run("larger product", func(t *testing.T) {
		cfg := SweepConfig{
			Parameters: map[string][]string{
				"cache-type-k": {"q4_0", "q8_0"},
				"cache-type-v": {"q4_0", "q8_0"},
				"batch-size":   {"1024", "2048", "4096"},
			},
			Toggles: map[string][]string{
				"np":        {"on", "off"},
				"load-mode": {"off", "mlock"},
			},
		}
		combos := cartesianProduct(cfg)
		expected := 2 * 2 * 3 * 2 * 2
		if len(combos) != expected {
			t.Fatalf("expected %d combos, got %d", expected, len(combos))
		}
	})

	t.Run("all categories populated", func(t *testing.T) {
		cfg := SweepConfig{
			ProfileFields: map[string][]string{"bin": {"/a"}},
			Parameters:    map[string][]string{"threads": {"8"}},
			Toggles:       map[string][]string{"np": {"on"}},
		}
		combos := cartesianProduct(cfg)
		if len(combos) != 1 {
			t.Fatalf("expected 1 combo, got %d", len(combos))
		}
		c := combos[0]
		if c.ProfileFields["bin"] != "/a" {
			t.Errorf("unexpected profile field: %v", c.ProfileFields)
		}
		if c.Parameters["threads"] != "8" {
			t.Errorf("unexpected parameter: %v", c.Parameters)
		}
		if c.Toggles["np"] != "on" {
			t.Errorf("unexpected toggle: %v", c.Toggles)
		}
	})
}

func TestFlattenCombo(t *testing.T) {
	combo := Combination{
		ProfileFields: map[string]string{"bin": "/usr/bin/llama-server"},
		Parameters:    map[string]string{"threads": "8"},
		Toggles:       map[string]string{"np": "on"},
	}
	flat := flattenCombo(combo)

	if flat["field:bin"] != "/usr/bin/llama-server" {
		t.Errorf("missing field:bin, got %v", flat)
	}
	if flat["threads"] != "8" {
		t.Errorf("missing threads, got %v", flat)
	}
	if flat["toggle:np"] != "on" {
		t.Errorf("missing toggle:np, got %v", flat)
	}
}

func TestComboLabel(t *testing.T) {
	combo := Combination{
		ProfileFields: map[string]string{"bin": "/usr/bin/llama-server"},
		Parameters:    map[string]string{"threads": "8"},
		Toggles:       map[string]string{"np": "on"},
	}
	label := comboLabel(combo)
	if label == "" {
		t.Error("label should not be empty")
	}
	if !strings.Contains(label, "threads=8") {
		t.Errorf("label should contain threads=8, got %q", label)
	}
}

func TestSetMappingValue(t *testing.T) {
	t.Run("update existing key", func(t *testing.T) {
		mapping := &yaml.Node{
			Kind: yaml.MappingNode,
			Content: []*yaml.Node{
				{Kind: yaml.ScalarNode, Value: "bin"},
				{Kind: yaml.ScalarNode, Value: "/old/path"},
			},
		}
		setMappingValue(mapping, "bin", "/new/path")
		if mapping.Content[1].Value != "/new/path" {
			t.Errorf("expected /new/path, got %s", mapping.Content[1].Value)
		}
	})

	t.Run("add new key", func(t *testing.T) {
		mapping := &yaml.Node{
			Kind:    yaml.MappingNode,
			Content: []*yaml.Node{},
		}
		setMappingValue(mapping, "bin", "/new/path")
		if len(mapping.Content) != 2 {
			t.Fatalf("expected 2 nodes, got %d", len(mapping.Content))
		}
		if mapping.Content[0].Value != "bin" || mapping.Content[1].Value != "/new/path" {
			t.Errorf("unexpected nodes: %s=%s", mapping.Content[0].Value, mapping.Content[1].Value)
		}
	})
}

func TestSetFlagValue(t *testing.T) {
	t.Run("update existing flag", func(t *testing.T) {
		flagsSeq := &yaml.Node{
			Kind: yaml.SequenceNode,
			Content: []*yaml.Node{
				{Kind: yaml.ScalarNode, Value: "--threads"},
				{Kind: yaml.ScalarNode, Value: "8"},
			},
		}
		setFlagValue(flagsSeq, "--threads", "12")
		if flagsSeq.Content[1].Value != "12" {
			t.Errorf("expected 12, got %s", flagsSeq.Content[1].Value)
		}
	})

	t.Run("add new flag", func(t *testing.T) {
		flagsSeq := &yaml.Node{
			Kind:    yaml.SequenceNode,
			Content: []*yaml.Node{},
		}
		setFlagValue(flagsSeq, "--threads", "8")
		if len(flagsSeq.Content) != 2 {
			t.Fatalf("expected 2 nodes, got %d", len(flagsSeq.Content))
		}
		if flagsSeq.Content[0].Value != "--threads" || flagsSeq.Content[1].Value != "8" {
			t.Errorf("unexpected flag: %s=%s", flagsSeq.Content[0].Value, flagsSeq.Content[1].Value)
		}
	})
}

func TestRemoveFlag(t *testing.T) {
	t.Run("remove flag with value", func(t *testing.T) {
		flagsSeq := &yaml.Node{
			Kind: yaml.SequenceNode,
			Content: []*yaml.Node{
				{Kind: yaml.ScalarNode, Value: "--threads"},
				{Kind: yaml.ScalarNode, Value: "8"},
				{Kind: yaml.ScalarNode, Value: "--batch-size"},
				{Kind: yaml.ScalarNode, Value: "2048"},
			},
		}
		removeFlag(flagsSeq, "--threads")
		if len(flagsSeq.Content) != 2 {
			t.Fatalf("expected 2 nodes after removal, got %d", len(flagsSeq.Content))
		}
		if flagsSeq.Content[0].Value != "--batch-size" {
			t.Errorf("expected --batch-size first, got %s", flagsSeq.Content[0].Value)
		}
	})

	t.Run("flag not present", func(t *testing.T) {
		flagsSeq := &yaml.Node{
			Kind: yaml.SequenceNode,
			Content: []*yaml.Node{
				{Kind: yaml.ScalarNode, Value: "--threads"},
				{Kind: yaml.ScalarNode, Value: "8"},
			},
		}
		removeFlag(flagsSeq, "--nonexistent")
		if len(flagsSeq.Content) != 2 {
			t.Errorf("should not modify when flag not present, got %d nodes", len(flagsSeq.Content))
		}
	})

	t.Run("flag with negative numeric value", func(t *testing.T) {
		flagsSeq := &yaml.Node{
			Kind: yaml.SequenceNode,
			Content: []*yaml.Node{
				{Kind: yaml.ScalarNode, Value: "--some-flag"},
				{Kind: yaml.ScalarNode, Value: "-1"},
				{Kind: yaml.ScalarNode, Value: "--batch-size"},
				{Kind: yaml.ScalarNode, Value: "2048"},
			},
		}
		removeFlag(flagsSeq, "--some-flag")
		if len(flagsSeq.Content) != 2 {
			t.Fatalf("expected 2 nodes after removal (flag + negative value), got %d", len(flagsSeq.Content))
		}
		if flagsSeq.Content[0].Value != "--batch-size" {
			t.Errorf("expected --batch-size first, got %s", flagsSeq.Content[0].Value)
		}
	})
}

func TestApplyToggle(t *testing.T) {
	t.Run("np on adds flag", func(t *testing.T) {
		flagsSeq := &yaml.Node{
			Kind:    yaml.SequenceNode,
			Content: []*yaml.Node{},
		}
		applyToggle(flagsSeq, "np", "on")
		if len(flagsSeq.Content) != 2 {
			t.Fatalf("expected 2 nodes, got %d", len(flagsSeq.Content))
		}
		if flagsSeq.Content[0].Value != "-np" {
			t.Errorf("expected -np, got %s", flagsSeq.Content[0].Value)
		}
	})

	t.Run("np off removes flag", func(t *testing.T) {
		flagsSeq := &yaml.Node{
			Kind: yaml.SequenceNode,
			Content: []*yaml.Node{
				{Kind: yaml.ScalarNode, Value: "-np"},
				{Kind: yaml.ScalarNode, Value: "1"},
			},
		}
		applyToggle(flagsSeq, "np", "off")
		if len(flagsSeq.Content) != 0 {
			t.Errorf("expected empty after np off, got %d nodes", len(flagsSeq.Content))
		}
	})

	t.Run("load-mode mlock sets value", func(t *testing.T) {
		flagsSeq := &yaml.Node{
			Kind:    yaml.SequenceNode,
			Content: []*yaml.Node{},
		}
		applyToggle(flagsSeq, "load-mode", "mlock")
		if len(flagsSeq.Content) != 2 {
			t.Fatalf("expected 2 nodes, got %d", len(flagsSeq.Content))
		}
		if flagsSeq.Content[0].Value != "--load-mode" || flagsSeq.Content[1].Value != "mlock" {
			t.Errorf("unexpected: %s=%s", flagsSeq.Content[0].Value, flagsSeq.Content[1].Value)
		}
	})

	t.Run("load-mode off removes", func(t *testing.T) {
		flagsSeq := &yaml.Node{
			Kind: yaml.SequenceNode,
			Content: []*yaml.Node{
				{Kind: yaml.ScalarNode, Value: "--load-mode"},
				{Kind: yaml.ScalarNode, Value: "mlock"},
			},
		}
		applyToggle(flagsSeq, "load-mode", "off")
		if len(flagsSeq.Content) != 0 {
			t.Errorf("expected empty after load-mode off, got %d nodes", len(flagsSeq.Content))
		}
	})
}

func TestBackupRestoreConfig(t *testing.T) {
	dir := t.TempDir()
	configFile := filepath.Join(dir, "config.yaml")
	backupFile := filepath.Join(dir, "config.yaml.backup")

	original := []byte("profile: test\nthreads: 8\n")
	os.WriteFile(configFile, original, 0644)

	data, err := os.ReadFile(configFile)
	if err != nil {
		t.Fatal(err)
	}
	err = os.WriteFile(backupFile, data, 0644)
	if err != nil {
		t.Fatal(err)
	}

	os.WriteFile(configFile, []byte("profile: modified\n"), 0644)

	restored, _ := os.ReadFile(backupFile)
	os.WriteFile(configFile, restored, 0644)

	result, _ := os.ReadFile(configFile)
	if string(result) != string(original) {
		t.Errorf("restore failed: got %q, want %q", string(result), string(original))
	}
}

func TestResolvePort(t *testing.T) {
	t.Run("default dense port", func(t *testing.T) {
		viper.Reset()
		defer viper.Reset()

		port := resolvePort("test")
		if port != 8090 {
			t.Errorf("expected 8090, got %d", port)
		}
	})

	t.Run("moe type uses moe port", func(t *testing.T) {
		viper.Reset()
		viper.Set("profiles.moemodel.type", "moe")
		defer viper.Reset()

		port := resolvePort("moemodel")
		if port != 8091 {
			t.Errorf("expected 8091, got %d", port)
		}
	})

	t.Run("explicit profile port", func(t *testing.T) {
		viper.Reset()
		viper.Set("profiles.custom.port", 9090)
		defer viper.Reset()

		port := resolvePort("custom")
		if port != 9090 {
			t.Errorf("expected 9090, got %d", port)
		}
	})

	t.Run("custom dense port", func(t *testing.T) {
		viper.Reset()
		viper.Set("llama_server.dense_port", 7070)
		defer viper.Reset()

		port := resolvePort("test")
		if port != 7070 {
			t.Errorf("expected 7070, got %d", port)
		}
	})
}

func TestSortedKeys(t *testing.T) {
	m := map[string][]string{
		"z": {"1"},
		"a": {"2"},
		"m": {"3"},
	}
	keys := sortedKeys(m)
	if len(keys) != 3 {
		t.Fatalf("expected 3 keys, got %d", len(keys))
	}
	if keys[0] != "a" || keys[1] != "m" || keys[2] != "z" {
		t.Errorf("unexpected order: %v", keys)
	}
}

func TestGetReportWriter(t *testing.T) {
	t.Run("json format", func(t *testing.T) {
		w, err := GetReportWriter("json")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if w.Extension() != ".json" {
			t.Errorf("expected .json, got %s", w.Extension())
		}
	})

	t.Run("csv format", func(t *testing.T) {
		w, err := GetReportWriter("csv")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if w.Extension() != ".csv" {
			t.Errorf("expected .csv, got %s", w.Extension())
		}
	})

	t.Run("unknown format", func(t *testing.T) {
		_, err := GetReportWriter("xml")
		if err == nil {
			t.Error("expected error for unknown format")
		}
	})
}

func TestCollectOverrideKeys(t *testing.T) {
	results := []SweepResult{
		{Overrides: map[string]string{"threads": "8", "toggle:np": "on"}},
		{Overrides: map[string]string{"threads": "12", "field:bin": "/a"}},
	}
	keys := collectOverrideKeys(results)
	if len(keys) != 3 {
		t.Errorf("expected 3 unique keys, got %d: %v", len(keys), keys)
	}
}

func TestSaveSweepReportJSON(t *testing.T) {
	dir := t.TempDir()
	report := &SweepReport{
		Profile:       "test",
		Model:         "Qwen3.8-27B.gguf",
		Hostname:      "auriga",
		Timestamp:     "2026-08-31T10:00:00Z",
		TotalDuration: 180000,
		Results: []SweepResult{
			{
				Index:      0,
				Overrides:  map[string]string{"threads": "8"},
				TTFT:       150,
				Prompt:     100.5,
				DurationMs: 60000,
				NoThink:    BenchData{Median: 25.3, Min: 22.0, Max: 28.0},
				Think:      BenchData{Median: 18.1, Min: 16.0, Max: 20.0},
			},
		},
		Complete: true,
	}

	path := filepath.Join(dir, "test-report.json")
	err := (&JSONReportWriter{}).Write(report, path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("cannot read report: %v", err)
	}
	content := string(data)
	for _, expected := range []string{`"profile"`, `"model"`, `"hostname"`, `"timestamp"`, `"total_duration_ms"`, `"duration_ms"`, "25.3"} {
		if !strings.Contains(content, expected) {
			t.Errorf("JSON should contain %s", expected)
		}
	}
}

func TestSaveSweepReportCSV(t *testing.T) {
	dir := t.TempDir()
	report := &SweepReport{
		Profile: "test",
		Results: []SweepResult{
			{
				Index:     0,
				Overrides: map[string]string{"threads": "8", "toggle:np": "on"},
				TTFT:      150,
				Prompt:    100.5,
				NoThink:   BenchData{Median: 25.3, Min: 22.0, Max: 28.0},
				Think:     BenchData{Median: 18.1, Min: 16.0, Max: 20.0},
			},
			{
				Index:     1,
				Overrides: map[string]string{"threads": "12", "toggle:np": "off"},
				TTFT:      160,
				Prompt:    98.2,
				NoThink:   BenchData{Median: 27.1, Min: 24.0, Max: 30.0},
				Think:     BenchData{Median: 19.5, Min: 17.0, Max: 22.0},
			},
		},
		Complete: true,
	}

	path := filepath.Join(dir, "test-report.csv")
	err := (&CSVReportWriter{}).Write(report, path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.HasSuffix(path, ".csv") {
		t.Errorf("expected .csv extension, got %s", path)
	}

	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("cannot open report: %v", err)
	}
	defer f.Close()

	reader := csv.NewReader(f)
	records, err := reader.ReadAll()
	if err != nil {
		t.Fatalf("cannot parse CSV: %v", err)
	}

	if len(records) != 3 {
		t.Fatalf("expected 3 rows (header + 2 data), got %d", len(records))
	}

	header := records[0]
	if header[0] != "index" || header[1] != "ttft_ms" {
		t.Errorf("unexpected header: %v", header[:2])
	}

	hasThreads := false
	hasToggleNP := false
	hasDuration := false
	for _, h := range header {
		if h == "threads" {
			hasThreads = true
		}
		if h == "toggle:np" {
			hasToggleNP = true
		}
		if h == "duration_ms" {
			hasDuration = true
		}
	}
	if !hasThreads || !hasToggleNP {
		t.Errorf("header missing override columns: %v", header)
	}
	if !hasDuration {
		t.Errorf("header missing duration_ms column: %v", header)
	}

	row1 := records[1]
	if row1[0] != "0" {
		t.Errorf("first row index: got %q, want '0'", row1[0])
	}
	if row1[1] != "150" {
		t.Errorf("first row TTFT: got %q, want '150'", row1[1])
	}
}

func TestSaveSweepReportCSV_WithErrors(t *testing.T) {
	dir := t.TempDir()
	report := &SweepReport{
		Profile: "test",
		Results: []SweepResult{
			{
				Index:     0,
				Overrides: map[string]string{"threads": "8"},
				Error:     "server timeout",
			},
		},
		Complete: false,
	}

	path := filepath.Join(dir, "test-errors.csv")
	err := (&CSVReportWriter{}).Write(report, path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	f, _ := os.Open(path)
	defer f.Close()
	records, _ := csv.NewReader(f).ReadAll()

	if len(records) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(records))
	}

	errorIdx := -1
	for i, h := range records[0] {
		if h == "error" {
			errorIdx = i
		}
	}
	if errorIdx < 0 {
		t.Fatal("no error column in header")
	}
	if records[1][errorIdx] != "server timeout" {
		t.Errorf("error column: got %q, want 'server timeout'", records[1][errorIdx])
	}
}

func TestSaveSweepReport_TimestampInFilename(t *testing.T) {
	dir := t.TempDir()
	startTime := time.Date(2026, 8, 31, 10, 30, 45, 0, time.UTC)

	report := &SweepReport{
		Profile:   "test-profile",
		Timestamp: startTime.Format(time.RFC3339),
		Results:   []SweepResult{},
		Complete:  true,
	}

	path := filepath.Join(dir, "test-report.json")
	err := (&JSONReportWriter{}).Write(report, path)
	if err != nil {
		t.Fatalf("write failed: %v", err)
	}

	expectedTimestamp := startTime.Format("2006-01-02T15-04-05")
	expectedFilename := "test-profile-" + expectedTimestamp + ".json"
	if expectedFilename == "" {
		t.Fatal("empty filename")
	}

	data, _ := os.ReadFile(path)
	content := string(data)
	if !strings.Contains(content, startTime.Format(time.RFC3339)) {
		t.Errorf("report should contain sweep start timestamp, got: %s", content[:100])
	}
}

func TestCartesianProduct_EmptyValues(t *testing.T) {
	cfg := SweepConfig{
		Parameters: map[string][]string{
			"cache-type-k": {"q4_0"},
			"threads":      {},
		},
	}
	combos := cartesianProduct(cfg)
	if combos != nil {
		t.Errorf("expected nil for config with empty dimension values, got %d combos", len(combos))
	}
}
