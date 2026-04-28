package main

import (
	"time"

	. "dappco.re/go"
	"github.com/wailsapp/wails/v3/pkg/application"
)

func TestDesktop_NewAgentRunner_Good(t *T) {
	runner := NewAgentRunner("api", "influx", "db", "m3", "model", "work")
	name := runner.ServiceName()
	running := runner.IsRunning()

	AssertEqual(t, "AgentRunner", name)
	AssertFalse(t, running)
}

func TestDesktop_NewAgentRunner_Bad(t *T) {
	runner := NewAgentRunner("", "", "", "", "", "")
	task := runner.CurrentTask()
	running := runner.IsRunning()

	AssertEqual(t, "", task)
	AssertFalse(t, running)
}

func TestDesktop_NewAgentRunner_Ugly(t *T) {
	runner := NewAgentRunner("api", "influx", "db", "m3", "model", "work")
	runner.running = true
	running := runner.IsRunning()

	AssertTrue(t, running)
	AssertEqual(t, "AgentRunner", runner.ServiceName())
}

func TestDesktop_AgentRunner_ServiceName_Good(t *T) {
	runner := &AgentRunner{}
	got := runner.ServiceName()
	want := "AgentRunner"

	AssertEqual(t, want, got)
	AssertNotEqual(t, "", got)
}

func TestDesktop_AgentRunner_ServiceName_Bad(t *T) {
	var runner *AgentRunner
	got := runner.ServiceName()
	want := "AgentRunner"

	AssertEqual(t, want, got)
	AssertNotEqual(t, "", got)
}

func TestDesktop_AgentRunner_ServiceName_Ugly(t *T) {
	runner := NewAgentRunner("", "", "", "", "", "")
	first := runner.ServiceName()
	second := runner.ServiceName()

	AssertEqual(t, first, second)
	AssertEqual(t, "AgentRunner", first)
}

func TestDesktop_AgentRunner_ServiceStartup_Good(t *T) {
	runner := &AgentRunner{}
	err := runner.ServiceStartup(Background(), application.ServiceOptions{})
	got := runner.ServiceName()

	AssertNoError(t, err)
	AssertEqual(t, "AgentRunner", got)
}

func TestDesktop_AgentRunner_ServiceStartup_Bad(t *T) {
	runner := &AgentRunner{}
	ctx, cancel := WithCancel(Background())
	cancel()

	err := runner.ServiceStartup(ctx, application.ServiceOptions{})
	AssertNoError(t, err)
	AssertFalse(t, runner.IsRunning())
}

func TestDesktop_AgentRunner_ServiceStartup_Ugly(t *T) {
	var runner AgentRunner
	err := runner.ServiceStartup(Background(), application.ServiceOptions{})
	task := runner.CurrentTask()

	AssertNoError(t, err)
	AssertEqual(t, "", task)
}

func TestDesktop_AgentRunner_IsRunning_Good(t *T) {
	runner := &AgentRunner{running: true}
	got := runner.IsRunning()
	want := true

	AssertEqual(t, want, got)
	AssertTrue(t, got)
}

func TestDesktop_AgentRunner_IsRunning_Bad(t *T) {
	runner := &AgentRunner{}
	got := runner.IsRunning()
	want := false

	AssertEqual(t, want, got)
	AssertFalse(t, got)
}

func TestDesktop_AgentRunner_IsRunning_Ugly(t *T) {
	runner := &AgentRunner{running: true}
	runner.Stop()
	got := runner.IsRunning()

	AssertFalse(t, got)
	AssertEqual(t, "", runner.CurrentTask())
}

func TestDesktop_AgentRunner_CurrentTask_Good(t *T) {
	runner := &AgentRunner{task: "scoring"}
	got := runner.CurrentTask()
	want := "scoring"

	AssertEqual(t, want, got)
	AssertNotEqual(t, "", got)
}

func TestDesktop_AgentRunner_CurrentTask_Bad(t *T) {
	runner := &AgentRunner{}
	got := runner.CurrentTask()
	want := ""

	AssertEqual(t, want, got)
	AssertEmpty(t, got)
}

func TestDesktop_AgentRunner_CurrentTask_Ugly(t *T) {
	runner := &AgentRunner{running: true, task: "stopping"}
	runner.Stop()
	got := runner.CurrentTask()

	AssertEqual(t, "", got)
	AssertFalse(t, runner.IsRunning())
}

