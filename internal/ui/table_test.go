package ui

import (
	"strings"
	"testing"
)

func TestStripANSI(t *testing.T) {
	colored := "\033[92m✓\033[0m"
	result := stripANSI(colored)
	if result != "✓" {
		t.Errorf("expected '✓', got %q", result)
	}
}

func TestStripANSI_Plain(t *testing.T) {
	result := stripANSI("hello")
	if result != "hello" {
		t.Errorf("expected 'hello', got %q", result)
	}
}

func TestNewTable(t *testing.T) {
	tbl := NewTable("Test", "NAME", "VALUE")
	if len(tbl.Columns) != 2 {
		t.Errorf("expected 2 columns, got %d", len(tbl.Columns))
	}
	if tbl.Title != "Test" {
		t.Errorf("expected title 'Test', got %q", tbl.Title)
	}
}

func TestTable_AddRow_ExpandsWidth(t *testing.T) {
	tbl := NewTable("", "A", "B")
	tbl.AddRow("short", "x")
	tbl.AddRow("a very long value here", "y")

	if tbl.Columns[0].Width != len("a very long value here") {
		t.Errorf("expected width %d, got %d", len("a very long value here"), tbl.Columns[0].Width)
	}
}

func TestTable_AddRow_ANSINotCounted(t *testing.T) {
	tbl := NewTable("", "STATUS")
	tbl.AddRow("\033[92m✓\033[0m")

	// Width should be based on visible "✓" (3 bytes in UTF-8), not the full ANSI string
	if tbl.Columns[0].Width != len("STATUS") {
		t.Errorf("ANSI codes should not affect width, got %d", tbl.Columns[0].Width)
	}
}

func TestTable_Print(t *testing.T) {
	tbl := NewTable("My Table", "COL1", "COL2")
	tbl.AddRow("a", "b")

	out := captureStdout(func() { tbl.Print() })
	if !strings.Contains(out, "My Table") {
		t.Error("missing title")
	}
	if !strings.Contains(out, "COL1") {
		t.Error("missing header")
	}
	if !strings.Contains(out, "─") {
		t.Error("missing separator")
	}
}
