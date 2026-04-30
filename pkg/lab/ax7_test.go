package lab

import (
	. "dappco.re/go"
	"dappco.re/go/cli/pkg/cli"
)

func TestLab_AddLabCommands_Good(t *T) {
	root := &cli.Command{Use: "core"}
	AddLabCommands(root)
	cmd, _, err := root.Find([]string{"lab"})

	AssertNoError(t, err)
	AssertEqual(t, "lab", cmd.Name())
}

func TestLab_AddLabCommands_Bad(t *T) {
	root := &cli.Command{Use: "core"}
	AddLabCommands(root)
	AddLabCommands(root)

	AssertLen(t, root.Commands(), 1)
	AssertEqual(t, "lab", root.Commands()[0].Name())
}

func TestLab_AddLabCommands_Ugly(t *T) {
	root := &cli.Command{Use: "core"}
	root.AddCommand(&cli.Command{Use: "lab"})
	AddLabCommands(root)

	AssertLen(t, root.Commands(), 1)
	AssertEqual(t, "lab", root.Commands()[0].Name())
}

func TestLab_RunServe_Good(t *T) {
	t.Setenv("CORE_LAB_API_TOKEN", "")
	err := RunServe(CommandOptions{Bind: "0.0.0.0:8080"})
	got := ErrorMessage(err)

	AssertError(t, err)
	AssertContains(t, got, "non-loopback")
}

func TestLab_RunServe_Bad(t *T) {
	t.Setenv("CORE_LAB_API_TOKEN", "")
	err := RunServe(CommandOptions{Bind: "127.0.0.1:8080", AllowRemote: true})
	got := ErrorMessage(err)

	AssertError(t, err)
	AssertContains(t, got, "CORE_LAB_API_TOKEN")
}

func TestLab_RunServe_Ugly(t *T) {
	t.Setenv("CORE_LAB_API_TOKEN", "")
	err := RunServe(CommandOptions{Bind: "not-a-host", AllowRemote: false})
	got := ErrorMessage(err)

	AssertError(t, err)
	AssertContains(t, got, "non-loopback")
}

func TestLab_ValidateBindAddress_Good(t *T) {
	err := ValidateBindAddress("127.0.0.1:8080", false)
	got := IsLoopbackBindAddress("127.0.0.1:8080")
	want := true

	AssertNoError(t, err)
	AssertEqual(t, want, got)
}

func TestLab_ValidateBindAddress_Bad(t *T) {
	err := ValidateBindAddress("0.0.0.0:8080", false)
	got := ErrorMessage(err)
	want := "non-loopback"

	AssertError(t, err)
	AssertContains(t, got, want)
}

func TestLab_ValidateBindAddress_Ugly(t *T) {
	err := ValidateBindAddress(":8080", true)
	got := IsLoopbackBindAddress(":8080")
	want := false

	AssertNoError(t, err)
	AssertEqual(t, want, got)
}

func TestLab_IsLoopbackBindAddress_Good(t *T) {
	got := IsLoopbackBindAddress("localhost:8080")
	ipv4 := IsLoopbackBindAddress("127.0.0.1:8080")
	ipv6 := IsLoopbackBindAddress("[::1]:8080")

	AssertTrue(t, got)
	AssertTrue(t, ipv4)
	AssertTrue(t, ipv6)
}

func TestLab_IsLoopbackBindAddress_Bad(t *T) {
	got := IsLoopbackBindAddress("0.0.0.0:8080")
	wildcard := IsLoopbackBindAddress(":8080")
	remote := IsLoopbackBindAddress("example.com:8080")

	AssertFalse(t, got)
	AssertFalse(t, wildcard)
	AssertFalse(t, remote)
}

func TestLab_IsLoopbackBindAddress_Ugly(t *T) {
	empty := IsLoopbackBindAddress("")
	malformed := IsLoopbackBindAddress("::notanaddr:8080")
	missingPort := IsLoopbackBindAddress("localhost")

	AssertFalse(t, empty)
	AssertFalse(t, malformed)
	AssertFalse(t, missingPort)
}

func TestLab_ValidateRemoteAuth_Good(t *T) {
	err := ValidateRemoteAuth(false, "")
	remoteErr := ValidateRemoteAuth(true, "token")
	want := true

	AssertNoError(t, err)
	AssertNoError(t, remoteErr)
	AssertTrue(t, want)
}

func TestLab_ValidateRemoteAuth_Bad(t *T) {
	err := ValidateRemoteAuth(true, "")
	got := ErrorMessage(err)
	want := "CORE_LAB_API_TOKEN"

	AssertError(t, err)
	AssertContains(t, got, want)
}

func TestLab_ValidateRemoteAuth_Ugly(t *T) {
	err := ValidateRemoteAuth(true, "  ")
	got := ErrorMessage(err)
	want := "--allow-remote"

	AssertError(t, err)
	AssertContains(t, got, want)
}
