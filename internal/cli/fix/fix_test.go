package fix

import (
	"testing"
)

func TestNewFixCmd(t *testing.T) {
	cmd := NewFixCmd()
	if cmd.Name() != "fix" {
		t.Errorf("expected 'fix', got %q", cmd.Name())
	}

	flags := []string{"list", "failed", "model", "run", "model-override"}
	for _, f := range flags {
		if cmd.Flags().Lookup(f) == nil {
			t.Errorf("expected flag --%s", f)
		}
	}
}

func TestResolveRunDir_NonExistent(t *testing.T) {
	result := resolveRunDir("/nonexistent/path", "latest")
	if result != "" {
		t.Errorf("expected empty for nonexistent dir, got %q", result)
	}
}

func TestResolveRunDir_SpecificRun(t *testing.T) {
	dir := t.TempDir()
	result := resolveRunDir("/nonexistent", dir)
	if result != "" {
		t.Errorf("expected empty for nonexistent run, got %q", result)
	}
}
