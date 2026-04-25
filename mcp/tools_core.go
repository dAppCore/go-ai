package mcp

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

func (s *Service) registerBuiltInTools() error {
	registrations := []Tool{
		tool("file", "file_read", "Read the contents of a file", typedHandler(s.readFile)),
		tool("file", "file_write", "Write content to a file", typedHandler(s.writeFile)),
		tool("file", "file_delete", "Delete a file or empty directory", typedHandler(s.deleteFile)),
		tool("file", "file_rename", "Rename or move a file", typedHandler(s.renameFile)),
		tool("file", "file_exists", "Check whether a file or directory exists", typedHandler(s.fileExists)),
		tool("file", "file_edit", "Edit a file by replacing text", typedHandler(s.editFile)),
		tool("dir", "dir_list", "List the contents of a directory", typedHandler(s.listDirectory)),
		tool("dir", "dir_create", "Create a directory", typedHandler(s.createDirectory)),
		tool("language", "lang_detect", "Detect the programming language of a file path", typedHandler(s.detectLanguage)),
		tool("language", "lang_list", "List supported programming languages", typedHandler(s.listLanguages)),
		tool("rag", "rag_query", "Query the RAG vector database", typedHandler(s.ragQuery)),
		tool("rag", "rag_ingest", "Ingest files into the RAG vector database", typedHandler(s.ragIngest)),
		tool("rag", "rag_collections", "List RAG collections", typedHandler(s.ragCollections)),
		tool("ml", "ml_generate", "Generate text with an ML backend", typedHandler(s.mlGenerate)),
		tool("ml", "ml_score", "Score a prompt and response", typedHandler(s.mlScore)),
		tool("ml", "ml_probe", "Run inference capability probes", typedHandler(s.mlProbe)),
		tool("ml", "ml_status", "Show ML pipeline status", typedHandler(s.mlStatus)),
		tool("ml", "ml_backends", "List available ML backends", typedHandler(s.mlBackends)),
		tool("metrics", "metrics_record", "Record a metrics event", typedHandler(s.metricsRecord)),
		tool("metrics", "metrics_query", "Query metrics events", typedHandler(s.metricsQuery)),
		tool("process", "process_start", "Start a managed process", typedHandler(s.processStart)),
		tool("process", "process_stop", "Stop a managed process", typedHandler(s.processStop)),
		tool("process", "process_kill", "Kill a managed process", typedHandler(s.processKill)),
		tool("process", "process_list", "List managed processes", typedHandler(s.processList)),
		tool("process", "process_output", "Read managed process output", typedHandler(s.processOutput)),
		tool("process", "process_input", "Write to managed process stdin", typedHandler(s.processInput)),
		tool("websocket", "ws_start", "Start the WebSocket endpoint", typedHandler(s.wsStart)),
		tool("websocket", "ws_info", "Inspect WebSocket endpoint state", typedHandler(s.wsInfo)),
		tool("browser", "webview_connect", "Connect to a browser debug endpoint", typedHandler(s.webviewConnect)),
		tool("browser", "webview_disconnect", "Disconnect from the browser debug endpoint", typedHandler(s.webviewDisconnect)),
		tool("browser", "webview_navigate", "Navigate the browser to a URL", typedHandler(s.webviewNavigate)),
		tool("browser", "webview_click", "Click an element by selector", typedHandler(s.webviewClick)),
		tool("browser", "webview_type", "Type text into an element", typedHandler(s.webviewType)),
		tool("browser", "webview_query", "Query DOM elements by selector", typedHandler(s.webviewQuery)),
		tool("browser", "webview_console", "Read browser console messages", typedHandler(s.webviewConsole)),
		tool("browser", "webview_eval", "Evaluate JavaScript in the browser", typedHandler(s.webviewEval)),
		tool("browser", "webview_screenshot", "Capture a browser screenshot", typedHandler(s.webviewScreenshot)),
		tool("browser", "webview_wait", "Wait for an element by selector", typedHandler(s.webviewWait)),
		tool("ide_chat", "ide_chat_send", "Send a chat message to an IDE session", typedHandler(s.ideChatSend)),
		tool("ide_chat", "ide_chat_history", "Retrieve IDE chat history", typedHandler(s.ideChatHistory)),
		tool("ide_chat", "ide_session_list", "List IDE agent sessions", typedHandler(s.ideSessionList)),
		tool("ide_chat", "ide_session_create", "Create an IDE agent session", typedHandler(s.ideSessionCreate)),
		tool("ide_chat", "ide_plan_status", "Get IDE plan status", typedHandler(s.idePlanStatus)),
		tool("ide_build", "ide_build_status", "Get IDE build status", typedHandler(s.ideBuildStatus)),
		tool("ide_build", "ide_build_list", "List IDE builds", typedHandler(s.ideBuildList)),
		tool("ide_build", "ide_build_logs", "Get IDE build logs", typedHandler(s.ideBuildLogs)),
		tool("ide_dashboard", "ide_dashboard_overview", "Get IDE dashboard overview", typedHandler(s.ideDashboardOverview)),
		tool("ide_dashboard", "ide_dashboard_activity", "Get IDE dashboard activity", typedHandler(s.ideDashboardActivity)),
		tool("ide_dashboard", "ide_dashboard_metrics", "Get IDE dashboard metrics", typedHandler(s.ideDashboardMetrics)),
	}

	for _, registration := range registrations {
		if err := s.RegisterTool(registration); err != nil {
			return err
		}
	}
	return nil
}

