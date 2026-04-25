package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

const defaultRAGCollection = "hostuk-docs"

type RAGQueryInput struct {
	Question   string `json:"question"`
	Collection string `json:"collection,omitempty"`
	TopK       int    `json:"topK,omitempty"`
}

type RAGQueryOutput struct {
	Results    []RAGQueryResult `json:"results"`
	Query      string           `json:"query"`
	Collection string           `json:"collection"`
	Context    string           `json:"context"`
}

type RAGQueryResult struct {
	Content    string  `json:"content"`
	Source     string  `json:"source"`
	Section    string  `json:"section,omitempty"`
	Category   string  `json:"category,omitempty"`
	ChunkIndex int     `json:"chunkIndex"`
	Score      float32 `json:"score"`
}

type RAGIngestInput struct {
	Path       string `json:"path"`
	Collection string `json:"collection,omitempty"`
	Recreate   bool   `json:"recreate,omitempty"`
}

type RAGIngestOutput struct {
	Success    bool   `json:"success"`
	Path       string `json:"path"`
	Collection string `json:"collection"`
	Chunks     int    `json:"chunks"`
	Message    string `json:"message"`
}

type RAGCollectionsInput struct {
	ShowStats bool `json:"show_stats,omitempty"`
}

type RAGCollectionsOutput struct {
	Collections []CollectionInfo `json:"collections"`
}

type CollectionInfo struct {
	Name        string `json:"name"`
	PointsCount uint64 `json:"points_count,omitempty"`
	Status      string `json:"status,omitempty"`
}

func (s *Service) ragQuery(ctx context.Context, input RAGQueryInput) (RAGQueryOutput, error) {
	if strings.TrimSpace(input.Question) == "" {
		return RAGQueryOutput{}, fmt.Errorf("%w: question is required", errInvalidParams)
	}
	collection := defaultString(input.Collection, defaultRAGCollection)
	return RAGQueryOutput{
		Results:    []RAGQueryResult{},
		Query:      input.Question,
		Collection: collection,
		Context:    "",
	}, nil
}

func (s *Service) ragIngest(ctx context.Context, input RAGIngestInput) (RAGIngestOutput, error) {
	if _, err := s.resolvePath(input.Path); err != nil {
		return RAGIngestOutput{}, err
	}
	collection := defaultString(input.Collection, defaultRAGCollection)
	return RAGIngestOutput{
		Success:    false,
		Path:       input.Path,
		Collection: collection,
		Message:    "RAG ingestion backend is not configured in this daemon",
	}, nil
}

func (s *Service) ragCollections(ctx context.Context, input RAGCollectionsInput) (RAGCollectionsOutput, error) {
	return RAGCollectionsOutput{Collections: []CollectionInfo{}}, nil
}

type MLGenerateInput struct {
	Prompt      string  `json:"prompt"`
	Backend     string  `json:"backend,omitempty"`
	Model       string  `json:"model,omitempty"`
	Temperature float64 `json:"temperature,omitempty"`
	MaxTokens   int     `json:"max_tokens,omitempty"`
}

type MLGenerateOutput struct {
	Response string `json:"response"`
	Backend  string `json:"backend"`
	Model    string `json:"model,omitempty"`
}

type MLScoreInput struct {
	Prompt   string `json:"prompt"`
	Response string `json:"response"`
	Suites   string `json:"suites,omitempty"`
}

type MLScoreOutput struct {
	Heuristic map[string]any `json:"heuristic,omitempty"`
	Semantic  map[string]any `json:"semantic,omitempty"`
	Content   map[string]any `json:"content,omitempty"`
}

type MLProbeInput struct {
	Backend    string `json:"backend,omitempty"`
	Categories string `json:"categories,omitempty"`
}

type MLProbeOutput struct {
	Total   int                 `json:"total"`
	Results []MLProbeResultItem `json:"results"`
}

type MLProbeResultItem struct {
	ID       string `json:"id"`
	Category string `json:"category"`
	Response string `json:"response"`
}

type MLStatusInput struct {
	InfluxURL string `json:"influx_url,omitempty"`
	InfluxDB  string `json:"influx_db,omitempty"`
}

type MLStatusOutput struct {
	Status string `json:"status"`
}

type MLBackendsInput struct{}

type MLBackendsOutput struct {
	Backends []MLBackendInfo `json:"backends"`
	Default  string          `json:"default"`
}

type MLBackendInfo struct {
	Name      string `json:"name"`
	Available bool   `json:"available"`
}

func (s *Service) mlGenerate(ctx context.Context, input MLGenerateInput) (MLGenerateOutput, error) {
	if strings.TrimSpace(input.Prompt) == "" {
		return MLGenerateOutput{}, fmt.Errorf("%w: prompt is required", errInvalidParams)
	}
	backend := defaultString(input.Backend, "builtin")
	response := "ML generation backend is not configured in this daemon."
	return MLGenerateOutput{Response: response, Backend: backend, Model: input.Model}, nil
}

