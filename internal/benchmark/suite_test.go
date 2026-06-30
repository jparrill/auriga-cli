package benchmark

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/viper"
)

func TestLoadSuite(t *testing.T) {
	dir := t.TempDir()
	suiteDir := filepath.Join(dir, "test-suite")
	os.MkdirAll(suiteDir, 0755)

	yaml := `name: test-suite
description: A test suite
language: python
format: humaneval
problems: problems.jsonl
`
	os.WriteFile(filepath.Join(suiteDir, "suite.yaml"), []byte(yaml), 0644)

	viper.Set("benchmark.suites_dir", dir)

	suite, err := LoadSuite("test-suite")
	if err != nil {
		t.Fatal(err)
	}
	if suite.Name != "test-suite" {
		t.Errorf("expected name 'test-suite', got %q", suite.Name)
	}
	if suite.Format != "humaneval" {
		t.Errorf("expected format 'humaneval', got %q", suite.Format)
	}
	if suite.Dir != suiteDir {
		t.Errorf("expected dir %q, got %q", suiteDir, suite.Dir)
	}
}

func TestLoadSuite_NotFound(t *testing.T) {
	viper.Set("benchmark.suites_dir", t.TempDir())
	_, err := LoadSuite("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent suite")
	}
}

func TestListSuites(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"suite-a", "suite-b"} {
		suiteDir := filepath.Join(dir, name)
		os.MkdirAll(suiteDir, 0755)
		yaml := "name: " + name + "\ndescription: test\nformat: humaneval\n"
		os.WriteFile(filepath.Join(suiteDir, "suite.yaml"), []byte(yaml), 0644)
	}

	viper.Set("benchmark.suites_dir", dir)

	suites, err := ListSuites()
	if err != nil {
		t.Fatal(err)
	}
	if len(suites) != 2 {
		t.Errorf("expected 2 suites, got %d", len(suites))
	}
}

func TestLoadProblems(t *testing.T) {
	dir := t.TempDir()
	problems := `{"task_id": "test/0", "prompt": "def foo():", "test": "assert foo() == 1", "entry_point": "foo"}
{"task_id": "test/1", "prompt": "def bar():", "test": "assert bar() == 2", "entry_point": "bar"}
`
	os.WriteFile(filepath.Join(dir, "problems.jsonl"), []byte(problems), 0644)

	suite := &Suite{
		Problems: "problems.jsonl",
		Dir:      dir,
	}

	loaded, err := LoadProblems(suite)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded) != 2 {
		t.Errorf("expected 2 problems, got %d", len(loaded))
	}
	if loaded[0].TaskID != "test/0" {
		t.Errorf("expected task_id 'test/0', got %q", loaded[0].TaskID)
	}
	if loaded[0].EntryPoint != "foo" {
		t.Errorf("expected entry_point 'foo', got %q", loaded[0].EntryPoint)
	}
}

func TestLoadProblems_NoFile(t *testing.T) {
	suite := &Suite{Problems: "", Dir: t.TempDir()}
	problems, err := LoadProblems(suite)
	if err != nil {
		t.Fatal(err)
	}
	if problems != nil {
		t.Error("expected nil for empty problems field")
	}
}

func TestLoadProblems_WithLevels(t *testing.T) {
	dir := t.TempDir()
	problems := `{"task_id": "L1-01", "level": "L1-design", "prompt": "Design...", "eval": ["build"]}
{"task_id": "L2-01", "level": "L2-impl", "prompt": "Implement...", "eval": ["build", "test"], "test_cmd": "go test ./..."}
`
	os.WriteFile(filepath.Join(dir, "problems.jsonl"), []byte(problems), 0644)

	suite := &Suite{Problems: "problems.jsonl", Dir: dir}
	loaded, err := LoadProblems(suite)
	if err != nil {
		t.Fatal(err)
	}
	if loaded[0].Level != "L1-design" {
		t.Errorf("expected level 'L1-design', got %q", loaded[0].Level)
	}
	if loaded[1].TestCmd != "go test ./..." {
		t.Errorf("expected test_cmd, got %q", loaded[1].TestCmd)
	}
}
