package profile

import (
	"testing"
)

func TestNewProfileSetupCmd(t *testing.T) {
	tests := []struct {
		name  string
		check func(t *testing.T)
	}{
		{
			name: "When building profile command, it should register the setup subcommand",
			check: func(t *testing.T) {
				cmd := NewProfileCmd()
				for _, sub := range cmd.Commands() {
					if sub.Name() == "setup" {
						return
					}
				}
				t.Error("setup subcommand not found")
			},
		},
		{
			name: "When creating setup command, it should require repo flag",
			check: func(t *testing.T) {
				cmd := newProfileSetupCmd()
				f := cmd.Flags().Lookup("repo")
				if f == nil {
					t.Fatal("--repo flag not found")
				}
			},
		},
		{
			name: "When creating setup command, it should have vision flag defaulting to false",
			check: func(t *testing.T) {
				cmd := newProfileSetupCmd()
				f := cmd.Flags().Lookup("vision")
				if f == nil {
					t.Fatal("--vision flag not found")
				}
				if f.DefValue != "false" {
					t.Errorf("got default %q, want %q", f.DefValue, "false")
				}
			},
		},
		{
			name: "When creating setup command, it should have quant flag defaulting to empty",
			check: func(t *testing.T) {
				cmd := newProfileSetupCmd()
				f := cmd.Flags().Lookup("quant")
				if f == nil {
					t.Fatal("--quant flag not found")
				}
				if f.DefValue != "" {
					t.Errorf("got default %q, want empty", f.DefValue)
				}
			},
		},
		{
			name: "When creating setup command, it should require exactly 1 argument",
			check: func(t *testing.T) {
				cmd := newProfileSetupCmd()
				if err := cmd.Args(cmd, []string{}); err == nil {
					t.Error("expected error with 0 args")
				}
				if err := cmd.Args(cmd, []string{"name"}); err != nil {
					t.Errorf("expected no error with 1 arg, got %v", err)
				}
				if err := cmd.Args(cmd, []string{"a", "b"}); err == nil {
					t.Error("expected error with 2 args")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, tt.check)
	}
}
