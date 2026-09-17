package ui

import "testing"

func TestFormatGiB(t *testing.T) {
	tests := []struct {
		name  string
		bytes int64
		want  string
	}{
		{name: "When bytes are zero, it should format zero GiB", bytes: 0, want: "0.0 GiB"},
		{name: "When bytes equal one GiB, it should format one GiB", bytes: 1 << 30, want: "1.0 GiB"},
		{name: "When bytes are negative, it should preserve negative margin", bytes: -(1 << 29), want: "-0.5 GiB"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := FormatGiB(tt.bytes); got != tt.want {
				t.Errorf("FormatGiB(%d) = %q, want %q", tt.bytes, got, tt.want)
			}
		})
	}
}