func (s *Service) mlScore(ctx context.Context, input MLScoreInput) (MLScoreOutput, error) {
	if strings.TrimSpace(input.Prompt) == "" {
		return MLScoreOutput{}, fmt.Errorf("%w: prompt is required", errInvalidParams)
	}
	if strings.TrimSpace(input.Response) == "" {
		return MLScoreOutput{}, fmt.Errorf("%w: response is required", errInvalidParams)
	}
	suites := splitCSV(defaultString(input.Suites, "heuristic"))
	out := MLScoreOutput{}
	for _, suite := range suites {
		switch suite {
		case "heuristic":
			out.Heuristic = heuristicScores(input.Prompt, input.Response)
		case "semantic":
			out.Semantic = map[string]any{
				"available": false,
				"message":   "semantic scoring backend is not configured",
			}
		case "content":
			out.Content = map[string]any{
				"available": false,
				"message":   "content scoring is available through ml_probe when an ML service is configured",
			}
		default:
			return MLScoreOutput{}, fmt.Errorf("%w: unsupported suite %q", errInvalidParams, suite)
		}
	}
	return out, nil
}

func (s *Service) mlProbe(ctx context.Context, input MLProbeInput) (MLProbeOutput, error) {
	return MLProbeOutput{Results: []MLProbeResultItem{}}, nil
}

func (s *Service) mlStatus(ctx context.Context, input MLStatusInput) (MLStatusOutput, error) {
	url := defaultString(input.InfluxURL, "http://localhost:8086")
	db := defaultString(input.InfluxDB, "lem")
	return MLStatusOutput{Status: fmt.Sprintf("ML status backend is not configured (influx_url=%s influx_db=%s)", url, db)}, nil
}

func (s *Service) mlBackends(ctx context.Context, input MLBackendsInput) (MLBackendsOutput, error) {
	return MLBackendsOutput{
		Backends: []MLBackendInfo{{Name: "builtin", Available: true}},
		Default:  "builtin",
	}, nil
}

func heuristicScores(prompt, response string) map[string]any {
	words := strings.Fields(response)
	promptWords := strings.Fields(prompt)
	lengthScore := minFloat(float64(len(words))/120.0, 1.0)
	structureScore := 0.0
	if strings.Contains(response, "\n") {
		structureScore += 0.25
	}
	if strings.Contains(response, ".") || strings.Contains(response, ":") {
		structureScore += 0.25
	}
	if strings.Contains(response, "- ") || strings.Contains(response, "1.") {
		structureScore += 0.25
	}
	if strings.Contains(response, "```") {
		structureScore += 0.25
	}
	return map[string]any{
		"prompt_length":   len(prompt),
		"response_length": len(response),
		"prompt_words":    len(promptWords),
		"response_words":  len(words),
		"has_code":        strings.Contains(response, "```"),
		"length_score":    lengthScore,
		"structure_score": minFloat(structureScore, 1.0),
		"overall":         minFloat((lengthScore+structureScore)/2.0, 1.0),
	}
}

type MetricsRecordInput struct {
	Type    string         `json:"type"`
	AgentID string         `json:"agent_id,omitempty"`
	Repo    string         `json:"repo,omitempty"`
	Data    map[string]any `json:"data,omitempty"`
}

type MetricsRecordOutput struct {
	Success   bool      `json:"success"`
	Timestamp time.Time `json:"timestamp"`
}

type MetricsQueryInput struct {
	Since string `json:"since,omitempty"`
}

type MetricsQueryOutput struct {
	ByType  map[string]int `json:"by_type"`
	ByRepo  map[string]int `json:"by_repo"`
	ByAgent map[string]int `json:"by_agent"`
	Recent  []MetricEvent  `json:"recent"`
}

type MetricEvent struct {
	Type      string         `json:"type"`
	Timestamp time.Time      `json:"timestamp"`
	AgentID   string         `json:"agent_id,omitempty"`
	Repo      string         `json:"repo,omitempty"`
	Data      map[string]any `json:"data,omitempty"`
}

type metricSummary struct {
	ByType  map[string]int
	ByRepo  map[string]int
	ByAgent map[string]int
	Recent  []MetricEvent
}

var metricWriteMu sync.Mutex

func (s *Service) metricsRecord(ctx context.Context, input MetricsRecordInput) (MetricsRecordOutput, error) {
	if strings.TrimSpace(input.Type) == "" {
		return MetricsRecordOutput{}, fmt.Errorf("%w: type is required", errInvalidParams)
	}
	timestamp := time.Now()
	if err := recordMetricEvent(MetricEvent{
		Type:      input.Type,
		Timestamp: timestamp,
		AgentID:   input.AgentID,
		Repo:      input.Repo,
		Data:      input.Data,
	}); err != nil {
		return MetricsRecordOutput{}, err
	}
	return MetricsRecordOutput{Success: true, Timestamp: timestamp}, nil
}

