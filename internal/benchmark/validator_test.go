package benchmark

import (
	"testing"
)

func TestTruncate_Short(t *testing.T) {
	result := truncate("hello", 10)
	if result != "hello" {
		t.Errorf("expected 'hello', got %q", result)
	}
}

func TestTruncate_Long(t *testing.T) {
	input := "this is a very long string that should be truncated"
	result := truncate(input, 10)
	if len(result) != 10 {
		t.Errorf("expected length 10, got %d", len(result))
	}
	expected := input[len(input)-10:]
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestTruncate_Exact(t *testing.T) {
	result := truncate("12345", 5)
	if result != "12345" {
		t.Errorf("expected '12345', got %q", result)
	}
}

func TestValidateBuild_NoPackageJson(t *testing.T) {
	dir := t.TempDir()
	ok, errMsg := ValidateBuild(dir)
	if ok {
		t.Error("expected failure for missing package.json")
	}
	if errMsg != "No package.json found" {
		t.Errorf("unexpected error: %q", errMsg)
	}
}
