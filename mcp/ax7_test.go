package mcp

import (
	"context"
	"os"

	core "dappco.re/go"
)

type ax7Subsystem struct {
	called *bool
	err    error
}

func (s ax7Subsystem) Name() string { return "ax7" }

func (s ax7Subsystem) RegisterTools(*Service) {
	if s.called != nil {
		*s.called = true
	}
}

func (s ax7Subsystem) Shutdown(context.Context) error {
	if s.called != nil {
		*s.called = true
	}
	return s.err
}

func TestMCP_New_Good(t *core.T) {
	service, err := New(WithWorkspaceRoot(t.TempDir()))
	names := service.ToolNames()

	core.AssertNoError(t, err)
	core.AssertTrue(t, len(names) > 0)
}

func TestMCP_New_Bad(t *core.T) {
	service, err := New(42)
	got := core.ErrorMessage(err)

	core.AssertNil(t, service)
	core.AssertError(t, err)
	core.AssertContains(t, got, "unsupported")
}

func TestMCP_New_Ugly(t *core.T) {
	service, err := New(Options{Unrestricted: true})
	root := service.WorkspaceRoot()

	core.AssertNoError(t, err)
	core.AssertEqual(t, "", root)
}

func TestMCP_WithWorkspaceRoot_Good(t *core.T) {
	service := &Service{}
	option := WithWorkspaceRoot(t.TempDir())
	err := option(service)

	core.AssertNoError(t, err)
	core.AssertNotEqual(t, "", service.WorkspaceRoot())
}

func TestMCP_WithWorkspaceRoot_Bad(t *core.T) {
	service := &Service{workspaceRoot: "before"}
	option := WithWorkspaceRoot("")
	err := option(service)

	core.AssertNoError(t, err)
	core.AssertEqual(t, "", service.WorkspaceRoot())
}

func TestMCP_WithWorkspaceRoot_Ugly(t *core.T) {
	service := &Service{}
	option := WithWorkspaceRoot(".")
	err := option(service)

	core.AssertNoError(t, err)
	core.AssertTrue(t, service.WorkspaceRoot() != ".")
}

func TestMCP_WithProcessService_Good(t *core.T) {
	service := &Service{}
	option := WithProcessService("process")
	err := option(service)

	core.AssertNoError(t, err)
	core.AssertEqual(t, "process", service.processService)
}

func TestMCP_WithProcessService_Bad(t *core.T) {
	service := &Service{processService: "before"}
	option := WithProcessService(nil)
	err := option(service)

	core.AssertNoError(t, err)
	core.AssertNil(t, service.processService)
}

func TestMCP_WithProcessService_Ugly(t *core.T) {
	service := &Service{}
	payload := map[string]bool{"ok": true}
	err := WithProcessService(payload)(service)

	core.AssertNoError(t, err)
	core.AssertEqual(t, payload, service.processService)
}

func TestMCP_WithWSHub_Good(t *core.T) {
	service := &Service{}
	option := WithWSHub("hub")
	err := option(service)

	core.AssertNoError(t, err)
	core.AssertEqual(t, "hub", service.wsHub)
}

func TestMCP_WithWSHub_Bad(t *core.T) {
	service := &Service{wsHub: "before"}
	option := WithWSHub(nil)
	err := option(service)

	core.AssertNoError(t, err)
	core.AssertNil(t, service.wsHub)
}

func TestMCP_WithWSHub_Ugly(t *core.T) {
	service := &Service{}
	payload := map[string]bool{"connected": true}
	err := WithWSHub(payload)(service)

	core.AssertNoError(t, err)
	core.AssertEqual(t, payload, service.wsHub)
}

func TestMCP_WithSubsystem_Good(t *core.T) {
	service := &Service{}
	sub := ax7Subsystem{}
	err := WithSubsystem(sub)(service)

	core.AssertNoError(t, err)
	core.AssertLen(t, service.subsystems, 1)
}