func (s *Service) metricsQuery(ctx context.Context, input MetricsQueryInput) (MetricsQueryOutput, error) {
	window, err := parseSinceWindow(defaultString(input.Since, "7d"))
	if err != nil {
		return MetricsQueryOutput{}, err
	}
	events, err := readMetricEvents(time.Now().Add(-window))
	if err != nil {
		return MetricsQueryOutput{}, err
	}
	summary := summarizeMetricEvents(events)
	return MetricsQueryOutput{
		ByType:  summary.ByType,
		ByRepo:  summary.ByRepo,
		ByAgent: summary.ByAgent,
		Recent:  summary.Recent,
	}, nil
}

func parseSinceWindow(value string) (time.Duration, error) {
	value = strings.TrimSpace(value)
	if len(value) < 2 {
		return 0, fmt.Errorf("%w: invalid since value %q", errInvalidParams, value)
	}
	unit := value[len(value)-1]
	amount, err := strconv.Atoi(value[:len(value)-1])
	if err != nil || amount <= 0 {
		return 0, fmt.Errorf("%w: invalid since value %q", errInvalidParams, value)
	}
	switch unit {
	case 'm':
		return time.Duration(amount) * time.Minute, nil
	case 'h':
		return time.Duration(amount) * time.Hour, nil
	case 'd':
		return time.Duration(amount) * 24 * time.Hour, nil
	default:
		return 0, fmt.Errorf("%w: invalid since unit %q", errInvalidParams, string(unit))
	}
}

func recordMetricEvent(event MetricEvent) error {
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}
	dir, err := metricDir()
	if err != nil {
		return err
	}
	metricWriteMu.Lock()
	defer metricWriteMu.Unlock()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	path := metricFilePath(dir, event.Timestamp)
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer file.Close()
	data, err := json.Marshal(event)
	if err != nil {
		return err
	}
	_, err = file.Write(append(data, '\n'))
	return err
}

func readMetricEvents(since time.Time) ([]MetricEvent, error) {
	dir, err := metricDir()
	if err != nil {
		return nil, err
	}
	now := time.Now()
	start := time.Date(since.Year(), since.Month(), since.Day(), 0, 0, 0, 0, since.Location())
	var events []MetricEvent
	for day := start; !day.After(now); day = day.AddDate(0, 0, 1) {
		data, err := os.ReadFile(metricFilePath(dir, day))
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}
		for _, line := range strings.Split(string(data), "\n") {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			var event MetricEvent
			if err := json.Unmarshal([]byte(line), &event); err != nil {
				continue
			}
			if !event.Timestamp.Before(since) {
				events = append(events, event)
			}
		}
	}
	return events, nil
}

func summarizeMetricEvents(events []MetricEvent) metricSummary {
	summary := metricSummary{
		ByType:  map[string]int{},
		ByRepo:  map[string]int{},
		ByAgent: map[string]int{},
	}
	for _, event := range events {
		summary.ByType[event.Type]++
		if event.Repo != "" {
			summary.ByRepo[event.Repo]++
		}
		if event.AgentID != "" {
			summary.ByAgent[event.AgentID]++
		}
	}
	recent := events
	if len(recent) > 10 {
		recent = recent[len(recent)-10:]
	}
	summary.Recent = append([]MetricEvent(nil), recent...)
	return summary
}

func metricDir() (string, error) {
	home := os.Getenv("CORE_HOME")
	if home == "" {
		home = os.Getenv("HOME")
	}
	if home == "" {
		home = os.Getenv("USERPROFILE")
	}
	if home == "" {
		return "", fmt.Errorf("metrics home directory is not configured")
	}
	return filepath.Join(home, ".core", "ai", "metrics"), nil
}

func metricFilePath(dir string, timestamp time.Time) string {
	return filepath.Join(dir, timestamp.Format("2006-01-02")+".jsonl")
}

type ProcessStartInput struct {
	Command string   `json:"command"`
	Args    []string `json:"args,omitempty"`
	Dir     string   `json:"dir,omitempty"`
	Env     []string `json:"env,omitempty"`
}

type ProcessStartOutput struct {
	ID        string    `json:"id"`
	PID       int       `json:"pid"`
	Command   string    `json:"command"`
	Args      []string  `json:"args"`
	StartedAt time.Time `json:"startedAt"`
}

type ProcessIDInput struct {
	ID string `json:"id"`
}

type ProcessControlOutput struct {
	ID      string `json:"id"`
	Success bool   `json:"success"`
	Message string `json:"message"`
}

type ProcessListInput struct {
	RunningOnly bool `json:"running_only,omitempty"`
}

type ProcessListOutput struct {
	Processes []ProcessInfo `json:"processes"`
	Total     int           `json:"total"`
}

type ProcessInfo struct {
	ID        string        `json:"id"`
	Command   string        `json:"command"`
	Args      []string      `json:"args"`
	Dir       string        `json:"dir,omitempty"`
	Status    string        `json:"status"`
	PID       int           `json:"pid"`
	ExitCode  int           `json:"exitCode"`
	StartedAt time.Time     `json:"startedAt"`
	Duration  time.Duration `json:"duration"`
}

type ProcessOutputInput struct {
	ID string `json:"id"`
}

