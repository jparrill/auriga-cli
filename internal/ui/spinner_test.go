package ui

import (
	"errors"
	"testing"
)

func TestWithSpinner_Success(t *testing.T) {
	err := WithSpinner("test", func() error {
		return nil
	})
	if err != nil {
		t.Errorf("expected nil error, got %v", err)
	}
}

func TestWithSpinner_Error(t *testing.T) {
	expected := errors.New("test error")
	err := WithSpinner("test", func() error {
		return expected
	})
	if err == nil {
		t.Error("expected error, got nil")
	}
}