func TestDesktop_AgentRunner_Start_Good(t *T) {
	runner := &AgentRunner{running: true}
	err := runner.Start()
	running := runner.IsRunning()

	AssertNoError(t, err)
	AssertTrue(t, running)
}

func TestDesktop_AgentRunner_Start_Bad(t *T) {
	runner := &AgentRunner{running: true, task: "already running"}
	err := runner.Start()
	task := runner.CurrentTask()

	AssertNoError(t, err)
	AssertEqual(t, "already running", task)
}

func TestDesktop_AgentRunner_Start_Ugly(t *T) {
	runner := NewAgentRunner("", "", "", "", "", "")
	runner.running = true
	err := runner.Start()

	AssertNoError(t, err)
	AssertTrue(t, runner.IsRunning())
}

func TestDesktop_AgentRunner_Stop_Good(t *T) {
	_, cancel := WithCancel(Background())
	runner := &AgentRunner{running: true, task: "scoring", cancel: cancel}
	runner.Stop()

	AssertFalse(t, runner.IsRunning())
	AssertEqual(t, "", runner.CurrentTask())
}

func TestDesktop_AgentRunner_Stop_Bad(t *T) {
	runner := &AgentRunner{}
	runner.Stop()
	got := runner.IsRunning()

	AssertFalse(t, got)
	AssertEqual(t, "", runner.CurrentTask())
}

func TestDesktop_AgentRunner_Stop_Ugly(t *T) {
	runner := &AgentRunner{running: true, task: "queued"}
	runner.Stop()
	got := runner.CurrentTask()

	AssertEqual(t, "", got)
	AssertFalse(t, runner.IsRunning())
}

func TestDesktop_NewDashboardService_Good(t *T) {
	service := NewDashboardService("http://127.0.0.1:1", "training", "/tmp/db.duckdb")
	name := service.ServiceName()
	snapshot := service.GetSnapshot()

	AssertEqual(t, "DashboardService", name)
	AssertEqual(t, "/tmp/db.duckdb", snapshot.DBPath)
}

func TestDesktop_NewDashboardService_Bad(t *T) {
	service := NewDashboardService("", "", "")
	snapshot := service.GetSnapshot()
	models := service.GetModels()

	AssertEqual(t, "", snapshot.DBPath)
	AssertEmpty(t, models)
}

func TestDesktop_NewDashboardService_Ugly(t *T) {
	service := NewDashboardService("http://127.0.0.1:1", "training", "")
	generation := service.GetGeneration()
	training := service.GetTraining()

	AssertEqual(t, GenerationStats{}, generation)
	AssertEmpty(t, training)
}

func TestDesktop_DashboardService_ServiceName_Good(t *T) {
	service := &DashboardService{}
	got := service.ServiceName()
	want := "DashboardService"

	AssertEqual(t, want, got)
	AssertNotEqual(t, "", got)
}

func TestDesktop_DashboardService_ServiceName_Bad(t *T) {
	var service *DashboardService
	got := service.ServiceName()
	want := "DashboardService"

	AssertEqual(t, want, got)
	AssertNotEqual(t, "", got)
}

func TestDesktop_DashboardService_ServiceName_Ugly(t *T) {
	service := NewDashboardService("", "", "")
	first := service.ServiceName()
	second := service.ServiceName()

	AssertEqual(t, first, second)
	AssertEqual(t, "DashboardService", first)
}

func TestDesktop_DashboardService_ServiceStartup_Good(t *T) {
	service := NewDashboardService("http://127.0.0.1:1", "training", "")
	ctx, cancel := WithCancel(Background())
	cancel()

	err := service.ServiceStartup(ctx, application.ServiceOptions{})
	AssertNoError(t, err)
	AssertEqual(t, "DashboardService", service.ServiceName())
}

func TestDesktop_DashboardService_ServiceStartup_Bad(t *T) {
	service := NewDashboardService("http://127.0.0.1:1", "training", "")
	ctx, cancel := WithCancel(Background())
	cancel()

	err := service.ServiceStartup(ctx, application.ServiceOptions{})
	AssertNoError(t, err)
	AssertEmpty(t, service.GetModels())
}

func TestDesktop_DashboardService_ServiceStartup_Ugly(t *T) {
	service := NewDashboardService("http://127.0.0.1:1", "training", "")
	err := service.ServiceStartup(Background(), application.ServiceOptions{})
	snapshot := service.GetSnapshot()

	AssertNoError(t, err)
	AssertEqual(t, "", snapshot.DBPath)
}