func tool(group, name, description string, handler ToolHandler) Tool {
	return Tool{
		Name:        name,
		Description: description,
		Group:       group,
		InputSchema: objectSchema(),
		Handler:     handler,
	}
}

type ReadFileInput struct {
	Path string `json:"path"`
}

type ReadFileOutput struct {
	Content  string `json:"content"`
	Language string `json:"language"`
	Path     string `json:"path"`
}

type WriteFileInput struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

type WriteFileOutput struct {
	Success bool   `json:"success"`
	Path    string `json:"path"`
}

type DeleteFileInput struct {
	Path string `json:"path"`
}

type DeleteFileOutput struct {
	Success bool   `json:"success"`
	Path    string `json:"path"`
}

type RenameFileInput struct {
	OldPath string `json:"oldPath"`
	NewPath string `json:"newPath"`
}

type RenameFileOutput struct {
	Success bool   `json:"success"`
	OldPath string `json:"oldPath"`
	NewPath string `json:"newPath"`
}

type FileExistsInput struct {
	Path string `json:"path"`
}

type FileExistsOutput struct {
	Exists bool   `json:"exists"`
	IsDir  bool   `json:"isDir"`
	Path   string `json:"path"`
}

type EditFileInput struct {
	Path       string `json:"path"`
	OldString  string `json:"old_string"`
	NewString  string `json:"new_string"`
	ReplaceAll bool   `json:"replace_all,omitempty"`
}

type EditFileOutput struct {
	Path         string `json:"path"`
	Success      bool   `json:"success"`
	Replacements int    `json:"replacements"`
}

type ListDirectoryInput struct {
	Path string `json:"path"`
}

type ListDirectoryOutput struct {
	Entries []DirectoryEntry `json:"entries"`
	Path    string           `json:"path"`
}

type DirectoryEntry struct {
	Name  string `json:"name"`
	Path  string `json:"path"`
	IsDir bool   `json:"isDir"`
	Size  int64  `json:"size"`
}

type CreateDirectoryInput struct {
	Path string `json:"path"`
}

type CreateDirectoryOutput struct {
	Success bool   `json:"success"`
	Path    string `json:"path"`
}

type DetectLanguageInput struct {
	Path string `json:"path"`
}

type DetectLanguageOutput struct {
	Language string `json:"language"`
	Path     string `json:"path"`
}

type ListLanguagesInput struct{}

type ListLanguagesOutput struct {
	Languages []LanguageInfo `json:"languages"`
}

type LanguageInfo struct {
	ID         string   `json:"id"`
	Name       string   `json:"name"`
	Extensions []string `json:"extensions"`
}

func (s *Service) readFile(ctx context.Context, input ReadFileInput) (ReadFileOutput, error) {
	path, err := s.resolvePath(input.Path)
	if err != nil {
		return ReadFileOutput{}, err
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return ReadFileOutput{}, err
	}
	return ReadFileOutput{Content: string(content), Language: detectLanguageFromPath(input.Path), Path: input.Path}, nil
}

func (s *Service) writeFile(ctx context.Context, input WriteFileInput) (WriteFileOutput, error) {
	path, err := s.resolvePath(input.Path)
	if err != nil {
		return WriteFileOutput{}, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return WriteFileOutput{}, err
	}
	if err := os.WriteFile(path, []byte(input.Content), 0o644); err != nil {
		return WriteFileOutput{}, err
	}
	return WriteFileOutput{Success: true, Path: input.Path}, nil
}

