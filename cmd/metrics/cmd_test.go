package metrics

import (
	"testing"
	"time"
)

func TestParseDuration_Good(t *testing.T) {
	tests := []struct {
		input string
		want  time.Duration
	}{
		{"7d", 7 * 24 * time.Hour},
		{"1d", 24 * time.Hour},
		{"24h", 24 * time.Hour},
		{"30m", 30 * time.Minute},
		{"1h", time.Hour},
	}

	for _, tc := range tests {
		got, err := parseDuration(tc.input)
		if err != nil {
			t.Errorf("parseDuration(%q): unexpected error: %v", tc.input, err)
			continue
		}
		if got != tc.want {
			t.Errorf("parseDuration(%q) = %v, want %v", tc.input, got, tc.want)
		}
	}
}

func TestParseDuration_Bad(t *testing.T) {
	bad := []string{
		"",    // too short
		"d",   // too short
		"0d",  // non-positive
		"-1d", // negative
		"abc", // non-numeric
		"7x",  // unknown unit
	}

	for _, input := range bad {
		_, err := parseDuration(input)
		if err == nil {
			t.Errorf("parseDuration(%q): expected error, got nil", input)
		}
	}
}