func TestDesktop_DashboardService_GetSnapshot_Good(t *T) {
	service := &DashboardService{dbPath: "/tmp/db.duckdb", lastRefresh: time.Unix(1, 0)}
	service.modelInventory = []ModelInfo{{Name: "model"}}
	snapshot := service.GetSnapshot()

	AssertEqual(t, "/tmp/db.duckdb", snapshot.DBPath)
	AssertLen(t, snapshot.Models, 1)
}

func TestDesktop_DashboardService_GetSnapshot_Bad(t *T) {
	service := &DashboardService{}
	snapshot := service.GetSnapshot()
	got := snapshot.UpdatedAt

	AssertEqual(t, "", snapshot.DBPath)
	AssertNotEqual(t, "", got)
}

func TestDesktop_DashboardService_GetSnapshot_Ugly(t *T) {
	service := &DashboardService{generationStats: GenerationStats{GoldenCompleted: 1}}
	snapshot := service.GetSnapshot()
	got := snapshot.Generation.GoldenCompleted

	AssertEqual(t, 1, got)
	AssertEmpty(t, snapshot.Training)
}

func TestDesktop_DashboardService_GetTraining_Good(t *T) {
	service := &DashboardService{trainingStatus: []TrainingRow{{Model: "m"}}}
	training := service.GetTraining()
	got := training[0].Model

	AssertLen(t, training, 1)
	AssertEqual(t, "m", got)
}

func TestDesktop_DashboardService_GetTraining_Bad(t *T) {
	service := &DashboardService{}
	training := service.GetTraining()
	got := len(training)

	AssertEqual(t, 0, got)
	AssertEmpty(t, training)
}

func TestDesktop_DashboardService_GetTraining_Ugly(t *T) {
	service := &DashboardService{trainingStatus: []TrainingRow{{Model: "m", Loss: 0.5}}}
	training := service.GetTraining()
	training[0].Loss = 0.1

	AssertEqual(t, 0.1, training[0].Loss)
	AssertEqual(t, 0.1, service.trainingStatus[0].Loss)
}

func TestDesktop_DashboardService_GetGeneration_Good(t *T) {
	service := &DashboardService{generationStats: GenerationStats{GoldenCompleted: 3, GoldenTarget: 10}}
	generation := service.GetGeneration()
	got := generation.GoldenCompleted

	AssertEqual(t, 3, got)
	AssertEqual(t, 10, generation.GoldenTarget)
}

func TestDesktop_DashboardService_GetGeneration_Bad(t *T) {
	service := &DashboardService{}
	generation := service.GetGeneration()
	got := generation.GoldenTarget

	AssertEqual(t, 0, got)
	AssertEqual(t, GenerationStats{}, generation)
}

func TestDesktop_DashboardService_GetGeneration_Ugly(t *T) {
	service := &DashboardService{generationStats: GenerationStats{ExpansionPct: 99.5}}
	generation := service.GetGeneration()
	got := generation.ExpansionPct

	AssertEqual(t, 99.5, got)
	AssertEqual(t, 0, generation.GoldenCompleted)
}

func TestDesktop_DashboardService_GetModels_Good(t *T) {
	service := &DashboardService{modelInventory: []ModelInfo{{Name: "m", Status: "scored"}}}
	models := service.GetModels()
	got := models[0].Status

	AssertLen(t, models, 1)
	AssertEqual(t, "scored", got)
}

func TestDesktop_DashboardService_GetModels_Bad(t *T) {
	service := &DashboardService{}
	models := service.GetModels()
	got := len(models)

	AssertEqual(t, 0, got)
	AssertEmpty(t, models)
}

func TestDesktop_DashboardService_GetModels_Ugly(t *T) {
	service := &DashboardService{modelInventory: []ModelInfo{{Name: "m", Accuracy: 0.9}}}
	models := service.GetModels()
	models[0].Accuracy = 0.1

	AssertEqual(t, 0.1, models[0].Accuracy)
	AssertEqual(t, 0.1, service.modelInventory[0].Accuracy)
}

func TestDesktop_DashboardService_Refresh_Good(t *T) {
	service := NewDashboardService("http://127.0.0.1:1", "training", "")
	err := service.Refresh()
	snapshot := service.GetSnapshot()

	AssertNoError(t, err)
	AssertNotEqual(t, "", snapshot.UpdatedAt)
}