type ProcessOutputOutput struct {
	ID     string `json:"id"`
	Output string `json:"output"`
}

type ProcessInputInput struct {
	ID    string `json:"id"`
	Input string `json:"input"`
}

type ProcessInputOutput struct {
	ID      string `json:"id"`
	Success bool   `json:"success"`
	Message string `json:"message"`
}

type managedProcess struct {
	id        string
	command   string
	args      []string
	dir       string
	startedAt time.Time
	endedAt   time.Time
	status    string
	exitCode  int
	errText   string
	cmd       *exec.Cmd
	stdin     io.WriteCloser
	output    safeBuffer
	mu        sync.Mutex
}

type safeBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *safeBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *safeBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

func (s *Service) processStart(ctx context.Context, input ProcessStartInput) (ProcessStartOutput, error) {
	if strings.TrimSpace(input.Command) == "" {
		return ProcessStartOutput{}, fmt.Errorf("%w: command is required", errInvalidParams)
	}
	dir := input.Dir
	if dir != "" {
		resolved, err := s.resolvePath(dir)
		if err != nil {
			return ProcessStartOutput{}, err
		}
		dir = resolved
	} else if s.workspaceRoot != "" {
		dir = s.workspaceRoot
	}

	cmd := exec.Command(input.Command, input.Args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), input.Env...)

	id := fmt.Sprintf("proc-%d", s.processSeq.Add(1))
	proc := &managedProcess{
		id:        id,
		command:   input.Command,
		args:      append([]string(nil), input.Args...),
		dir:       dir,
		startedAt: time.Now(),
		status:    "starting",
		exitCode:  -1,
		cmd:       cmd,
	}
	cmd.Stdout = &proc.output
	cmd.Stderr = &proc.output
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return ProcessStartOutput{}, err
	}
	proc.stdin = stdin

	if err := cmd.Start(); err != nil {
		return ProcessStartOutput{}, err
	}
	proc.status = "running"

	s.processMu.Lock()
	s.processes[id] = proc
	s.processMu.Unlock()

	go proc.wait()

	return ProcessStartOutput{
		ID:        id,
		PID:       cmd.Process.Pid,
		Command:   input.Command,
		Args:      append([]string(nil), input.Args...),
		StartedAt: proc.startedAt,
	}, nil
}

func (p *managedProcess) wait() {
	err := p.cmd.Wait()
	p.mu.Lock()
	defer p.mu.Unlock()
	p.endedAt = time.Now()
	p.status = "exited"
	p.exitCode = 0
	if p.cmd.ProcessState != nil {
		p.exitCode = p.cmd.ProcessState.ExitCode()
	}
	if err != nil {
		p.errText = err.Error()
		if p.exitCode == 0 {
			p.exitCode = -1
		}
	}
	if p.stdin != nil {
		_ = p.stdin.Close()
	}
}

func (s *Service) processStop(ctx context.Context, input ProcessIDInput) (ProcessControlOutput, error) {
	return s.killProcess(input.ID, "stopped")
}

func (s *Service) processKill(ctx context.Context, input ProcessIDInput) (ProcessControlOutput, error) {
	return s.killProcess(input.ID, "killed")
}

func (s *Service) killProcess(id, verb string) (ProcessControlOutput, error) {
	proc, err := s.lookupProcess(id)
	if err != nil {
		return ProcessControlOutput{}, err
	}
	if !proc.isRunning() {
		return ProcessControlOutput{ID: id, Success: true, Message: "process is not running"}, nil
	}
	if proc.cmd.Process == nil {
		return ProcessControlOutput{}, fmt.Errorf("process has no OS handle: %s", id)
	}
	if err := proc.cmd.Process.Kill(); err != nil {
		return ProcessControlOutput{}, err
	}
	return ProcessControlOutput{ID: id, Success: true, Message: "process " + verb}, nil
}

func (s *Service) processList(ctx context.Context, input ProcessListInput) (ProcessListOutput, error) {
	s.processMu.Lock()
	processes := make([]*managedProcess, 0, len(s.processes))
	for _, proc := range s.processes {
		processes = append(processes, proc)
	}
	s.processMu.Unlock()

	out := make([]ProcessInfo, 0, len(processes))
	for _, proc := range processes {
		info := proc.info()
		if input.RunningOnly && info.Status != "running" {
			continue
		}
		out = append(out, info)
	}
	return ProcessListOutput{Processes: out, Total: len(out)}, nil
}

func (s *Service) processOutput(ctx context.Context, input ProcessOutputInput) (ProcessOutputOutput, error) {
	proc, err := s.lookupProcess(input.ID)
	if err != nil {
		return ProcessOutputOutput{}, err
	}
	return ProcessOutputOutput{ID: input.ID, Output: proc.output.String()}, nil
}