func (s *Service) deleteFile(ctx context.Context, input DeleteFileInput) (DeleteFileOutput, error) {
	path, err := s.resolvePath(input.Path)
	if err != nil {
		return DeleteFileOutput{}, err
	}
	if err := os.Remove(path); err != nil {
		return DeleteFileOutput{}, err
	}
	return DeleteFileOutput{Success: true, Path: input.Path}, nil
}

func (s *Service) renameFile(ctx context.Context, input RenameFileInput) (RenameFileOutput, error) {
	oldPath, err := s.resolvePath(input.OldPath)
	if err != nil {
		return RenameFileOutput{}, err
	}
	newPath, err := s.resolvePath(input.NewPath)
	if err != nil {
		return RenameFileOutput{}, err
	}
	if err := os.MkdirAll(filepath.Dir(newPath), 0o755); err != nil {
		return RenameFileOutput{}, err
	}
	if err := os.Rename(oldPath, newPath); err != nil {
		return RenameFileOutput{}, err
	}
	return RenameFileOutput{Success: true, OldPath: input.OldPath, NewPath: input.NewPath}, nil
}

func (s *Service) fileExists(ctx context.Context, input FileExistsInput) (FileExistsOutput, error) {
	path, err := s.resolvePath(input.Path)
	if err != nil {
		return FileExistsOutput{Exists: false, Path: input.Path}, nil
	}
	info, err := os.Stat(path)
	if err != nil {
		return FileExistsOutput{Exists: false, Path: input.Path}, nil
	}
	return FileExistsOutput{Exists: true, IsDir: info.IsDir(), Path: input.Path}, nil
}

func (s *Service) editFile(ctx context.Context, input EditFileInput) (EditFileOutput, error) {
	if input.OldString == "" {
		return EditFileOutput{}, fmt.Errorf("%w: old_string is required", errInvalidParams)
	}
	path, err := s.resolvePath(input.Path)
	if err != nil {
		return EditFileOutput{}, err
	}
	contentBytes, err := os.ReadFile(path)
	if err != nil {
		return EditFileOutput{}, err
	}
	content := string(contentBytes)
	replacements := strings.Count(content, input.OldString)
	if replacements == 0 {
		return EditFileOutput{}, fmt.Errorf("old_string not found")
	}
	if input.ReplaceAll {
		content = strings.ReplaceAll(content, input.OldString, input.NewString)
	} else {
		content = strings.Replace(content, input.OldString, input.NewString, 1)
		replacements = 1
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return EditFileOutput{}, err
	}
	return EditFileOutput{Path: input.Path, Success: true, Replacements: replacements}, nil
}

func (s *Service) listDirectory(ctx context.Context, input ListDirectoryInput) (ListDirectoryOutput, error) {
	path, err := s.resolvePath(input.Path)
	if err != nil {
		return ListDirectoryOutput{}, err
	}
	entries, err := os.ReadDir(path)
	if err != nil {
		return ListDirectoryOutput{}, err
	}
	slices.SortFunc(entries, func(a, b os.DirEntry) int {
		return strings.Compare(a.Name(), b.Name())
	})
	out := make([]DirectoryEntry, 0, len(entries))
	for _, entry := range entries {
		info, _ := entry.Info()
		var size int64
		if info != nil && !info.IsDir() {
			size = info.Size()
		}
		out = append(out, DirectoryEntry{
			Name:  entry.Name(),
			Path:  directoryEntryPath(input.Path, entry.Name()),
			IsDir: entry.IsDir(),
			Size:  size,
		})
	}
	return ListDirectoryOutput{Entries: out, Path: input.Path}, nil
}

func (s *Service) createDirectory(ctx context.Context, input CreateDirectoryInput) (CreateDirectoryOutput, error) {
	path, err := s.resolvePath(input.Path)
	if err != nil {
		return CreateDirectoryOutput{}, err
	}
	if err := os.MkdirAll(path, 0o755); err != nil {
		return CreateDirectoryOutput{}, err
	}
	return CreateDirectoryOutput{Success: true, Path: input.Path}, nil
}

func (s *Service) detectLanguage(ctx context.Context, input DetectLanguageInput) (DetectLanguageOutput, error) {
	return DetectLanguageOutput{Language: detectLanguageFromPath(input.Path), Path: input.Path}, nil
}

func (s *Service) listLanguages(ctx context.Context, input ListLanguagesInput) (ListLanguagesOutput, error) {
	return ListLanguagesOutput{Languages: supportedLanguages()}, nil
}