func TestDesktop_DashboardService_Refresh_Bad(t *T) {
	var service DashboardService
	AssertPanics(t, func() {
		_ = service.Refresh()
	})
	AssertEqual(t, "", service.dbPath)
}

func TestDesktop_DashboardService_Refresh_Ugly(t *T) {
	service := NewDashboardService("http://127.0.0.1:1", "", "")
	err := service.Refresh()
	generation := service.GetGeneration()

	AssertNoError(t, err)
	AssertEqual(t, GenerationStats{}, generation)
}

func TestDesktop_DashboardService_RunQuery_Good(t *T) {
	service := &DashboardService{}
	rows, err := service.RunQuery("select 1")
	got := ErrorMessage(err)

	AssertNil(t, rows)
	AssertError(t, err)
	AssertContains(t, got, "no database configured")
}

func TestDesktop_DashboardService_RunQuery_Bad(t *T) {
	service := &DashboardService{dbPath: ""}
	rows, err := service.RunQuery("")
	got := ErrorMessage(err)

	AssertNil(t, rows)
	AssertError(t, err)
	AssertContains(t, got, "no database configured")
}

func TestDesktop_DashboardService_RunQuery_Ugly(t *T) {
	service := &DashboardService{dbPath: "/path/that/does/not/exist.duckdb"}
	rows, err := service.RunQuery("select 1")
	got := ErrorMessage(err)

	AssertNil(t, rows)
	AssertError(t, err)
	AssertContains(t, got, "open db")
}

func TestDesktop_NewDockerService_Good(t *T) {
	service := NewDockerService("/tmp/deploy")
	name := service.ServiceName()
	status := service.GetStatus()

	AssertEqual(t, "DockerService", name)
	AssertEqual(t, "/tmp/deploy", status.ComposeDir)
}

func TestDesktop_NewDockerService_Bad(t *T) {
	service := NewDockerService("")
	status := service.GetStatus()
	running := service.IsRunning()

	AssertFalse(t, running)
	AssertNotEqual(t, "", status.ComposeDir)
}

func TestDesktop_NewDockerService_Ugly(t *T) {
	service := NewDockerService("/tmp/deploy")
	service.services["db"] = ContainerStatus{Running: true}
	status := service.GetStatus()

	AssertTrue(t, status.Running)
	AssertLen(t, status.Services, 1)
}

func TestDesktop_DockerService_ServiceName_Good(t *T) {
	service := &DockerService{}
	got := service.ServiceName()
	want := "DockerService"

	AssertEqual(t, want, got)
	AssertNotEqual(t, "", got)
}

func TestDesktop_DockerService_ServiceName_Bad(t *T) {
	var service *DockerService
	got := service.ServiceName()
	want := "DockerService"

	AssertEqual(t, want, got)
	AssertNotEqual(t, "", got)
}

func TestDesktop_DockerService_ServiceName_Ugly(t *T) {
	service := NewDockerService("/tmp/deploy")
	first := service.ServiceName()
	second := service.ServiceName()

	AssertEqual(t, first, second)
	AssertEqual(t, "DockerService", first)
}

func TestDesktop_DockerService_ServiceStartup_Good(t *T) {
	t.Setenv("PATH", "")
	service := NewDockerService(t.TempDir())
	ctx, cancel := WithCancel(Background())
	cancel()

	err := service.ServiceStartup(ctx, application.ServiceOptions{})
	AssertNoError(t, err)
	AssertEqual(t, "DockerService", service.ServiceName())
}

func TestDesktop_DockerService_ServiceStartup_Bad(t *T) {
	t.Setenv("PATH", "")
	service := NewDockerService("")
	ctx, cancel := WithCancel(Background())
	cancel()

	err := service.ServiceStartup(ctx, application.ServiceOptions{})
	AssertNoError(t, err)
	AssertFalse(t, service.IsRunning())
}

func TestDesktop_DockerService_ServiceStartup_Ugly(t *T) {
	t.Setenv("PATH", "")
	service := NewDockerService(t.TempDir())
	err := service.ServiceStartup(Background(), application.ServiceOptions{})
	status := service.GetStatus()

	AssertNoError(t, err)
	AssertFalse(t, status.Running)
}

