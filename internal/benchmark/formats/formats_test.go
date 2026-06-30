package formats

import (
	"testing"
)

type mockRunner struct{}

func (m *mockRunner) BuildPrompt(p Problem, s Suite) (string, error) {
	return "mock prompt", nil
}
func (m *mockRunner) ValidateResponse(resp string, p Problem, dir string) (bool, string, error) {
	return true, "", nil
}
func (m *mockRunner) BuildRetryPrompt(p Problem, dir string, err string) (string, error) {
	return "retry", nil
}

func TestRegisterAndGet(t *testing.T) {
	Register("mock", &mockRunner{})

	r, err := Get("mock")
	if err != nil {
		t.Fatal(err)
	}
	if r == nil {
		t.Error("expected non-nil runner")
	}

	prompt, err := r.BuildPrompt(Problem{}, Suite{})
	if err != nil || prompt != "mock prompt" {
		t.Errorf("unexpected prompt: %q, err: %v", prompt, err)
	}
}

func TestGet_Unknown(t *testing.T) {
	_, err := Get("nonexistent-format")
	if err == nil {
		t.Error("expected error for unknown format")
	}
}

func TestAvailable(t *testing.T) {
	Register("test-fmt", &mockRunner{})
	avail := Available()
	found := false
	for _, a := range avail {
		if a == "test-fmt" {
			found = true
		}
	}
	if !found {
		t.Error("expected 'test-fmt' in available formats")
	}
}