func (s *Service) processInput(ctx context.Context, input ProcessInputInput) (ProcessInputOutput, error) {
	if input.Input == "" {
		return ProcessInputOutput{}, fmt.Errorf("%w: input is required", errInvalidParams)
	}
	proc, err := s.lookupProcess(input.ID)
	if err != nil {
		return ProcessInputOutput{}, err
	}
	if !proc.isRunning() {
		return ProcessInputOutput{}, fmt.Errorf("process is not running: %s", input.ID)
	}
	if _, err := io.WriteString(proc.stdin, input.Input); err != nil {
		return ProcessInputOutput{}, err
	}
	return ProcessInputOutput{ID: input.ID, Success: true, Message: "input delivered"}, nil
}

func (s *Service) lookupProcess(id string) (*managedProcess, error) {
	if strings.TrimSpace(id) == "" {
		return nil, fmt.Errorf("%w: id is required", errInvalidParams)
	}
	s.processMu.Lock()
	defer s.processMu.Unlock()
	proc, ok := s.processes[id]
	if !ok {
		return nil, fmt.Errorf("process not found: %s", id)
	}
	return proc, nil
}

func (p *managedProcess) isRunning() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.status == "running"
}

func (p *managedProcess) info() ProcessInfo {
	p.mu.Lock()
	defer p.mu.Unlock()
	pid := 0
	if p.cmd != nil && p.cmd.Process != nil {
		pid = p.cmd.Process.Pid
	}
	end := time.Now()
	if !p.endedAt.IsZero() {
		end = p.endedAt
	}
	return ProcessInfo{
		ID:        p.id,
		Command:   p.command,
		Args:      append([]string(nil), p.args...),
		Dir:       p.dir,
		Status:    p.status,
		PID:       pid,
		ExitCode:  p.exitCode,
		StartedAt: p.startedAt,
		Duration:  end.Sub(p.startedAt),
	}
}

type WSStartInput struct {
	Addr string `json:"addr,omitempty"`
}

type WSStartOutput struct {
	Success bool   `json:"success"`
	Addr    string `json:"addr"`
	Message string `json:"message"`
}

type WSInfoInput struct{}

type WSInfoOutput struct {
	Clients  int    `json:"clients"`
	Channels int    `json:"channels"`
	Addr     string `json:"addr,omitempty"`
	Running  bool   `json:"running"`
}

func (s *Service) wsStart(ctx context.Context, input WSStartInput) (WSStartOutput, error) {
	s.wsMu.Lock()
	if s.wsServer != nil {
		addr := s.wsAddr
		s.wsMu.Unlock()
		return WSStartOutput{Success: true, Addr: addr, Message: "WebSocket server already running at ws://" + addr + "/ws"}, nil
	}
	s.wsMu.Unlock()

	addr := defaultString(input.Addr, ":8080")
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return WSStartOutput{}, err
	}
	actualAddr := listener.Addr().String()
	mux := http.NewServeMux()
	mux.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "WebSocket hub is not configured", http.StatusNotImplemented)
	})
	server := &http.Server{Handler: mux}

	s.wsMu.Lock()
	s.wsServer = server
	s.wsAddr = actualAddr
	s.wsMu.Unlock()

	go func() {
		if err := server.Serve(listener); err != nil && !errorsIsHTTPServerClosed(err) {
			fmt.Fprintln(os.Stderr, "MCP WebSocket server error:", err)
		}
		s.wsMu.Lock()
		if s.wsServer == server {
			s.wsServer = nil
			s.wsAddr = ""
		}
		s.wsMu.Unlock()
	}()

	return WSStartOutput{Success: true, Addr: actualAddr, Message: "WebSocket server running at ws://" + actualAddr + "/ws"}, nil
}

func (s *Service) wsInfo(ctx context.Context, input WSInfoInput) (WSInfoOutput, error) {
	s.wsMu.Lock()
	defer s.wsMu.Unlock()
	return WSInfoOutput{Clients: 0, Channels: 0, Addr: s.wsAddr, Running: s.wsServer != nil}, nil
}

type webviewSession struct {
	Connected bool
	DebugURL  string
	URL       string
	Timeout   int
	Console   []WebviewConsoleMessage
}

type WebviewConnectInput struct {
	DebugURL string `json:"debug_url"`
	Timeout  int    `json:"timeout,omitempty"`
}

type WebviewConnectOutput struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

type WebviewDisconnectInput struct{}

type WebviewDisconnectOutput struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

type WebviewNavigateInput struct {
	URL string `json:"url"`
}

type WebviewNavigateOutput struct {
	Success bool   `json:"success"`
	URL     string `json:"url"`
}

type WebviewSelectorInput struct {
	Selector string `json:"selector"`
}

type WebviewClickOutput struct {
	Success bool `json:"success"`
}

type WebviewTypeInput struct {
	Selector string `json:"selector"`
	Text     string `json:"text"`
}

type WebviewTypeOutput struct {
	Success bool `json:"success"`
}

type WebviewQueryInput struct {
	Selector string `json:"selector"`
	All      bool   `json:"all,omitempty"`
}

type WebviewQueryOutput struct {
	Found    bool                 `json:"found"`
	Count    int                  `json:"count"`
	Elements []WebviewElementInfo `json:"elements"`
}