func TestDesktop_DockerService_Start_Good(t *T) {
	t.Setenv("PATH", "")
	service := NewDockerService(t.TempDir())
	err := service.Start()

	AssertError(t, err)
	AssertContains(t, ErrorMessage(err), "docker")
}

func TestDesktop_DockerService_Start_Bad(t *T) {
	t.Setenv("PATH", "")
	service := NewDockerService("")
	err := service.Start()

	AssertError(t, err)
	AssertContains(t, ErrorMessage(err), "docker")
}

func TestDesktop_DockerService_Start_Ugly(t *T) {
	t.Setenv("PATH", "")
	service := &DockerService{}
	err := service.Start()

	AssertError(t, err)
	AssertContains(t, ErrorMessage(err), "docker")
}

func TestDesktop_DockerService_Stop_Good(t *T) {
	t.Setenv("PATH", "")
	service := NewDockerService(t.TempDir())
	err := service.Stop()

	AssertError(t, err)
	AssertContains(t, ErrorMessage(err), "docker")
}

func TestDesktop_DockerService_Stop_Bad(t *T) {
	t.Setenv("PATH", "")
	service := NewDockerService("")
	err := service.Stop()

	AssertError(t, err)
	AssertContains(t, ErrorMessage(err), "docker")
}

func TestDesktop_DockerService_Stop_Ugly(t *T) {
	t.Setenv("PATH", "")
	service := &DockerService{}
	err := service.Stop()

	AssertError(t, err)
	AssertContains(t, ErrorMessage(err), "docker")
}

func TestDesktop_DockerService_Restart_Good(t *T) {
	t.Setenv("PATH", "")
	service := NewDockerService(t.TempDir())
	err := service.Restart()

	AssertError(t, err)
	AssertContains(t, ErrorMessage(err), "docker")
}

func TestDesktop_DockerService_Restart_Bad(t *T) {
	t.Setenv("PATH", "")
	service := NewDockerService("")
	err := service.Restart()

	AssertError(t, err)
	AssertContains(t, ErrorMessage(err), "docker")
}

func TestDesktop_DockerService_Restart_Ugly(t *T) {
	t.Setenv("PATH", "")
	service := &DockerService{}
	err := service.Restart()

	AssertError(t, err)
	AssertContains(t, ErrorMessage(err), "docker")
}

func TestDesktop_DockerService_StartService_Good(t *T) {
	t.Setenv("PATH", "")
	service := NewDockerService(t.TempDir())
	err := service.StartService("db")

	AssertError(t, err)
	AssertContains(t, ErrorMessage(err), "docker")
}

func TestDesktop_DockerService_StartService_Bad(t *T) {
	t.Setenv("PATH", "")
	service := NewDockerService("")
	err := service.StartService("")

	AssertError(t, err)
	AssertContains(t, ErrorMessage(err), "docker")
}

func TestDesktop_DockerService_StartService_Ugly(t *T) {
	t.Setenv("PATH", "")
	service := &DockerService{}
	err := service.StartService("db")

	AssertError(t, err)
	AssertContains(t, ErrorMessage(err), "docker")
}

func TestDesktop_DockerService_StopService_Good(t *T) {
	t.Setenv("PATH", "")
	service := NewDockerService(t.TempDir())
	err := service.StopService("db")

	AssertError(t, err)
	AssertContains(t, ErrorMessage(err), "docker")
}

func TestDesktop_DockerService_StopService_Bad(t *T) {
	t.Setenv("PATH", "")
	service := NewDockerService("")
	err := service.StopService("")

	AssertError(t, err)
	AssertContains(t, ErrorMessage(err), "docker")
}

func TestDesktop_DockerService_StopService_Ugly(t *T) {
	t.Setenv("PATH", "")
	service := &DockerService{}
	err := service.StopService("db")

	AssertError(t, err)
	AssertContains(t, ErrorMessage(err), "docker")
}

func TestDesktop_DockerService_RestartService_Good(t *T) {
	t.Setenv("PATH", "")
	service := NewDockerService(t.TempDir())
	err := service.RestartService("db")

	AssertError(t, err)
	AssertContains(t, ErrorMessage(err), "docker")
}

func TestDesktop_DockerService_RestartService_Bad(t *T) {
	t.Setenv("PATH", "")
	service := NewDockerService("")
	err := service.RestartService("")

	AssertError(t, err)
	AssertContains(t, ErrorMessage(err), "docker")
}