func TestMCP_WithSubsystem_Bad(t *core.T) {
	service := &Service{}
	err := WithSubsystem(nil)(service)
	got := len(service.subsystems)

	core.AssertNoError(t, err)
	core.AssertEqual(t, 0, got)
}

func TestMCP_WithSubsystem_Ugly(t *core.T) {
	service := &Service{}
	first := ax7Subsystem{}
	second := ax7Subsystem{}

	core.AssertNoError(t, WithSubsystem(first)(service))
	core.AssertNoError(t, WithSubsystem(second)(service))
	core.AssertLen(t, service.subsystems, 2)
}

func TestMCP_Service_WorkspaceRoot_Good(t *core.T) {
	service := &Service{workspaceRoot: "/repo"}
	got := service.WorkspaceRoot()
	want := "/repo"

	core.AssertEqual(t, want, got)
	core.AssertNotEqual(t, "", got)
}

func TestMCP_Service_WorkspaceRoot_Bad(t *core.T) {
	service := &Service{}
	got := service.WorkspaceRoot()
	want := ""

	core.AssertEqual(t, want, got)
	core.AssertEmpty(t, got)
}

func TestMCP_Service_WorkspaceRoot_Ugly(t *core.T) {
	service := &Service{workspaceRoot: ""}
	got := service.WorkspaceRoot()
	unrestricted := got == ""

	core.AssertTrue(t, unrestricted)
	core.AssertEqual(t, "", got)
}

func TestMCP_Service_Tools_Good(t *core.T) {
	handler := typedHandler(func(context.Context, struct{}) (map[string]bool, error) { return map[string]bool{"ok": true}, nil })
	service := &Service{tools: map[string]Tool{"x": {Name: "x", InputSchema: objectSchema(), Handler: handler}}, toolOrder: []string{"x"}}
	records := service.Tools()

	core.AssertLen(t, records, 1)
	core.AssertEqual(t, "x", records[0].Name)
}

func TestMCP_Service_Tools_Bad(t *core.T) {
	service := &Service{tools: map[string]Tool{}, toolOrder: nil}
	records := service.Tools()
	got := len(records)

	core.AssertEqual(t, 0, got)
	core.AssertEmpty(t, records)
}

func TestMCP_Service_Tools_Ugly(t *core.T) {
	handler := typedHandler(func(context.Context, struct{}) (map[string]bool, error) { return map[string]bool{"ok": true}, nil })
	service := &Service{tools: map[string]Tool{"x": {Name: "x", InputSchema: objectSchema(), Handler: handler}}, toolOrder: []string{"x"}}
	records := service.Tools()

	records[0].InputSchema["mutated"] = true
	core.AssertNil(t, service.tools["x"].InputSchema["mutated"])
}

func TestMCP_Service_ToolNames_Good(t *core.T) {
	service := &Service{toolOrder: []string{"a", "b"}}
	names := service.ToolNames()
	got := core.Join(",", names...)

	core.AssertEqual(t, "a,b", got)
	core.AssertLen(t, names, 2)
}

func TestMCP_Service_ToolNames_Bad(t *core.T) {
	service := &Service{}
	names := service.ToolNames()
	got := len(names)

	core.AssertEqual(t, 0, got)
	core.AssertEmpty(t, names)
}

func TestMCP_Service_ToolNames_Ugly(t *core.T) {
	service := &Service{toolOrder: []string{"a"}}
	names := service.ToolNames()
	names[0] = "mutated"

	core.AssertEqual(t, []string{"a"}, service.ToolNames())
	core.AssertEqual(t, []string{"mutated"}, names)
}

func TestMCP_Service_RegisterTool_Good(t *core.T) {
	handler := typedHandler(func(context.Context, struct{}) (map[string]bool, error) { return map[string]bool{"ok": true}, nil })
	service := &Service{tools: map[string]Tool{}}
	err := service.RegisterTool(Tool{Name: "custom", Handler: handler})

	core.AssertNoError(t, err)
	core.AssertEqual(t, []string{"custom"}, service.ToolNames())
}

