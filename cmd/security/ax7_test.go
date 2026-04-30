package security

import (
	. "dappco.re/go"
	"dappco.re/go/cli/pkg/cli"
)

func TestSecurity_AlertSummary_Add_Good(t *T) {
	summary := &AlertSummary{}
	summary.Add("critical")
	summary.Add("high")

	AssertEqual(t, 2, summary.Total)
	AssertEqual(t, 1, summary.Critical)
	AssertEqual(t, 1, summary.High)
}

func TestSecurity_AlertSummary_Add_Bad(t *T) {
	summary := &AlertSummary{}
	summary.Add("unknown-severity")
	got := summary.Unknown

	AssertEqual(t, 1, got)
	AssertEqual(t, 1, summary.Total)
}

func TestSecurity_AlertSummary_Add_Ugly(t *T) {
	summary := &AlertSummary{}
	summary.Add("HIGH")
	summary.Add("")

	AssertEqual(t, 1, summary.High)
	AssertEqual(t, 1, summary.Unknown)
}

func TestSecurity_AlertSummary_String_Good(t *T) {
	summary := &AlertSummary{}
	summary.Add("critical")
	got := summary.String()

	AssertContains(t, got, "critical")
	AssertContains(t, got, "1")
}

func TestSecurity_AlertSummary_String_Bad(t *T) {
	summary := &AlertSummary{}
	got := summary.String()
	want := "No alerts"

	AssertContains(t, got, want)
}

func TestSecurity_AlertSummary_String_Ugly(t *T) {
	summary := &AlertSummary{Low: 2, Unknown: 1, Total: 3}
	got := summary.String()
	plain := summary.PlainString()

	AssertContains(t, got, "low")
	AssertEqual(t, "2 low | 1 unknown", plain)
}

func TestSecurity_AlertSummary_PlainString_Good(t *T) {
	summary := &AlertSummary{Critical: 1, High: 2, Total: 3}
	got := summary.PlainString()
	want := "1 critical | 2 high"

	AssertEqual(t, want, got)
}

func TestSecurity_AlertSummary_PlainString_Bad(t *T) {
	summary := &AlertSummary{}
	got := summary.PlainString()
	want := "No alerts"

	AssertEqual(t, want, got)
}

func TestSecurity_AlertSummary_PlainString_Ugly(t *T) {
	summary := &AlertSummary{Medium: 1, Low: 1, Unknown: 1, Total: 3}
	got := summary.PlainString()
	want := "1 medium | 1 low | 1 unknown"

	AssertEqual(t, want, got)
}

func TestSecurity_AddSecurityCommands_Good(t *T) {
	root := &cli.Command{Use: "core"}
	AddSecurityCommands(root)
	cmd, _, err := root.Find([]string{"security"})

	AssertNoError(t, err)
	AssertEqual(t, "security", cmd.Name())
}

func TestSecurity_AddSecurityCommands_Bad(t *T) {
	root := &cli.Command{Use: "core"}
	AddSecurityCommands(root)
	AddSecurityCommands(root)

	AssertLen(t, root.Commands(), 1)
	AssertEqual(t, "security", root.Commands()[0].Name())
}

func TestSecurity_AddSecurityCommands_Ugly(t *T) {
	root := &cli.Command{Use: "core"}
	root.AddCommand(&cli.Command{Use: "security"})
	AddSecurityCommands(root)

	AssertLen(t, root.Commands(), 1)
	AssertEqual(t, "security", root.Commands()[0].Name())
}

func TestSecurity_RawMessage_UnmarshalJSON_Good(t *T) {
	var raw githubRawMessage
	err := raw.UnmarshalJSON([]byte(`{"ok":true}`))
	got := string(raw)

	AssertNoError(t, err)
	AssertEqual(t, `{"ok":true}`, got)
}

func TestSecurity_RawMessage_UnmarshalJSON_Bad(t *T) {
	var raw *githubRawMessage
	err := raw.UnmarshalJSON([]byte(`{"ok":true}`))
	got := ErrorMessage(err)

	AssertError(t, err)
	AssertContains(t, got, "nil raw message")
}

func TestSecurity_RawMessage_UnmarshalJSON_Ugly(t *T) {
	raw := githubRawMessage(`old`)
	err := raw.UnmarshalJSON([]byte(`null`))
	got := string(raw)

	AssertNoError(t, err)
	AssertEqual(t, "null", got)
}