func TestDesktop_DockerService_RestartService_Ugly(t *T) {
	t.Setenv("PATH", "")
	service := &DockerService{}
	err := service.RestartService("db")

	AssertError(t, err)
	AssertContains(t, ErrorMessage(err), "docker")
}

func TestDesktop_DockerService_Logs_Good(t *T) {
	t.Setenv("PATH", "")
	service := NewDockerService(t.TempDir())
	logs, err := service.Logs("db", 10)

	AssertEqual(t, "", logs)
	AssertError(t, err)
}

func TestDesktop_DockerService_Logs_Bad(t *T) {
	t.Setenv("PATH", "")
	service := NewDockerService("")
	logs, err := service.Logs("", 0)

	AssertEqual(t, "", logs)
	AssertError(t, err)
}

func TestDesktop_DockerService_Logs_Ugly(t *T) {
	t.Setenv("PATH", "")
	service := &DockerService{}
	logs, err := service.Logs("db", -1)

	AssertEqual(t, "", logs)
	AssertError(t, err)
}

func TestDesktop_DockerService_GetStatus_Good(t *T) {
	service := NewDockerService("/tmp/deploy")
	service.services["db"] = ContainerStatus{Running: true}
	status := service.GetStatus()

	AssertTrue(t, status.Running)
	AssertLen(t, status.Services, 1)
}

func TestDesktop_DockerService_GetStatus_Bad(t *T) {
	service := NewDockerService("/tmp/deploy")
	status := service.GetStatus()
	got := status.Running

	AssertFalse(t, got)
	AssertEmpty(t, status.Services)
}

func TestDesktop_DockerService_GetStatus_Ugly(t *T) {
	service := NewDockerService("")
	service.services["db"] = ContainerStatus{Running: false}
	status := service.GetStatus()

	AssertFalse(t, status.Running)
	AssertLen(t, status.Services, 1)
}

func TestDesktop_DockerService_IsRunning_Good(t *T) {
	service := NewDockerService("/tmp/deploy")
	service.services["db"] = ContainerStatus{Running: true}
	got := service.IsRunning()

	AssertTrue(t, got)
	AssertEqual(t, true, got)
}

func TestDesktop_DockerService_IsRunning_Bad(t *T) {
	service := NewDockerService("/tmp/deploy")
	service.services["db"] = ContainerStatus{Running: false}
	got := service.IsRunning()

	AssertFalse(t, got)
	AssertEqual(t, false, got)
}

func TestDesktop_DockerService_IsRunning_Ugly(t *T) {
	service := NewDockerService("/tmp/deploy")
	service.services["db"] = ContainerStatus{Running: false}
	service.services["api"] = ContainerStatus{Running: true}

	AssertTrue(t, service.IsRunning())
	AssertLen(t, service.services, 2)
}

func TestDesktop_DockerService_Pull_Good(t *T) {
	t.Setenv("PATH", "")
	service := NewDockerService(t.TempDir())
	err := service.Pull()

	AssertError(t, err)
	AssertContains(t, ErrorMessage(err), "docker")
}

func TestDesktop_DockerService_Pull_Bad(t *T) {
	t.Setenv("PATH", "")
	service := NewDockerService("")
	err := service.Pull()

	AssertError(t, err)
	AssertContains(t, ErrorMessage(err), "docker")
}

func TestDesktop_DockerService_Pull_Ugly(t *T) {
	t.Setenv("PATH", "")
	service := &DockerService{}
	err := service.Pull()

	AssertError(t, err)
	AssertContains(t, ErrorMessage(err), "docker")
}

func TestDesktop_NewTrayService_Good(t *T) {
	service := NewTrayService(nil)
	name := service.ServiceName()
	snapshot := service.GetSnapshot()

	AssertEqual(t, "TrayService", name)
	AssertEqual(t, TraySnapshot{}, snapshot)
}

func TestDesktop_NewTrayService_Bad(t *T) {
	service := NewTrayService(nil)
	err := service.StartStack()
	got := ErrorMessage(err)

	AssertError(t, err)
	AssertContains(t, got, "docker service")
}

func TestDesktop_NewTrayService_Ugly(t *T) {
	service := NewTrayService(nil)
	service.SetServices(&DashboardService{}, &DockerService{services: map[string]ContainerStatus{}}, &AgentRunner{})
	snapshot := service.GetSnapshot()

	AssertEqual(t, "TrayService", service.ServiceName())
	AssertFalse(t, snapshot.StackRunning)
}

