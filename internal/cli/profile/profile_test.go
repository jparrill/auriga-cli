package profile

import (
	"testing"
)

func TestNewProfileCreateCmd_DFlashFlag(t *testing.T) {
	cmd := newProfileCreateCmd()
	f := cmd.Flags().Lookup("dflash")
	if f == nil {
		t.Fatal("--dflash flag not found on create command")
	}
	if f.DefValue != "false" {
		t.Errorf("got default %q, want %q", f.DefValue, "false")
	}
}
