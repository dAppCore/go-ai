package metrics

import (
	"testing"
	"time"

	"forge.lthn.ai/core/cli/pkg/cli"
)

func TestParseSinceDuration_Good(t *testing.T) {
	tests := []struct {
		input string
		want  time.Duration
	}{
		{"168h", 168 * time.Hour},
		{"7d", 7 * 24 * time.Hour},
		{"1d", 24 * time.Hour},
		{"24h", 24 * time.Hour},
		{"30m", 30 * time.Minute},
		{"1h", time.Hour},
	}

	for _, tc := range tests {
		got, err := parseSinceDuration(tc.input)
		if err != nil {
			t.Errorf("parseSinceDuration(%q): unexpected error: %v", tc.input, err)
			continue
		}
		if got != tc.want {
			t.Errorf("parseSinceDuration(%q) = %v, want %v", tc.input, got, tc.want)
		}
	}
}

func TestParseSinceDuration_Bad(t *testing.T) {
	bad := []string{
		"",    // too short
		"d",   // too short
		"0d",  // non-positive
		"-1d", // negative
		"abc", // non-numeric
		"7x",  // unknown unit
	}

	for _, input := range bad {
		_, err := parseSinceDuration(input)
		if err == nil {
			t.Errorf("parseSinceDuration(%q): expected error, got nil", input)
		}
	}
}

func TestAddMetricsCommand_Good_CommandInstancesKeepFlagStateLocal(t *testing.T) {
	firstRoot := &cli.Command{Use: "core"}
	secondRoot := &cli.Command{Use: "core"}

	AddMetricsCommand(firstRoot)
	AddMetricsCommand(secondRoot)

	firstCommand, _, err := firstRoot.Find([]string{"metrics"})
	if err != nil {
		t.Fatalf("find first metrics command: %v", err)
	}
	secondCommand, _, err := secondRoot.Find([]string{"metrics"})
	if err != nil {
		t.Fatalf("find second metrics command: %v", err)
	}

	if err := firstCommand.Flags().Set("since", "24h"); err != nil {
		t.Fatalf("set first --since: %v", err)
	}

	firstSince, err := firstCommand.Flags().GetString("since")
	if err != nil {
		t.Fatalf("get first --since: %v", err)
	}
	secondSince, err := secondCommand.Flags().GetString("since")
	if err != nil {
		t.Fatalf("get second --since: %v", err)
	}

	if firstSince != "24h" {
		t.Fatalf("first command since = %q, want %q", firstSince, "24h")
	}
	if secondSince != "168h" {
		t.Fatalf("second command since leaked shared state: got %q, want %q", secondSince, "168h")
	}
}
