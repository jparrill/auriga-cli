package ui

import (
	"testing"

	"github.com/charmbracelet/log"
)

func TestInitLogger_Verbose(t *testing.T) {
	InitLogger(true)
	if Logger == nil {
		t.Fatal("Logger is nil after init")
	}
	if Logger.GetLevel() != log.DebugLevel {
		t.Errorf("expected DebugLevel, got %v", Logger.GetLevel())
	}
}

func TestInitLogger_Normal(t *testing.T) {
	InitLogger(false)
	if Logger == nil {
		t.Fatal("Logger is nil after init")
	}
	if Logger.GetLevel() != log.InfoLevel {
		t.Errorf("expected InfoLevel, got %v", Logger.GetLevel())
	}
}
