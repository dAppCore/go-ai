package ai

import (
	. "dappco.re/go"
	"dappco.re/go/cli/pkg/cli"
)

func TestAI_AddAICommands_Good(t *T) {
	root := &cli.Command{Use: "core"}
	AddAICommands(root)
	cmd, _, err := root.Find([]string{"ai"})

	AssertNoError(t, err)
	AssertEqual(t, "ai", cmd.Name())
}

func TestAI_AddAICommands_Bad(t *T) {
	root := &cli.Command{Use: "core"}
	AddAICommands(root)
	AddAICommands(root)

	AssertLen(t, root.Commands(), 1)
	AssertEqual(t, "ai", root.Commands()[0].Name())
}

func TestAI_AddAICommands_Ugly(t *T) {
	root := &cli.Command{Use: "core"}
	root.AddCommand(&cli.Command{Use: "ai"})
	AddAICommands(root)

	AssertLen(t, root.Commands(), 1)
	AssertEqual(t, "ai", root.Commands()[0].Name())
}
