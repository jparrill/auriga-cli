package benchmark

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	bench "github.com/jparrill/auriga-cli/internal/benchmark"
)

func makeResult(model, taskID string, success bool, duration int) bench.Result {
	return bench.Result{
		Model:    model,
		Backend:  "ollama",
		Suite:    "quality",
		TaskID:   taskID,
		Success:  success,
		Duration: duration,
	}
}

func TestBuildComparison(t *testing.T) {
	tests := []struct {
		name         string
		a, b         []bench.Result
		wantRows     int
		checkNilA    string
		checkNilB    string
	}{
		{
			name:     "When both runs have the same 2 tasks, it should produce 2 matched rows",
			a:        []bench.Result{makeResult("m", "t1", true, 10), makeResult("m", "t2", false, 20)},
			b:        []bench.Result{makeResult("m", "t1", true, 8), makeResult("m", "t2", true, 15)},
			wantRows: 2,
		},
		{
			name:      "When run B has a new task, it should appear with nil resultA",
			a:         []bench.Result{makeResult("m", "t1", true, 10)},
			b:         []bench.Result{makeResult("m", "t1", true, 8), makeResult("m", "t-new", false, 30)},
			wantRows:  2,
			checkNilA: "t-new",
		},
		{
			name:      "When run A has a task missing from B, it should appear with nil resultB",
			a:         []bench.Result{makeResult("m", "t1", true, 10), makeResult("m", "t-old", true, 5)},
			b:         []bench.Result{makeResult("m", "t1", true, 8)},
			wantRows:  2,
			checkNilB: "t-old",
		},
		{
			name:     "When both runs are empty, it should produce 0 rows",
			a:        nil,
			b:        nil,
			wantRows: 0,
		},
		{
			name:     "When both runs have the same single task, it should produce 1 row",
			a:        []bench.Result{makeResult("m", "t1", true, 10)},
			b:        []bench.Result{makeResult("m", "t1", false, 20)},
			wantRows: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rows := buildComparison(tt.a, tt.b)
			if len(rows) != tt.wantRows {
				t.Fatalf("got %d rows, want %d", len(rows), tt.wantRows)
			}
			if tt.checkNilA != "" {
				for _, r := range rows {
					if r.taskID == tt.checkNilA && r.resultA != nil {
						t.Errorf("resultA for %q should be nil", tt.checkNilA)
					}
				}
			}
			if tt.checkNilB != "" {
				for _, r := range rows {
					if r.taskID == tt.checkNilB && r.resultB != nil {
						t.Errorf("resultB for %q should be nil", tt.checkNilB)
					}
				}
			}
		})
	}
}

func TestDeltaSymbol(t *testing.T) {
	pass := makeResult("m", "t", true, 10)
	fail := makeResult("m", "t", false, 10)

	tests := []struct {
		name     string
		row      comparisonRow
		wantHas  string
	}{
		{
			name:    "When task improved fail to pass, it should show +",
			row:     comparisonRow{resultA: &fail, resultB: &pass},
			wantHas: "+",
		},
		{
			name:    "When task regressed pass to fail, it should show -",
			row:     comparisonRow{resultA: &pass, resultB: &fail},
			wantHas: "-",
		},
		{
			name:    "When task unchanged, it should show =",
			row:     comparisonRow{resultA: &pass, resultB: &pass},
			wantHas: "=",
		},
		{
			name:    "When task is new in B, it should show new",
			row:     comparisonRow{resultA: nil, resultB: &pass},
			wantHas: "new",
		},
		{
			name:    "When task is gone from B, it should show gone",
			row:     comparisonRow{resultA: &pass, resultB: nil},
			wantHas: "gone",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sym := deltaSymbol(tt.row)
			if !strings.Contains(sym, tt.wantHas) {
				t.Errorf("got %q, want substring %q", sym, tt.wantHas)
			}
		})
	}
}

func TestPassSymbol(t *testing.T) {
	pass := makeResult("m", "t", true, 10)
	fail := makeResult("m", "t", false, 10)

	tests := []struct {
		name    string
		result  *bench.Result
		wantHas string
	}{
		{"When result is pass, it should show checkmark", &pass, "✓"},
		{"When result is fail, it should show cross", &fail, "✗"},
		{"When result is nil, it should show dash", nil, "—"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sym := passSymbol(tt.result)
			if !strings.Contains(sym, tt.wantHas) {
				t.Errorf("got %q, want substring %q", sym, tt.wantHas)
			}
		})
	}
}

func TestTimeStr(t *testing.T) {
	r10 := makeResult("m", "t", true, 10)
	r120 := makeResult("m", "t", true, 120)

	tests := []struct {
		name    string
		result  *bench.Result
		wantHas string
	}{
		{"When result has 10s duration, it should show 10s", &r10, "10s"},
		{"When result has 120s duration, it should show 120s", &r120, "120s"},
		{"When result is nil, it should show dash", nil, "—"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := timeStr(tt.result)
			if !strings.Contains(s, tt.wantHas) {
				t.Errorf("got %q, want substring %q", s, tt.wantHas)
			}
		})
	}
}