func TestMCP_Service_RegisterTool_Bad(t *core.T) {
	service := &Service{tools: map[string]Tool{}}
	err := service.RegisterTool(Tool{Name: "", Handler: typedHandler(func(context.Context, struct{}) (map[string]bool, error) { return nil, nil })})
	got := core.ErrorMessage(err)

	core.AssertError(t, err)
	core.AssertContains(t, got, "name is required")
}

func TestMCP_Service_RegisterTool_Ugly(t *core.T) {
	handler := typedHandler(func(context.Context, struct{}) (map[string]bool, error) { return map[string]bool{"ok": true}, nil })
	service := &Service{tools: map[string]Tool{}}
	err := service.RegisterTool(Tool{Name: "custom", Handler: handler})

	core.AssertNoError(t, err)
	core.AssertEqual(t, "object", service.tools["custom"].InputSchema["type"])
}

func TestMCP_Service_RegisterToolFunc_Good(t *core.T) {
	handler := typedHandler(func(context.Context, struct{}) (map[string]bool, error) { return map[string]bool{"ok": true}, nil })
	service := &Service{tools: map[string]Tool{}}
	err := service.RegisterToolFunc("group", "custom", "Custom tool", handler)

	core.AssertNoError(t, err)
	core.AssertEqual(t, "group", service.tools["custom"].Group)
}

func TestMCP_Service_RegisterToolFunc_Bad(t *core.T) {
	service := &Service{tools: map[string]Tool{}}
	err := service.RegisterToolFunc("group", "", "Custom tool", nil)
	got := core.ErrorMessage(err)

	core.AssertError(t, err)
	core.AssertContains(t, got, "name is required")
}

func TestMCP_Service_RegisterToolFunc_Ugly(t *core.T) {
	handler := typedHandler(func(context.Context, struct{}) (map[string]bool, error) { return map[string]bool{"ok": true}, nil })
	service := &Service{tools: map[string]Tool{}}
	err := service.RegisterToolFunc("", "custom", "", handler)

	core.AssertNoError(t, err)
	core.AssertEqual(t, "", service.tools["custom"].Group)
}

func TestMCP_Service_Shutdown_Good(t *core.T) {
	called := false
	service := &Service{subsystems: []Subsystem{ax7Subsystem{called: &called}}}
	err := service.Shutdown(core.Background())

	core.AssertNoError(t, err)
	core.AssertTrue(t, called)
}

func TestMCP_Service_Shutdown_Bad(t *core.T) {
	service := &Service{subsystems: []Subsystem{ax7Subsystem{err: core.AnError}}}
	err := service.Shutdown(core.Background())
	got := core.ErrorMessage(err)

	core.AssertError(t, err)
	core.AssertContains(t, got, core.AnError.Error())
}

func TestMCP_Service_Shutdown_Ugly(t *core.T) {
	service := &Service{processes: map[string]*managedProcess{}}
	err := service.Shutdown(core.Background())
	got := len(service.processes)

	core.AssertNoError(t, err)
	core.AssertEqual(t, 0, got)
}

func TestMCP_Service_HandleFrame_Good(t *core.T) {
	service, err := New(WithWorkspaceRoot(t.TempDir()))
	core.RequireNoError(t, err)
	response, err := service.HandleFrame(core.Background(), []byte(`{"jsonrpc":"2.0","id":1,"method":"ping"}`))

	core.AssertNoError(t, err)
	core.AssertContains(t, string(response), `"result"`)
}

func TestMCP_Service_HandleFrame_Bad(t *core.T) {
	service, err := New(WithWorkspaceRoot(t.TempDir()))
	core.RequireNoError(t, err)
	response, err := service.HandleFrame(core.Background(), []byte(`{bad json`))

	core.AssertError(t, err)
	core.AssertContains(t, string(response), "parse error")
}

