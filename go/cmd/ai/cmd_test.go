package ai

import (
	core "dappco.re/go"
	"testing"
)

func TestAddAICommands_Good(t *testing.T) {
	root := core.New()

	if r := AddAICommands(root); !r.OK {
		t.Fatalf("register AI commands: %s", r.Error())
	}
	if r := AddAICommands(root); !r.OK {
		t.Fatalf("register duplicate AI commands: %s", r.Error())
	}

	commands := root.Commands()
	if len(commands) != 3 {
		t.Fatalf("expected 3 AI command paths, got %d: %#v", len(commands), commands)
	}

	for _, path := range []string{
		"ai",
		"ai/metrics",
		"ai/rag",
	} {
		cmd := root.Command(path)
		if !cmd.OK {
			t.Fatalf("find %s: %s", path, cmd.Error())
		}
	}
}

// --- AX-7 canonical triplets ---

func TestCmd_AddAICommands_Good(t *core.T) {
	root := core.New()
	r := AddAICommands(root)
	cmd := root.Command("ai")

	core.AssertTrue(t, r.OK)
	core.AssertTrue(t, cmd.OK)
	core.AssertEqual(t, "ai", cmd.Value.(*core.Command).Name)
}

func TestCmd_AddAICommands_Bad(t *core.T) {
	root := core.New()
	AddAICommands(root)
	AddAICommands(root)

	core.AssertLen(t, root.Commands(), 3)
	core.AssertEqual(t, "ai", root.Commands()[0])
}

func TestCmd_AddAICommands_Ugly(t *core.T) {
	root := core.New()
	root.Command("ai", core.Command{Description: "pre-existing"})
	AddAICommands(root)

	core.AssertLen(t, root.Commands(), 3)
	core.AssertEqual(t, "ai", root.Commands()[0])
}