func TestDesktop_TrayService_SetServices_Good(t *T) {
	tray := NewTrayService(nil)
	dashboard := &DashboardService{}
	docker := NewDockerService("/tmp/deploy")

	tray.SetServices(dashboard, docker, &AgentRunner{})
	AssertNotNil(t, tray.dashboard)
	AssertNotNil(t, tray.docker)
}

func TestDesktop_TrayService_SetServices_Bad(t *T) {
	tray := NewTrayService(nil)
	tray.SetServices(nil, nil, nil)
	snapshot := tray.GetSnapshot()

	AssertNil(t, tray.dashboard)
	AssertEqual(t, TraySnapshot{}, snapshot)
}

func TestDesktop_TrayService_SetServices_Ugly(t *T) {
	tray := NewTrayService(nil)
	tray.SetServices(&DashboardService{dbPath: "db"}, NewDockerService("/tmp/deploy"), &AgentRunner{task: "queued"})
	snapshot := tray.GetSnapshot()

	AssertEqual(t, "queued", snapshot.AgentTask)
	AssertEqual(t, "db", tray.dashboard.dbPath)
}

func TestDesktop_TrayService_ServiceName_Good(t *T) {
	tray := &TrayService{}
	got := tray.ServiceName()
	want := "TrayService"

	AssertEqual(t, want, got)
	AssertNotEqual(t, "", got)
}

func TestDesktop_TrayService_ServiceName_Bad(t *T) {
	var tray *TrayService
	got := tray.ServiceName()
	want := "TrayService"

	AssertEqual(t, want, got)
	AssertNotEqual(t, "", got)
}

func TestDesktop_TrayService_ServiceName_Ugly(t *T) {
	tray := NewTrayService(nil)
	first := tray.ServiceName()
	second := tray.ServiceName()

	AssertEqual(t, first, second)
	AssertEqual(t, "TrayService", first)
}

func TestDesktop_TrayService_ServiceStartup_Good(t *T) {
	tray := &TrayService{}
	err := tray.ServiceStartup(Background(), application.ServiceOptions{})
	got := tray.ServiceName()

	AssertNoError(t, err)
	AssertEqual(t, "TrayService", got)
}

func TestDesktop_TrayService_ServiceStartup_Bad(t *T) {
	tray := &TrayService{}
	ctx, cancel := WithCancel(Background())
	cancel()

	err := tray.ServiceStartup(ctx, application.ServiceOptions{})
	AssertNoError(t, err)
	AssertEqual(t, TraySnapshot{}, tray.GetSnapshot())
}

func TestDesktop_TrayService_ServiceStartup_Ugly(t *T) {
	var tray TrayService
	err := tray.ServiceStartup(Background(), application.ServiceOptions{})
	snapshot := tray.GetSnapshot()

	AssertNoError(t, err)
	AssertEqual(t, TraySnapshot{}, snapshot)
}

func TestDesktop_TrayService_ServiceShutdown_Good(t *T) {
	tray := &TrayService{}
	err := tray.ServiceShutdown()
	got := tray.ServiceName()

	AssertNoError(t, err)
	AssertEqual(t, "TrayService", got)
}

func TestDesktop_TrayService_ServiceShutdown_Bad(t *T) {
	var tray TrayService
	err := tray.ServiceShutdown()
	snapshot := tray.GetSnapshot()

	AssertNoError(t, err)
	AssertEqual(t, TraySnapshot{}, snapshot)
}

func TestDesktop_TrayService_ServiceShutdown_Ugly(t *T) {
	tray := NewTrayService(nil)
	err := tray.ServiceShutdown()
	got := tray.GetSnapshot()

	AssertNoError(t, err)
	AssertEqual(t, TraySnapshot{}, got)
}

func TestDesktop_TrayService_GetSnapshot_Good(t *T) {
	tray := NewTrayService(nil)
	tray.SetServices(&DashboardService{modelInventory: []ModelInfo{{Name: "m"}}}, NewDockerService("/tmp/deploy"), &AgentRunner{task: "queued"})
	snapshot := tray.GetSnapshot()

	AssertLen(t, snapshot.Models, 1)
	AssertEqual(t, "queued", snapshot.AgentTask)
}

