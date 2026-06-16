package ui

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
)

func captureStdout(fn func()) string {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	fn()
	w.Close()
	os.Stdout = old
	var buf bytes.Buffer
	io.Copy(&buf, r)
	return buf.String()
}

func TestOk(t *testing.T) {
	out := captureStdout(func() { Ok("test message") })
	if !strings.Contains(out, "test message") {
		t.Errorf("Ok output missing message: %q", out)
	}
	if !strings.Contains(out, "✓") {
		t.Errorf("Ok output missing checkmark: %q", out)
	}
}

func TestFail(t *testing.T) {
	out := captureStdout(func() { Fail("error message") })
	if !strings.Contains(out, "error message") {
		t.Errorf("Fail output missing message: %q", out)
	}
	if !strings.Contains(out, "✗") {
		t.Errorf("Fail output missing cross: %q", out)
	}
}

func TestWarn(t *testing.T) {
	out := captureStdout(func() { Warn("warning message") })
	if !strings.Contains(out, "warning message") {
		t.Errorf("Warn output missing message: %q", out)
	}
}

func TestInfo(t *testing.T) {
	out := captureStdout(func() { Info("info message") })
	if !strings.Contains(out, "info message") {
		t.Errorf("Info output missing message: %q", out)
	}
	if !strings.Contains(out, "→") {
		t.Errorf("Info output missing arrow: %q", out)
	}
}

func TestFormatKeyValue(t *testing.T) {
	result := FormatKeyValue("Name", "test")
	if !strings.Contains(result, "Name") || !strings.Contains(result, "test") {
		t.Errorf("FormatKeyValue missing content: %q", result)
	}
}
