package mcp

import (
	"context"

	core "dappco.re/go"
)

type exampleSubsystem struct{}

func (exampleSubsystem) Name() string { return "example" }

func (exampleSubsystem) RegisterTools(s *Service) {
	s.RegisterToolFunc("example", "example_echo", "Echo example", func(context.Context, RawMessage) (any, error) {
		return map[string]string{"ok": "true"}, nil
	})
}

func ExampleRawMessage_MarshalJSON() {
	raw := RawMessage(`{"ok":true}`)
	data, err := raw.MarshalJSON()

	core.Println(err == nil)
	core.Println(string(data))
	// Output:
	// true
	// {"ok":true}
}

func ExampleRawMessage_UnmarshalJSON() {
	var raw RawMessage
	err := raw.UnmarshalJSON([]byte(`{"ok":true}`))

	core.Println(err == nil)
	core.Println(string(raw))
	// Output:
	// true
	// {"ok":true}
}

func ExampleNew() {
	service, err := New(Options{Unrestricted: true})

	core.Println(err == nil)
	core.Println(len(service.Tools()) > 0)
	// Output:
	// true
	// true
}

func ExampleWithWorkspaceRoot() {
	service, err := New(WithWorkspaceRoot(""))

	core.Println(err == nil)
	core.Println(service.WorkspaceRoot() == "")
	// Output:
	// true
	// true
}

func ExampleWithProcessService() {
	marker := struct{ Name string }{Name: "process"}
	service, err := New(WithProcessService(marker))

	core.Println(err == nil)
	core.Println(service.processService == marker)
	// Output:
	// true
	// true
}

func ExampleWithWSHub() {
	marker := struct{ Name string }{Name: "hub"}
	service, err := New(WithWSHub(marker))

	core.Println(err == nil)
	core.Println(service.wsHub == marker)
	// Output:
	// true
	// true
}

func ExampleWithSubsystem() {
	service, err := New(WithSubsystem(exampleSubsystem{}))

	core.Println(err == nil)
	core.Println(core.Contains(core.Join(",", service.ToolNames()...), "example_echo"))
	// Output:
	// true
	// true
}

func ExampleService_WorkspaceRoot() {
	service, _ := New(WithWorkspaceRoot(""))

	core.Println(service.WorkspaceRoot() == "")
	// Output:
	// true
}

func ExampleService_Tools() {
	service, _ := New(WithWorkspaceRoot(""))

	core.Println(len(service.Tools()) > 0)
	// Output:
	// true
}

func ExampleService_ToolNames() {
	service, _ := New(WithWorkspaceRoot(""))

	core.Println(core.Contains(core.Join(",", service.ToolNames()...), "file_read"))
	// Output:
	// true
}

func ExampleService_RegisterTool() {
	service, _ := New(WithWorkspaceRoot(""))
	err := service.RegisterTool(Tool{Name: "example_tool", Handler: func(context.Context, RawMessage) (any, error) {
		return map[string]bool{"ok": true}, nil
	}})

	core.Println(err.OK)
	core.Println(core.Contains(core.Join(",", service.ToolNames()...), "example_tool"))
	// Output:
	// true
	// true
}

func ExampleService_RegisterToolFunc() {
	service, _ := New(WithWorkspaceRoot(""))
	err := service.RegisterToolFunc("example", "example_func", "Example func", func(context.Context, RawMessage) (any, error) {
		return map[string]bool{"ok": true}, nil
	})

	core.Println(err.OK)
	core.Println(service.tools["example_func"].Group)
	// Output:
	// true
	// example
}

func ExampleService_Shutdown() {
	service, _ := New(WithWorkspaceRoot(""))
	err := service.Shutdown(context.Background())

	core.Println(err.OK)
	// Output:
	// true
}