func TestDesktop_TrayService_GetSnapshot_Bad(t *T) {
	tray := NewTrayService(nil)
	snapshot := tray.GetSnapshot()
	got := snapshot.DockerServices

	AssertEqual(t, 0, got)
	AssertFalse(t, snapshot.StackRunning)
}

func TestDesktop_TrayService_GetSnapshot_Ugly(t *T) {
	docker := NewDockerService("/tmp/deploy")
	docker.services["db"] = ContainerStatus{Running: true}
	tray := NewTrayService(nil)
	tray.SetServices(nil, docker, &AgentRunner{running: true})

	snapshot := tray.GetSnapshot()
	AssertTrue(t, snapshot.StackRunning)
	AssertTrue(t, snapshot.AgentRunning)
}

func TestDesktop_TrayService_StartStack_Good(t *T) {
	tray := NewTrayService(nil)
	err := tray.StartStack()
	got := ErrorMessage(err)

	AssertError(t, err)
	AssertContains(t, got, "docker service")
}

func TestDesktop_TrayService_StartStack_Bad(t *T) {
	t.Setenv("PATH", "")
	tray := NewTrayService(nil)
	tray.SetServices(nil, NewDockerService(t.TempDir()), nil)

	err := tray.StartStack()
	AssertError(t, err)
	AssertContains(t, ErrorMessage(err), "docker")
}

func TestDesktop_TrayService_StartStack_Ugly(t *T) {
	t.Setenv("PATH", "")
	tray := NewTrayService(nil)
	tray.SetServices(nil, &DockerService{}, nil)

	err := tray.StartStack()
	AssertError(t, err)
	AssertContains(t, ErrorMessage(err), "docker")
}

func TestDesktop_TrayService_StopStack_Good(t *T) {
	tray := NewTrayService(nil)
	err := tray.StopStack()
	got := ErrorMessage(err)

	AssertError(t, err)
	AssertContains(t, got, "docker service")
}

func TestDesktop_TrayService_StopStack_Bad(t *T) {
	t.Setenv("PATH", "")
	tray := NewTrayService(nil)
	tray.SetServices(nil, NewDockerService(t.TempDir()), nil)

	err := tray.StopStack()
	AssertError(t, err)
	AssertContains(t, ErrorMessage(err), "docker")
}

func TestDesktop_TrayService_StopStack_Ugly(t *T) {
	t.Setenv("PATH", "")
	tray := NewTrayService(nil)
	tray.SetServices(nil, &DockerService{}, nil)

	err := tray.StopStack()
	AssertError(t, err)
	AssertContains(t, ErrorMessage(err), "docker")
}

func TestDesktop_TrayService_StartAgent_Good(t *T) {
	tray := NewTrayService(nil)
	err := tray.StartAgent()
	got := ErrorMessage(err)

	AssertError(t, err)
	AssertContains(t, got, "agent service")
}

func TestDesktop_TrayService_StartAgent_Bad(t *T) {
	tray := NewTrayService(nil)
	tray.SetServices(nil, nil, &AgentRunner{running: true})
	err := tray.StartAgent()

	AssertNoError(t, err)
	AssertTrue(t, tray.agent.IsRunning())
}

func TestDesktop_TrayService_StartAgent_Ugly(t *T) {
	tray := NewTrayService(nil)
	tray.SetServices(nil, nil, &AgentRunner{running: true, task: "queued"})
	err := tray.StartAgent()

	AssertNoError(t, err)
	AssertEqual(t, "queued", tray.agent.CurrentTask())
}

func TestDesktop_TrayService_StopAgent_Good(t *T) {
	tray := NewTrayService(nil)
	tray.SetServices(nil, nil, &AgentRunner{running: true, task: "queued"})
	tray.StopAgent()

	AssertFalse(t, tray.agent.IsRunning())
	AssertEqual(t, "", tray.agent.CurrentTask())
}

func TestDesktop_TrayService_StopAgent_Bad(t *T) {
	tray := NewTrayService(nil)
	tray.StopAgent()
	snapshot := tray.GetSnapshot()

	AssertEqual(t, TraySnapshot{}, snapshot)
	AssertNil(t, tray.agent)
}

func TestDesktop_TrayService_StopAgent_Ugly(t *T) {
	tray := NewTrayService(nil)
	tray.SetServices(nil, nil, &AgentRunner{})
	tray.StopAgent()

	AssertFalse(t, tray.agent.IsRunning())
	AssertEqual(t, "", tray.agent.CurrentTask())
}
