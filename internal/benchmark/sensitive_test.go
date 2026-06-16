package benchmark

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCheckSensitiveData_Clean(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "index.astro"), []byte("<h1>Hello World</h1>"), 0644)

	violations := CheckSensitiveData(dir)
	if len(violations) != 0 {
		t.Errorf("expected 0 violations, got %d: %v", len(violations), violations)
	}
}

func TestCheckSensitiveData_DetectsIP(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "config.js"), []byte("const host = '192.168.1.143';"), 0644)

	violations := CheckSensitiveData(dir)
	if len(violations) == 0 {
		t.Error("expected violations for server IP")
	}
	if violations[0].Description != "Server LAN IP" {
		t.Errorf("unexpected description: %q", violations[0].Description)
	}
}

func TestCheckSensitiveData_DetectsTailscale(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "setup.astro"), []byte("Connect to 100.77.65.108 via Tailscale"), 0644)

	violations := CheckSensitiveData(dir)
	if len(violations) == 0 {
		t.Error("expected violations for Tailscale IP")
	}
}

func TestCheckSensitiveData_DetectsEmail(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "about.md"), []byte("Contact: jparrill@redhat.com"), 0644)

	violations := CheckSensitiveData(dir)
	if len(violations) == 0 {
		t.Error("expected violations for email")
	}
}

func TestCheckSensitiveData_IgnoresBinary(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "image.png"), []byte("192.168.1.143"), 0644)

	violations := CheckSensitiveData(dir)
	if len(violations) != 0 {
		t.Errorf("expected 0 violations for .png, got %d", len(violations))
	}
}

func TestCheckSensitiveData_MultipleViolations(t *testing.T) {
	dir := t.TempDir()
	content := "IP: 192.168.1.143\nTailscale: 100.77.65.108\nEmail: padajuan@gmail.com\nHost: xenomorph"
	os.WriteFile(filepath.Join(dir, "data.json"), []byte(content), 0644)

	violations := CheckSensitiveData(dir)
	if len(violations) < 4 {
		t.Errorf("expected at least 4 violations, got %d", len(violations))
	}
}