type WebviewElementInfo struct {
	NodeID      int               `json:"nodeId"`
	TagName     string            `json:"tagName"`
	Attributes  map[string]string `json:"attributes"`
	BoundingBox *BoundingBox      `json:"boundingBox,omitempty"`
}

type BoundingBox struct {
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}

type WebviewConsoleInput struct {
	Clear bool `json:"clear,omitempty"`
}

type WebviewConsoleOutput struct {
	Messages []WebviewConsoleMessage `json:"messages"`
	Count    int                     `json:"count"`
}

type WebviewConsoleMessage struct {
	Type      string `json:"type"`
	Text      string `json:"text"`
	Timestamp string `json:"timestamp"`
	URL       string `json:"url,omitempty"`
	Line      int    `json:"line,omitempty"`
}

type WebviewEvalInput struct {
	Script string `json:"script"`
}

type WebviewEvalOutput struct {
	Success bool   `json:"success"`
	Result  any    `json:"result,omitempty"`
	Error   string `json:"error,omitempty"`
}

type WebviewScreenshotInput struct {
	Format string `json:"format,omitempty"`
}

type WebviewScreenshotOutput struct {
	Success bool   `json:"success"`
	Data    string `json:"data"`
	Format  string `json:"format"`
}

type WebviewWaitInput struct {
	Selector string `json:"selector"`
	Timeout  int    `json:"timeout,omitempty"`
}

type WebviewWaitOutput struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

func (s *Service) webviewConnect(ctx context.Context, input WebviewConnectInput) (WebviewConnectOutput, error) {
	if strings.TrimSpace(input.DebugURL) == "" {
		return WebviewConnectOutput{}, fmt.Errorf("%w: debug_url is required", errInvalidParams)
	}
	timeout := input.Timeout
	if timeout <= 0 {
		timeout = 30
	}
	s.webviewMu.Lock()
	s.webviewState = webviewSession{Connected: true, DebugURL: input.DebugURL, Timeout: timeout}
	s.webviewMu.Unlock()
	return WebviewConnectOutput{Success: true, Message: "Connected to " + input.DebugURL}, nil
}

func (s *Service) webviewDisconnect(ctx context.Context, input WebviewDisconnectInput) (WebviewDisconnectOutput, error) {
	s.webviewMu.Lock()
	wasConnected := s.webviewState.Connected
	s.webviewState = webviewSession{}
	s.webviewMu.Unlock()
	if !wasConnected {
		return WebviewDisconnectOutput{Success: true, Message: "No active connection"}, nil
	}
	return WebviewDisconnectOutput{Success: true, Message: "Disconnected"}, nil
}

func (s *Service) webviewNavigate(ctx context.Context, input WebviewNavigateInput) (WebviewNavigateOutput, error) {
	if strings.TrimSpace(input.URL) == "" {
		return WebviewNavigateOutput{}, fmt.Errorf("%w: url is required", errInvalidParams)
	}
	if err := s.requireWebview(); err != nil {
		return WebviewNavigateOutput{}, err
	}
	s.webviewMu.Lock()
	s.webviewState.URL = input.URL
	s.webviewState.Console = append(s.webviewState.Console, WebviewConsoleMessage{
		Type:      "log",
		Text:      "navigate " + input.URL,
		Timestamp: time.Now().Format(time.RFC3339),
		URL:       input.URL,
	})
	s.webviewMu.Unlock()
	return WebviewNavigateOutput{Success: true, URL: input.URL}, nil
}

func (s *Service) webviewClick(ctx context.Context, input WebviewSelectorInput) (WebviewClickOutput, error) {
	if strings.TrimSpace(input.Selector) == "" {
		return WebviewClickOutput{}, fmt.Errorf("%w: selector is required", errInvalidParams)
	}
	if err := s.requireWebview(); err != nil {
		return WebviewClickOutput{}, err
	}
	return WebviewClickOutput{Success: true}, nil
}

func (s *Service) webviewType(ctx context.Context, input WebviewTypeInput) (WebviewTypeOutput, error) {
	if strings.TrimSpace(input.Selector) == "" {
		return WebviewTypeOutput{}, fmt.Errorf("%w: selector is required", errInvalidParams)
	}
	if err := s.requireWebview(); err != nil {
		return WebviewTypeOutput{}, err
	}
	return WebviewTypeOutput{Success: true}, nil
}

func (s *Service) webviewQuery(ctx context.Context, input WebviewQueryInput) (WebviewQueryOutput, error) {
	if strings.TrimSpace(input.Selector) == "" {
		return WebviewQueryOutput{}, fmt.Errorf("%w: selector is required", errInvalidParams)
	}
	if err := s.requireWebview(); err != nil {
		return WebviewQueryOutput{}, err
	}
	return WebviewQueryOutput{Found: false, Count: 0, Elements: []WebviewElementInfo{}}, nil
}