func TestResultKey(t *testing.T) {
	tests := []struct {
		name      string
		r1, r2    bench.Result
		wantEqual bool
	}{
		{
			name:      "When task_id differs, keys should be different",
			r1:        makeResult("model-a", "task-1", true, 10),
			r2:        makeResult("model-a", "task-2", true, 10),
			wantEqual: false,
		},
		{
			name:      "When model differs, keys should be different",
			r1:        makeResult("model-a", "task-1", true, 10),
			r2:        makeResult("model-b", "task-1", true, 10),
			wantEqual: false,
		},
		{
			name:      "When all key fields match, keys should be equal",
			r1:        makeResult("model-a", "task-1", true, 10),
			r2:        makeResult("model-a", "task-1", false, 99),
			wantEqual: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			k1, k2 := resultKey(tt.r1), resultKey(tt.r2)
			if (k1 == k2) != tt.wantEqual {
				t.Errorf("resultKey(%q)==resultKey(%q) = %v, want %v", k1, k2, k1 == k2, tt.wantEqual)
			}
		})
	}
}

func TestLoadSummary(t *testing.T) {
	tests := []struct {
		name      string
		setup     func(dir string)
		wantCount int
		wantErr   bool
	}{
		{
			name: "When summary.json is valid with 2 results, it should load 2",
			setup: func(dir string) {
				results := []bench.Result{makeResult("m", "t1", true, 10), makeResult("m", "t2", false, 20)}
				data, _ := json.MarshalIndent(results, "", "  ")
				os.WriteFile(filepath.Join(dir, "summary.json"), data, 0644)
			},
			wantCount: 2,
		},
		{
			name:    "When summary.json is missing, it should return an error",
			setup:   func(dir string) {},
			wantErr: true,
		},
		{
			name: "When summary.json has invalid JSON, it should return an error",
			setup: func(dir string) {
				os.WriteFile(filepath.Join(dir, "summary.json"), []byte("not json"), 0644)
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			tt.setup(dir)
			loaded, err := loadSummary(dir)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(loaded) != tt.wantCount {
				t.Errorf("got %d results, want %d", len(loaded), tt.wantCount)
			}
		})
	}
}

func TestResolveRunDir(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(dir string)
		run     string
		wantNil bool
	}{
		{
			name: "When run is a valid timestamp dir, it should resolve to that dir",
			setup: func(dir string) {
				os.MkdirAll(filepath.Join(dir, "2026-07-12_1430"), 0755)
			},
			run: "2026-07-12_1430",
		},
		{
			name: "When run is latest and symlink exists, it should follow the symlink",
			setup: func(dir string) {
				target := filepath.Join(dir, "2026-07-12_1430")
				os.MkdirAll(target, 0755)
				os.Symlink("2026-07-12_1430", filepath.Join(dir, "latest"))
			},
			run: "latest",
		},
		{
			name:    "When run does not exist, it should return empty string",
			setup:   func(dir string) {},
			run:     "nonexistent",
			wantNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			tt.setup(dir)
			result := resolveRunDir(dir, tt.run)
			if tt.wantNil && result != "" {
				t.Errorf("expected empty string, got %q", result)
			}
			if !tt.wantNil && result == "" {
				t.Error("expected non-empty result, got empty")
			}
		})
	}
}

func TestHasLegacyResults(t *testing.T) {
	tests := []struct {
		name  string
		setup func(dir string)
		want  bool
	}{
		{
			name: "When dir has a subdirectory with metadata.json, it should return true",
			setup: func(dir string) {
				sub := filepath.Join(dir, "model__ollama")
				os.MkdirAll(sub, 0755)
				os.WriteFile(filepath.Join(sub, "metadata.json"), []byte("{}"), 0644)
			},
			want: true,
		},
		{
			name:  "When dir is empty, it should return false",
			setup: func(dir string) {},
			want:  false,
		},
		{
			name: "When dir has subdirectories without metadata.json, it should return false",
			setup: func(dir string) {
				os.MkdirAll(filepath.Join(dir, "subdir"), 0755)
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			tt.setup(dir)
			got := hasLegacyResults(dir)
			if got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNewBenchmarkCompareCmd(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr bool
	}{
		{"When given 2 args, it should accept them", []string{"run-a", "run-b"}, false},
		{"When given 1 arg, it should reject", []string{"only-one"}, true},
		{"When given 0 args, it should reject", []string{}, true},
		{"When given 3 args, it should reject", []string{"a", "b", "c"}, true},
	}

	cmd := newBenchmarkCompareCmd()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := cmd.Args(cmd, tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("got err=%v, wantErr=%v", err, tt.wantErr)
			}
		})
	}
}

func TestNewBenchmarkCmd_SubcommandRegistration(t *testing.T) {
	cmd := NewBenchmarkCmd()
	expected := []string{"list", "run", "suites", "download", "compare"}

	cmds := make(map[string]bool)
	for _, c := range cmd.Commands() {
		cmds[c.Name()] = true
	}

	for _, name := range expected {
		t.Run("When building benchmark cmd, it should register "+name, func(t *testing.T) {
			if !cmds[name] {
				t.Errorf("subcommand %q not found", name)
			}
		})
	}
}
