package benchmark

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"
)

func setupTestPatterns() {
	sensitivePatterns = []struct {
		Pattern     *regexp.Regexp
		Description string
	}{
		{regexp.MustCompile(`10\.0\.0\.99`), "Test LAN IP"},
		{regexp.MustCompile(`10\.10\.10\.1`), "Test VPN IP (server)"},
		{regexp.MustCompile(`10\.10\.10\.2`), "Test VPN IP (client)"},
		{regexp.MustCompile(`test@example\.com`), "Test email"},
		{regexp.MustCompile(`testhost`), "Test hostname"},
	}
}

func TestCheckSensitiveData_Clean(t *testing.T) {
	setupTestPatterns()
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "index.astro"), []byte("<h1>Hello World</h1>"), 0644)

	violations := CheckSensitiveData(dir)
	if len(violations) != 0 {
		t.Errorf("expected 0 violations, got %d: %v", len(violations), violations)
	}
}

func TestCheckSensitiveData_DetectsIP(t *testing.T) {
	setupTestPatterns()
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "config.js"), []byte("const host = '10.0.0.99';"), 0644)

	violations := CheckSensitiveData(dir)
	if len(violations) == 0 {
		t.Error("expected violations for server IP")
	}
	if violations[0].Description != "Test LAN IP" {
		t.Errorf("unexpected description: %q", violations[0].Description)
	}
}

func TestCheckSensitiveData_DetectsTailscale(t *testing.T) {
	setupTestPatterns()
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "setup.astro"), []byte("Connect to 10.10.10.1 via Tailscale"), 0644)

	violations := CheckSensitiveData(dir)
	if len(violations) == 0 {
		t.Error("expected violations for Tailscale IP")
	}
}

func TestCheckSensitiveData_DetectsEmail(t *testing.T) {
	setupTestPatterns()
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "about.md"), []byte("Contact: test@example.com"), 0644)

	violations := CheckSensitiveData(dir)
	if len(violations) == 0 {
		t.Error("expected violations for email")
	}
}

func TestCheckSensitiveData_IgnoresBinary(t *testing.T) {
	setupTestPatterns()
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "image.png"), []byte("10.0.0.99"), 0644)

	violations := CheckSensitiveData(dir)
	if len(violations) != 0 {
		t.Errorf("expected 0 violations for .png, got %d", len(violations))
	}
}

func TestCheckSensitiveData_MultipleViolations(t *testing.T) {
	setupTestPatterns()
	dir := t.TempDir()
	content := "IP: 10.0.0.99\nVPN: 10.10.10.1\nEmail: test@example.com\nHost: testhost"
	os.WriteFile(filepath.Join(dir, "data.json"), []byte(content), 0644)

	violations := CheckSensitiveData(dir)
	if len(violations) < 4 {
		t.Errorf("expected at least 4 violations, got %d", len(violations))
	}
}

func TestCheckSensitiveData_NoPatterns(t *testing.T) {
	sensitivePatterns = nil
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "config.js"), []byte("10.0.0.99"), 0644)

	violations := CheckSensitiveData(dir)
	if len(violations) != 0 {
		t.Errorf("expected 0 violations with no patterns, got %d", len(violations))
	}
}
