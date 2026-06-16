package profile

import (
	"testing"
)

func TestRepeatChar(t *testing.T) {
	result := repeatChar('─', 5)
	if len(result) != 5 {
		t.Errorf("expected length 5, got %d", len(result))
	}
}

func TestRepeatCharZero(t *testing.T) {
	result := repeatChar('─', 0)
	if len(result) != 0 {
		t.Errorf("expected length 0, got %d", len(result))
	}
}