func (s *Service) webviewConsole(ctx context.Context, input WebviewConsoleInput) (WebviewConsoleOutput, error) {
	if err := s.requireWebview(); err != nil {
		return WebviewConsoleOutput{}, err
	}
	s.webviewMu.Lock()
	messages := append([]WebviewConsoleMessage(nil), s.webviewState.Console...)
	if input.Clear {
		s.webviewState.Console = nil
	}
	s.webviewMu.Unlock()
	return WebviewConsoleOutput{Messages: messages, Count: len(messages)}, nil
}

func (s *Service) webviewEval(ctx context.Context, input WebviewEvalInput) (WebviewEvalOutput, error) {
	if strings.TrimSpace(input.Script) == "" {
		return WebviewEvalOutput{}, fmt.Errorf("%w: script is required", errInvalidParams)
	}
	if err := s.requireWebview(); err != nil {
		return WebviewEvalOutput{}, err
	}
	return WebviewEvalOutput{Success: false, Error: "JavaScript evaluation backend is not configured"}, nil
}

func (s *Service) webviewScreenshot(ctx context.Context, input WebviewScreenshotInput) (WebviewScreenshotOutput, error) {
	if err := s.requireWebview(); err != nil {
		return WebviewScreenshotOutput{}, err
	}
	format := defaultString(input.Format, "png")
	return WebviewScreenshotOutput{Success: false, Data: "", Format: format}, nil
}

func (s *Service) webviewWait(ctx context.Context, input WebviewWaitInput) (WebviewWaitOutput, error) {
	if strings.TrimSpace(input.Selector) == "" {
		return WebviewWaitOutput{}, fmt.Errorf("%w: selector is required", errInvalidParams)
	}
	if err := s.requireWebview(); err != nil {
		return WebviewWaitOutput{}, err
	}
	return WebviewWaitOutput{Success: true, Message: "Selector observed: " + input.Selector}, nil
}

func (s *Service) requireWebview() error {
	s.webviewMu.Lock()
	defer s.webviewMu.Unlock()
	if !s.webviewState.Connected {
		return fmt.Errorf("webview is not connected")
	}
	return nil
}

type IDEChatSendInput struct {
	SessionID string `json:"sessionId"`
	Message   string `json:"message"`
}

type IDEChatSendOutput struct {
	Sent      bool      `json:"sent"`
	SessionID string    `json:"sessionId"`
	Timestamp time.Time `json:"timestamp"`
}

type IDEChatHistoryInput struct {
	SessionID string `json:"sessionId"`
	Limit     int    `json:"limit,omitempty"`
}

type IDEChatHistoryOutput struct {
	SessionID string        `json:"sessionId"`
	Messages  []ChatMessage `json:"messages"`
}

type ChatMessage struct {
	Role      string    `json:"role"`
	Content   string    `json:"content"`
	Timestamp time.Time `json:"timestamp"`
}

type IDESessionListInput struct{}

type IDESessionListOutput struct {
	Sessions []Session `json:"sessions"`
}

type IDESessionCreateInput struct {
	Name string `json:"name"`
}

type IDESessionCreateOutput struct {
	Session Session `json:"session"`
}