func (s *Service) resolvePath(path string) (string, error) {
	if strings.TrimSpace(path) == "" {
		return "", fmt.Errorf("%w: path is required", errInvalidParams)
	}

	if s.workspaceRoot == "" {
		if filepath.IsAbs(path) {
			return filepath.Clean(path), nil
		}
		abs, err := filepath.Abs(path)
		if err != nil {
			return "", err
		}
		return abs, nil
	}

	var candidate string
	if filepath.IsAbs(path) {
		candidate = filepath.Clean(path)
	} else {
		cleanRelative := strings.TrimPrefix(filepath.Clean(string(filepath.Separator)+path), string(filepath.Separator))
		candidate = filepath.Join(s.workspaceRoot, cleanRelative)
	}
	absCandidate, err := filepath.Abs(candidate)
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(s.workspaceRoot, absCandidate)
	if err != nil {
		return "", err
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("path escapes workspace root: %s", path)
	}
	return absCandidate, nil
}

func directoryEntryPath(dir, name string) string {
	dir = strings.Trim(dir, string(filepath.Separator))
	if dir == "" || dir == "." {
		return name
	}
	return filepath.ToSlash(filepath.Join(dir, name))
}

func detectLanguageFromPath(path string) string {
	if filepath.Base(path) == "Dockerfile" {
		return "dockerfile"
	}
	if lang, ok := languageByExtension[filepath.Ext(path)]; ok {
		return lang
	}
	return "plaintext"
}

var languageByExtension = map[string]string{
	".ts":       "typescript",
	".tsx":      "typescript",
	".js":       "javascript",
	".jsx":      "javascript",
	".go":       "go",
	".py":       "python",
	".rs":       "rust",
	".rb":       "ruby",
	".java":     "java",
	".php":      "php",
	".c":        "c",
	".h":        "c",
	".cpp":      "cpp",
	".hpp":      "cpp",
	".cc":       "cpp",
	".cxx":      "cpp",
	".cs":       "csharp",
	".html":     "html",
	".htm":      "html",
	".css":      "css",
	".scss":     "scss",
	".json":     "json",
	".yaml":     "yaml",
	".yml":      "yaml",
	".xml":      "xml",
	".md":       "markdown",
	".markdown": "markdown",
	".sql":      "sql",
	".sh":       "shell",
	".bash":     "shell",
	".swift":    "swift",
	".kt":       "kotlin",
	".kts":      "kotlin",
}

func supportedLanguages() []LanguageInfo {
	return []LanguageInfo{
		{ID: "typescript", Name: "TypeScript", Extensions: []string{".ts", ".tsx"}},
		{ID: "javascript", Name: "JavaScript", Extensions: []string{".js", ".jsx"}},
		{ID: "go", Name: "Go", Extensions: []string{".go"}},
		{ID: "python", Name: "Python", Extensions: []string{".py"}},
		{ID: "rust", Name: "Rust", Extensions: []string{".rs"}},
		{ID: "ruby", Name: "Ruby", Extensions: []string{".rb"}},
		{ID: "java", Name: "Java", Extensions: []string{".java"}},
		{ID: "php", Name: "PHP", Extensions: []string{".php"}},
		{ID: "c", Name: "C", Extensions: []string{".c", ".h"}},
		{ID: "cpp", Name: "C++", Extensions: []string{".cpp", ".hpp", ".cc", ".cxx"}},
		{ID: "csharp", Name: "C#", Extensions: []string{".cs"}},
		{ID: "html", Name: "HTML", Extensions: []string{".html", ".htm"}},
		{ID: "css", Name: "CSS", Extensions: []string{".css"}},
		{ID: "scss", Name: "SCSS", Extensions: []string{".scss"}},
		{ID: "json", Name: "JSON", Extensions: []string{".json"}},
		{ID: "yaml", Name: "YAML", Extensions: []string{".yaml", ".yml"}},
		{ID: "xml", Name: "XML", Extensions: []string{".xml"}},
		{ID: "markdown", Name: "Markdown", Extensions: []string{".md", ".markdown"}},
		{ID: "sql", Name: "SQL", Extensions: []string{".sql"}},
		{ID: "shell", Name: "Shell", Extensions: []string{".sh", ".bash"}},
		{ID: "swift", Name: "Swift", Extensions: []string{".swift"}},
		{ID: "kotlin", Name: "Kotlin", Extensions: []string{".kt", ".kts"}},
		{ID: "dockerfile", Name: "Dockerfile", Extensions: []string{}},
	}
}