func TestMCP_Service_HandleFrame_Ugly(t *core.T) {
	service, err := New(WithWorkspaceRoot(t.TempDir()))
	core.RequireNoError(t, err)
	response, err := service.HandleFrame(core.Background(), []byte(`{"jsonrpc":"2.0","method":"notifications/initialized"}`))

	core.AssertNoError(t, err)
	core.AssertNil(t, response)
}

func TestMCP_Service_ServeStdio_Good(t *core.T) {
	service, err := New(WithWorkspaceRoot(t.TempDir()))
	core.RequireNoError(t, err)
	oldReader, oldWriter := stdioReader, stdioWriter
	defer func() { stdioReader, stdioWriter = oldReader, oldWriter }()

	var output safeBuffer
	stdioReader = core.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}` + "\n")
	stdioWriter = &output
	err = service.ServeStdio(core.Background())

	core.AssertNoError(t, err)
	core.AssertContains(t, output.String(), `"tools"`)
}

func TestMCP_Service_ServeStdio_Bad(t *core.T) {
	service, err := New(WithWorkspaceRoot(t.TempDir()))
	core.RequireNoError(t, err)
	oldReader, oldWriter := stdioReader, stdioWriter
	defer func() { stdioReader, stdioWriter = oldReader, oldWriter }()

	var output safeBuffer
	stdioReader = core.NewReader("{bad json\n")
	stdioWriter = &output
	err = service.ServeStdio(core.Background())

	core.AssertNoError(t, err)
	core.AssertContains(t, output.String(), "parse error")
}

func TestMCP_Service_ServeStdio_Ugly(t *core.T) {
	service, err := New(WithWorkspaceRoot(t.TempDir()))
	core.RequireNoError(t, err)
	oldReader, oldWriter := stdioReader, stdioWriter
	defer func() { stdioReader, stdioWriter = oldReader, oldWriter }()

	stdioReader = core.NewReader("")
	stdioWriter = &safeBuffer{}
	err = service.ServeStdio(core.Background())

	core.AssertNoError(t, err)
	core.AssertEqual(t, []string{}, []string{})
}

func TestMCP_Service_ServeTCP_Good(t *core.T) {
	service, err := New(WithWorkspaceRoot(t.TempDir()))
	core.RequireNoError(t, err)
	addr := reserveTCPAddr(t)
	ctx, cancel := core.WithCancel(core.Background())

	errCh := make(chan error, 1)
	go func() { errCh <- service.ServeTCP(ctx, addr) }()
	waitForTCP(t, addr)
	cancel()
	core.AssertNoError(t, <-errCh)
}

func TestMCP_Service_ServeTCP_Bad(t *core.T) {
	service, err := New(WithWorkspaceRoot(t.TempDir()))
	core.RequireNoError(t, err)
	err = service.ServeTCP(core.Background(), "127.0.0.1:bad")

	core.AssertError(t, err)
	core.AssertContains(t, core.ErrorMessage(err), "listen")
}

func TestMCP_Service_ServeTCP_Ugly(t *core.T) {
	service, err := New(WithWorkspaceRoot(t.TempDir()))
	core.RequireNoError(t, err)
	err = service.ServeTCP(core.Background(), "256.256.256.256:1")

	core.AssertError(t, err)
	core.AssertContains(t, core.ErrorMessage(err), "listen")
}

func TestMCP_Service_ServeUnix_Good(t *core.T) {
	service, err := New(WithWorkspaceRoot(t.TempDir()))
	core.RequireNoError(t, err)
	socketPath := core.Path(t.TempDir(), "mcp.sock")
	ctx, cancel := core.WithCancel(core.Background())

	errCh := make(chan error, 1)
	go func() { errCh <- service.ServeUnix(ctx, socketPath) }()
	waitForUnix(t, socketPath)
	cancel()
	core.AssertNoError(t, <-errCh)
}

func TestMCP_Service_ServeUnix_Bad(t *core.T) {
	service, err := New(WithWorkspaceRoot(t.TempDir()))
	core.RequireNoError(t, err)
	err = service.ServeUnix(core.Background(), "\x00")

	core.AssertError(t, err)
	core.AssertContains(t, core.ErrorMessage(err), "invalid")
}

func TestMCP_Service_ServeUnix_Ugly(t *core.T) {
	service, err := New(WithWorkspaceRoot(t.TempDir()))
	core.RequireNoError(t, err)
	socketPath := core.Path(t.TempDir(), "mcp.sock")
	core.AssertNoError(t, os.WriteFile(socketPath, []byte("stale socket"), 0o600))
	ctx, cancel := core.WithCancel(core.Background())

	errCh := make(chan error, 1)
	go func() { errCh <- service.ServeUnix(ctx, socketPath) }()
	waitForUnix(t, socketPath)
	cancel()
	core.AssertNoError(t, <-errCh)
}

func TestMCP_Service_Run_Good(t *core.T) {
	service, err := New(WithWorkspaceRoot(t.TempDir()))
	core.RequireNoError(t, err)
	oldReader, oldWriter := stdioReader, stdioWriter
	defer func() { stdioReader, stdioWriter = oldReader, oldWriter }()

	var output safeBuffer
	stdioReader = core.NewReader(`{"jsonrpc":"2.0","id":1,"method":"ping"}` + "\n")
	stdioWriter = &output
	err = service.Run(core.Background())

	core.AssertNoError(t, err)
	core.AssertContains(t, output.String(), `"result"`)
}

func TestMCP_Service_Run_Bad(t *core.T) {
	t.Setenv("MCP_ADDR", "127.0.0.1:bad")
	service, err := New(WithWorkspaceRoot(t.TempDir()))
	core.RequireNoError(t, err)

	err = service.Run(core.Background())
	core.AssertError(t, err)
	core.AssertContains(t, core.ErrorMessage(err), "listen")
}

func TestMCP_Service_Run_Ugly(t *core.T) {
	t.Setenv("MCP_UNIX_SOCKET", core.Path(t.TempDir(), "socket-name-that-is-intentionally-too-long-for-a-unix-domain-socket-path-because-the-kernel-limit-is-small"))
	service, err := New(WithWorkspaceRoot(t.TempDir()))
	core.RequireNoError(t, err)

	err = service.Run(core.Background())
	core.AssertError(t, err)
	core.AssertContains(t, core.ErrorMessage(err), "invalid")
}

func TestMCP_Buffer_Write_Good(t *core.T) {
	var buffer safeBuffer
	n, err := buffer.Write([]byte("agent"))
	got := buffer.String()

	core.AssertNoError(t, err)
	core.AssertEqual(t, 5, n)
	core.AssertEqual(t, "agent", got)
}

func TestMCP_Buffer_Write_Bad(t *core.T) {
	var buffer safeBuffer
	n, err := buffer.Write(nil)
	got := buffer.String()

	core.AssertNoError(t, err)
	core.AssertEqual(t, 0, n)
	core.AssertEqual(t, "", got)
}

func TestMCP_Buffer_Write_Ugly(t *core.T) {
	var buffer safeBuffer
	first, firstErr := buffer.Write([]byte("agent"))
	second, secondErr := buffer.Write([]byte("-ready"))

	core.AssertNoError(t, firstErr)
	core.AssertNoError(t, secondErr)
	core.AssertEqual(t, 11, first+second)
}

func TestMCP_Buffer_String_Good(t *core.T) {
	var buffer safeBuffer
	_, err := buffer.Write([]byte("agent"))
	got := buffer.String()

	core.AssertNoError(t, err)
	core.AssertEqual(t, "agent", got)
}

func TestMCP_Buffer_String_Bad(t *core.T) {
	var buffer safeBuffer
	got := buffer.String()
	want := ""

	core.AssertEqual(t, want, got)
	core.AssertEmpty(t, got)
}

func TestMCP_Buffer_String_Ugly(t *core.T) {
	var buffer safeBuffer
	_, err := buffer.Write([]byte("agent"))
	first := buffer.String()

	core.AssertNoError(t, err)
	core.AssertEqual(t, first, buffer.String())
}