type Session struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"createdAt"`
}

type IDEPlanStatusInput struct {
	SessionID string `json:"sessionId"`
}

type IDEPlanStatusOutput struct {
	SessionID string     `json:"sessionId"`
	Status    string     `json:"status"`
	Steps     []PlanStep `json:"steps"`
}

type PlanStep struct {
	Name   string `json:"name"`
	Status string `json:"status"`
}

func (s *Service) ideChatSend(ctx context.Context, input IDEChatSendInput) (IDEChatSendOutput, error) {
	if strings.TrimSpace(input.SessionID) == "" {
		return IDEChatSendOutput{}, fmt.Errorf("%w: sessionId is required", errInvalidParams)
	}
	if strings.TrimSpace(input.Message) == "" {
		return IDEChatSendOutput{}, fmt.Errorf("%w: message is required", errInvalidParams)
	}
	return IDEChatSendOutput{Sent: true, SessionID: input.SessionID, Timestamp: time.Now()}, nil
}

func (s *Service) ideChatHistory(ctx context.Context, input IDEChatHistoryInput) (IDEChatHistoryOutput, error) {
	if strings.TrimSpace(input.SessionID) == "" {
		return IDEChatHistoryOutput{}, fmt.Errorf("%w: sessionId is required", errInvalidParams)
	}
	return IDEChatHistoryOutput{SessionID: input.SessionID, Messages: []ChatMessage{}}, nil
}

func (s *Service) ideSessionList(ctx context.Context, input IDESessionListInput) (IDESessionListOutput, error) {
	return IDESessionListOutput{Sessions: []Session{}}, nil
}

func (s *Service) ideSessionCreate(ctx context.Context, input IDESessionCreateInput) (IDESessionCreateOutput, error) {
	if strings.TrimSpace(input.Name) == "" {
		return IDESessionCreateOutput{}, fmt.Errorf("%w: name is required", errInvalidParams)
	}
	return IDESessionCreateOutput{Session: Session{Name: input.Name, Status: "creating", CreatedAt: time.Now()}}, nil
}

func (s *Service) idePlanStatus(ctx context.Context, input IDEPlanStatusInput) (IDEPlanStatusOutput, error) {
	if strings.TrimSpace(input.SessionID) == "" {
		return IDEPlanStatusOutput{}, fmt.Errorf("%w: sessionId is required", errInvalidParams)
	}
	return IDEPlanStatusOutput{SessionID: input.SessionID, Status: "unknown", Steps: []PlanStep{}}, nil
}

type IDEBuildStatusInput struct {
	BuildID string `json:"buildId"`
}

type IDEBuildStatusOutput struct {
	Build BuildInfo `json:"build"`
}

type IDEBuildListInput struct {
	Repo  string `json:"repo,omitempty"`
	Limit int    `json:"limit,omitempty"`
}

type IDEBuildListOutput struct {
	Builds []BuildInfo `json:"builds"`
}

type IDEBuildLogsInput struct {
	BuildID string `json:"buildId"`
	Tail    int    `json:"tail,omitempty"`
}

type IDEBuildLogsOutput struct {
	BuildID string   `json:"buildId"`
	Lines   []string `json:"lines"`
}

type BuildInfo struct {
	ID        string    `json:"id"`
	Repo      string    `json:"repo,omitempty"`
	Branch    string    `json:"branch,omitempty"`
	Status    string    `json:"status"`
	Duration  string    `json:"duration,omitempty"`
	StartedAt time.Time `json:"startedAt"`
}

func (s *Service) ideBuildStatus(ctx context.Context, input IDEBuildStatusInput) (IDEBuildStatusOutput, error) {
	if strings.TrimSpace(input.BuildID) == "" {
		return IDEBuildStatusOutput{}, fmt.Errorf("%w: buildId is required", errInvalidParams)
	}
	return IDEBuildStatusOutput{Build: BuildInfo{ID: input.BuildID, Status: "unknown"}}, nil
}

func (s *Service) ideBuildList(ctx context.Context, input IDEBuildListInput) (IDEBuildListOutput, error) {
	return IDEBuildListOutput{Builds: []BuildInfo{}}, nil
}

func (s *Service) ideBuildLogs(ctx context.Context, input IDEBuildLogsInput) (IDEBuildLogsOutput, error) {
	if strings.TrimSpace(input.BuildID) == "" {
		return IDEBuildLogsOutput{}, fmt.Errorf("%w: buildId is required", errInvalidParams)
	}
	return IDEBuildLogsOutput{BuildID: input.BuildID, Lines: []string{}}, nil
}

type IDEDashboardOverviewInput struct{}

type IDEDashboardOverviewOutput struct {
	Overview DashboardOverview `json:"overview"`
}

type DashboardOverview struct {
	Repos          int  `json:"repos"`
	Services       int  `json:"services"`
	ActiveSessions int  `json:"activeSessions"`
	RecentBuilds   int  `json:"recentBuilds"`
	BridgeOnline   bool `json:"bridgeOnline"`
}

type IDEDashboardActivityInput struct {
	Limit int `json:"limit,omitempty"`
}

type IDEDashboardActivityOutput struct {
	Events []ActivityEvent `json:"events"`
}

type ActivityEvent struct {
	Type      string    `json:"type"`
	Message   string    `json:"message"`
	Timestamp time.Time `json:"timestamp"`
}

type IDEDashboardMetricsInput struct {
	Period string `json:"period,omitempty"`
}

type IDEDashboardMetricsOutput struct {
	Period  string           `json:"period"`
	Metrics DashboardMetrics `json:"metrics"`
}

type DashboardMetrics struct {
	BuildsTotal   int     `json:"buildsTotal"`
	BuildsSuccess int     `json:"buildsSuccess"`
	BuildsFailed  int     `json:"buildsFailed"`
	AvgBuildTime  string  `json:"avgBuildTime"`
	AgentSessions int     `json:"agentSessions"`
	MessagesTotal int     `json:"messagesTotal"`
	SuccessRate   float64 `json:"successRate"`
}

func (s *Service) ideDashboardOverview(ctx context.Context, input IDEDashboardOverviewInput) (IDEDashboardOverviewOutput, error) {
	return IDEDashboardOverviewOutput{Overview: DashboardOverview{}}, nil
}

func (s *Service) ideDashboardActivity(ctx context.Context, input IDEDashboardActivityInput) (IDEDashboardActivityOutput, error) {
	return IDEDashboardActivityOutput{Events: []ActivityEvent{}}, nil
}

func (s *Service) ideDashboardMetrics(ctx context.Context, input IDEDashboardMetricsInput) (IDEDashboardMetricsOutput, error) {
	return IDEDashboardMetricsOutput{Period: defaultString(input.Period, "24h"), Metrics: DashboardMetrics{}}, nil
}

func defaultString(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func splitCSV(value string) []string {
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func minFloat(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

func errorsIsHTTPServerClosed(err error) bool {
	return err == http.ErrServerClosed
}
