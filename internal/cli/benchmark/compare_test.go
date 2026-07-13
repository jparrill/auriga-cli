package benchmark

import (
	"encoding/json"
	"os"
	"path/filepath"
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

func TestBuildComparison_MatchesByKey(t *testing.T) {
	a := []bench.Result{
		makeResult("qwen3:32b", "task-1", true, 10),
		makeResult("qwen3:32b", "task-2", false, 20),
	}
	b := []bench.Result{
		makeResult("qwen3:32b", "task-1", true, 8),
		makeResult("qwen3:32b", "task-2", true, 15),
	}

	rows := buildComparison(a, b)
	if len(rows) != 2 {
		t.Fatalf("When both runs have the same 2 tasks, it should produce 2 rows, got %d", len(rows))
	}
	for _, row := range rows {
		if row.resultA == nil || row.resultB == nil {
			t.Error("When tasks exist in both runs, both results should be non-nil")
		}
	}
}

func TestBuildComparison_DetectsNewTasks(t *testing.T) {
	a := []bench.Result{
		makeResult("qwen3:32b", "task-1", true, 10),
	}
	b := []bench.Result{
		makeResult("qwen3:32b", "task-1", true, 8),
		makeResult("qwen3:32b", "task-new", false, 30),
	}

	rows := buildComparison(a, b)
	if len(rows) != 2 {
		t.Fatalf("When run B has 1 new task, it should produce 2 rows, got %d", len(rows))
	}

	var newRow *comparisonRow
	for i, r := range rows {
		if r.taskID == "task-new" {
			newRow = &rows[i]
		}
	}
	if newRow == nil {
		t.Fatal("When run B has a new task, it should appear in the comparison")
	}
	if newRow.resultA != nil {
		t.Error("When a task is new in run B, resultA should be nil")
	}
	if newRow.resultB == nil {
		t.Error("When a task is new in run B, resultB should be non-nil")
	}
}

func TestBuildComparison_DetectsGoneTasks(t *testing.T) {
	a := []bench.Result{
		makeResult("qwen3:32b", "task-1", true, 10),
		makeResult("qwen3:32b", "task-old", true, 5),
	}
	b := []bench.Result{
		makeResult("qwen3:32b", "task-1", true, 8),
	}

	rows := buildComparison(a, b)
	if len(rows) != 2 {
		t.Fatalf("When run A has 1 task missing from B, it should produce 2 rows, got %d", len(rows))
	}

	var goneRow *comparisonRow
	for i, r := range rows {
		if r.taskID == "task-old" {
			goneRow = &rows[i]
		}
	}
	if goneRow == nil {
		t.Fatal("When a task is gone from run B, it should appear in the comparison")
	}
	if goneRow.resultA == nil {
		t.Error("When a task is gone from B, resultA should be non-nil")
	}
	if goneRow.resultB != nil {
		t.Error("When a task is gone from B, resultB should be nil")
	}
}

func TestBuildComparison_EmptyRuns(t *testing.T) {
	rows := buildComparison(nil, nil)
	if len(rows) != 0 {
		t.Errorf("When both runs are empty, it should produce 0 rows, got %d", len(rows))
	}
}

func TestBuildComparison_NoDuplicateKeys(t *testing.T) {
	a := []bench.Result{
		makeResult("model-a", "task-1", true, 10),
	}
	b := []bench.Result{
		makeResult("model-a", "task-1", false, 20),
	}

	rows := buildComparison(a, b)
	if len(rows) != 1 {
		t.Errorf("When both runs have the same single task, it should produce 1 row, got %d", len(rows))
	}
}

func TestDeltaSymbol_Improved(t *testing.T) {
	rA := makeResult("m", "t", false, 10)
	rB := makeResult("m", "t", true, 8)
	row := comparisonRow{resultA: &rA, resultB: &rB}
	sym := deltaSymbol(row)
	if sym == "" {
		t.Error("When task improved (fail→pass), delta should not be empty")
	}
}

func TestDeltaSymbol_Regressed(t *testing.T) {
	rA := makeResult("m", "t", true, 10)
	rB := makeResult("m", "t", false, 8)
	row := comparisonRow{resultA: &rA, resultB: &rB}
	sym := deltaSymbol(row)
	if sym == "" {
		t.Error("When task regressed (pass→fail), delta should not be empty")
	}
}

func TestDeltaSymbol_New(t *testing.T) {
	rB := makeResult("m", "t", true, 8)
	row := comparisonRow{resultA: nil, resultB: &rB}
	sym := deltaSymbol(row)
	if sym == "" {
		t.Error("When task is new in B, delta should not be empty")
	}
}

func TestDeltaSymbol_Gone(t *testing.T) {
	rA := makeResult("m", "t", true, 10)
	row := comparisonRow{resultA: &rA, resultB: nil}
	sym := deltaSymbol(row)
	if sym == "" {
		t.Error("When task is gone from B, delta should not be empty")
	}
}

func TestResultKey_Unique(t *testing.T) {
	r1 := makeResult("model-a", "task-1", true, 10)
	r2 := makeResult("model-a", "task-2", true, 10)
	r3 := makeResult("model-b", "task-1", true, 10)

	if resultKey(r1) == resultKey(r2) {
		t.Error("When task_id differs, keys should be different")
	}
	if resultKey(r1) == resultKey(r3) {
		t.Error("When model differs, keys should be different")
	}
}

func TestLoadSummary_ValidJSON(t *testing.T) {
	dir := t.TempDir()
	results := []bench.Result{
		makeResult("qwen3:32b", "task-1", true, 10),
		makeResult("qwen3:32b", "task-2", false, 20),
	}
	data, _ := json.MarshalIndent(results, "", "  ")
	os.WriteFile(filepath.Join(dir, "summary.json"), data, 0644)

	loaded, err := loadSummary(dir)
	if err != nil {
		t.Fatalf("When summary.json is valid, loadSummary should not error: %v", err)
	}
	if len(loaded) != 2 {
		t.Errorf("When summary.json has 2 results, it should load 2, got %d", len(loaded))
	}
}

func TestLoadSummary_MissingFile(t *testing.T) {
	dir := t.TempDir()
	_, err := loadSummary(dir)
	if err == nil {
		t.Error("When summary.json is missing, loadSummary should return an error")
	}
}

func TestNewBenchmarkCompareCmd_Registered(t *testing.T) {
	cmd := NewBenchmarkCmd()
	found := false
	for _, sub := range cmd.Commands() {
		if sub.Name() == "compare" {
			found = true
			break
		}
	}
	if !found {
		t.Error("When building benchmark command, it should register the compare subcommand")
	}
}

func TestNewBenchmarkCompareCmd_RequiresTwoArgs(t *testing.T) {
	cmd := newBenchmarkCompareCmd()
	if cmd.Args == nil {
		t.Fatal("When creating compare command, it should have an args validator")
	}
	err := cmd.Args(cmd, []string{"only-one"})
	if err == nil {
		t.Error("When given 1 arg, compare should reject it")
	}
	err = cmd.Args(cmd, []string{"run-a", "run-b"})
	if err != nil {
		t.Error("When given 2 args, compare should accept them")
	}
}
