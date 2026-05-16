# Sonar sweeps — core_go-ai findings

195 findings across 19 rules. One rule per commit; fix every line listed under each rule.

## CRITICAL

### go:S1192 — String literals should not be duplicated (110×, code smell)

- `go/ai/ai_test.go:32` — Define a constant instead of duplicating this literal "agent-1" 4 times.
- `go/ai/ai_test.go:33` — Define a constant instead of duplicating this literal "core/go-ai" 6 times.
- `go/ai/ai_test.go:115` — Define a constant instead of duplicating this literal "core/go-rag" 3 times.
- `go/ai/book_state_demo.go:143` — Define a constant instead of duplicating this literal "ai.NewBookStateDemo" 5 times.
- `go/ai/book_state_demo.go:191` — Define a constant instead of duplicating this literal "ai.BookStateDemo.Ask" 6 times.
- `go/ai/book_state_demo_example_test.go:79` — Define a constant instead of duplicating this literal "What lesson?" 3 times.
- `go/ai/book_state_demo_http.go:40` — Define a constant instead of duplicating this literal "method not allowed" 3 times.
- `go/ai/book_state_demo_test.go:67` — Define a constant instead of duplicating this literal "What lesson?" 3 times.
- `go/ai/book_state_demo_test.go:129` — Define a constant instead of duplicating this literal "memvid://entry" 7 times.
- `go/ai/book_state_demo_test.go:130` — Define a constant instead of duplicating this literal "memvid://bundle" 8 times.
- `go/ai/book_state_demo_test.go:158` — Define a constant instead of duplicating this literal "sha256:test" 4 times.
- `go/ai/metrics.go:79` — Define a constant instead of duplicating this literal "record event" 8 times.
- `go/ai/metrics.go:121` — Define a constant instead of duplicating this literal "read events" 3 times.
- `go/ai/metrics_test.go:53` — Define a constant instead of duplicating this literal "security.scan" 4 times.
- `go/ai/metrics_test.go:53` — Define a constant instead of duplicating this literal "core/go-ai" 9 times.
- `go/ai/metrics_test.go:89` — Define a constant instead of duplicating this literal "core/go-rag" 3 times.
- `go/ai/metrics_test.go:123` — Define a constant instead of duplicating this literal "agent-1" 6 times.
- `go/ai/provider_router.go:132` — Define a constant instead of duplicating this literal "ai.ProviderRouter.Chat" 8 times.
- `go/ai/provider_router_example_test.go:15` — Define a constant instead of duplicating this literal "gemma-test" 4 times.
- `go/ai/provider_router_test.go:20` — Define a constant instead of duplicating this literal "NewProviderRouter() error = %s" 3 times.
- `go/ai/provider_router_test.go:42` — Define a constant instead of duplicating this literal "model is required" 3 times.
- `go/ai/provider_router_test.go:59` — Define a constant instead of duplicating this literal "remote ok" 3 times.
- `go/ai/provider_router_test.go:71` — Define a constant instead of duplicating this literal "Chat() error = %s" 4 times.
- `go/ai/provider_router_test.go:132` — Define a constant instead of duplicating this literal "router context" 4 times.
- `go/ai/provider_router_test.go:176` — Define a constant instead of duplicating this literal "fake.Chat" 4 times.
- `go/ai/provider_router_test.go:176` — Define a constant instead of duplicating this literal "local offline" 3 times.
- `go/ai/provider_router_test.go:278` — Define a constant instead of duplicating this literal "NewProviderRouterWithOptions() error = %s" 3 times.
- `go/ai/rag_test.go:21` — Define a constant instead of duplicating this literal "Investigate build failure" 3 times.
- `go/ai/rag_test.go:22` — Define a constant instead of duplicating this literal "CI compile step fails" 4 times.
- `go/ai/rag_test.go:27` — Define a constant instead of duplicating this literal "buildTaskQuery() = %q, want %q" 3 times.
- `go/ai/rag_test.go:38` — Define a constant instead of duplicating this literal "buildTaskQuery() rune length = %d, want %d" 3 times.
- `go/ai/rag_test.go:118` — Define a constant instead of duplicating this literal "QueryRAGForTask() error = %s, want nil" 7 times.
- `go/ai/rag_test.go:120` — Define a constant instead of duplicating this literal "QueryRAGForTask() = %q, want empty string" 3 times.
- `go/ai/rag_test.go:185` — Define a constant instead of duplicating this literal "docs/build.md" 3 times.
- `go/cmd/book-state-demo/main.go:179` — Define a constant instead of duplicating this literal "book-state-demo.route" 4 times.
- `go/cmd/embed-bench/main.go:39` — Define a constant instead of duplicating this literal "scoring-calibration" 3 times.
- `go/cmd/embed-bench/main.go:47` — Define a constant instead of duplicating this literal "openbrain-architecture" 3 times.
- `go/cmd/embed-bench/main.go:55` — Define a constant instead of duplicating this literal "deployment-infrastructure" 3 times.
- `go/cmd/embed-bench/main.go:63` — Define a constant instead of duplicating this literal "lem-training" 3 times.
- `go/cmd/embed-bench/main_test.go:14` — Define a constant instead of duplicating this literal "nomic-embed-text:latest" 5 times.
- `go/cmd/embed-bench/main_test.go:15` — Define a constant instead of duplicating this literal "mxbai-embed-large:latest" 4 times.
- `go/cmd/embed-bench/main_test.go:16` — Define a constant instead of duplicating this literal "snowflake-arctic-embed2:335m" 3 times.
- `go/cmd/embed-bench/main_test.go:22` — Define a constant instead of duplicating this literal "nomic-embed-text" 5 times.
- `go/cmd/embed-bench/main_test.go:74` — Define a constant instead of duplicating this literal "topic-a" 3 times.
- `go/cmd/embed-bench/main_test.go:146` — Define a constant instead of duplicating this literal "unexpected path: %q" 3 times.
- `go/cmd/embed-bench/main_test.go:150` — Define a constant instead of duplicating this literal "write response: %v" 4 times.
- `go/cmd/lab/cmd_lab_test.go:52` — Define a constant instead of duplicating this literal "--allow-remote" 3 times.
- `go/cmd/lab/cmd_lab_test.go:122` — Define a constant instead of duplicating this literal "expected-token" 5 times.
- `go/cmd/lab/cmd_lab_test.go:124` — Define a constant instead of duplicating this literal "0.0.0.0:8080" 4 times.
- `go/cmd/lab/cmd_lab_test.go:233` — Define a constant instead of duplicating this literal "/health" 5 times.
- `go/cmd/lab/cmd_lab_test.go:241` — Define a constant instead of duplicating this literal "expected 200 status, got %d" 4 times.
- `go/cmd/lem-desktop/dashboard_test.go:12` — Define a constant instead of duplicating this literal "/tmp/db.duckdb" 4 times.
- `go/cmd/lem-desktop/dashboard_test.go:12` — Define a constant instead of duplicating this literal "http://127.0.0.1:1" 7 times.
- `go/cmd/lem-desktop/docker_test.go:11` — Define a constant instead of duplicating this literal "/tmp/deploy" 9 times.
- `go/cmd/lem-desktop/tray.go:89` — Define a constant instead of duplicating this literal "lem.desktop.tray" 3 times.
- `go/cmd/lem-desktop/tray_test.go:25` — Define a constant instead of duplicating this literal "docker service" 3 times.
- `go/cmd/lem-desktop/tray_test.go:40` — Define a constant instead of duplicating this literal "/tmp/deploy" 4 times.
- `go/cmd/metrics/cmd.go:92` — Define a constant instead of duplicating this literal "  %-30s %v\n" 3 times.
- `go/cmd/metrics/cmd.go:137` — Define a constant instead of duplicating this literal "invalid duration: " 4 times.
- `go/cmd/rag/cmd_test.go:11` — Define a constant instead of duplicating this literal "ai/rag" 4 times.
- `go/cmd/security/cmd_alerts_test.go:12` — Define a constant instead of duplicating this literal "repos/acme/api/dependabot/alerts?state=open" 4 times.
- `go/cmd/security/cmd_alerts_test.go:41` — Define a constant instead of duplicating this literal "repos/acme/api/code-scanning/alerts?state=open" 4 times.
- `go/cmd/security/cmd_alerts_test.go:43` — Define a constant instead of duplicating this literal "repos/acme/api/secret-scanning/alerts?state=open" 4 times.
- `go/cmd/security/cmd_alerts_test.go:55` — Define a constant instead of duplicating this literal "unexpected endpoint: %s" 4 times.
- `go/cmd/security/cmd_alerts_test.go:60` — Define a constant instead of duplicating this literal "acme/api" 4 times.
- `go/cmd/security/cmd_deps_test.go:11` — Define a constant instead of duplicating this literal "repos/acme/api/dependabot/alerts?state=open" 3 times.
- `go/cmd/security/cmd_deps_test.go:12` — Define a constant instead of duplicating this literal "unexpected endpoint: %s" 3 times.
- `go/cmd/security/cmd_jobs.go:226` — Define a constant instead of duplicating this literal "invalid target format: use owner/repo" 3 times.
- `go/cmd/security/cmd_jobs_test.go:60` — Define a constant instead of duplicating this literal "CVE-2026-0001" 3 times.
- `go/cmd/security/cmd_jobs_test.go:134` — Define a constant instead of duplicating this literal "acme/api" 18 times.
- `go/cmd/security/cmd_jobs_test.go:134` — Define a constant instead of duplicating this literal "acme/web" 8 times.
- `go/cmd/security/cmd_jobs_test.go:370` — Define a constant instead of duplicating this literal "acme/security" 6 times.
- `go/cmd/security/cmd_jobs_test.go:382` — Define a constant instead of duplicating this literal "https://github.com/acme/security/issues/1" 3 times.
- `go/cmd/security/cmd_jobs_test.go:477` — Define a constant instead of duplicating this literal "acme/api, acme/web" 3 times.
- `go/cmd/security/cmd_scan_test.go:11` — Define a constant instead of duplicating this literal "repos/acme/api/code-scanning/alerts?state=open" 3 times.
- `go/cmd/security/cmd_scan_test.go:12` — Define a constant instead of duplicating this literal "unexpected endpoint: %s" 3 times.
- `go/cmd/security/cmd_scan_test.go:14` — Define a constant instead of duplicating this literal "\x70ath" 3 times.
- `go/cmd/security/cmd_secrets_test.go:11` — Define a constant instead of duplicating this literal "repos/acme/api/secret-scanning/alerts?state=open" 3 times.
- `go/cmd/security/cmd_secrets_test.go:12` — Define a constant instead of duplicating this literal "unexpected endpoint: %s" 3 times.
- `go/cmd/security/cmd_security.go:26` — Define a constant instead of duplicating this literal "security.github.api" 5 times.
- `go/cmd/security/cmd_security_test.go:128` — Define a constant instead of duplicating this literal "acme/api" 10 times.
- `go/cmd/security/cmd_security_test.go:128` — Define a constant instead of duplicating this literal "security.alerts" 4 times.
- `go/cmd/security/cmd_security_test.go:192` — Define a constant instead of duplicating this literal "No alerts" 4 times.
- `go/cmd/security/cmd_security_test.go:235` — Define a constant instead of duplicating this literal "repos/acme/api/dependabot/alerts?state=open" 9 times.
- `go/cmd/security/cmd_security_test.go:246` — Define a constant instead of duplicating this literal "#!/bin/sh\nprintf '404 Not Found' >&2\nexit 1\n" 3 times.
- `go/cmd/security/cmd_security_test.go:445` — Define a constant instead of duplicating this literal "security scan" 4 times.
- `go/cmd/security/cmd_security_test.go:453` — Define a constant instead of duplicating this literal "acme/web" 4 times.
- `go/cmd/security/cmd_security_test.go:469` — Define a constant instead of duplicating this literal "acme/docs" 3 times.
- `go/cmd/security/security_targets_test.go:12` — Define a constant instead of duplicating this literal "wailsapp/wails" 3 times.
- `go/cmd/security/security_targets_test.go:48` — Define a constant instead of duplicating this literal "go-rag" 4 times.
- `go/cmd/security/security_targets_test.go:72` — Define a constant instead of duplicating this literal "acme/api" 7 times.
- `go/mcp/service_test.go:27` — Define a constant instead of duplicating this literal "Custom tool" 3 times.
- `go/mcp/tools_external.go:1060` — Define a constant instead of duplicating this literal "%w: selector is required" 4 times.
- `go/mcp/tools_external.go:1203` — Define a constant instead of duplicating this literal "%w: sessionId is required" 3 times.
- `go/mcp/tools_external_test.go:85` — Define a constant instead of duplicating this literal "gpt-test" 3 times.
- `go/pkg/api/handlers_test.go:45` — Define a constant instead of duplicating this literal "/v1/score/content" 3 times.
- `go/pkg/lab/cmd_test.go:39` — Define a constant instead of duplicating this literal "0.0.0.0:8080" 3 times.
- `go/pkg/lab/cmd_test.go:43` — Define a constant instead of duplicating this literal "non-loopback" 3 times.
- `go/pkg/lab/cmd_test.go:48` — Define a constant instead of duplicating this literal "127.0.0.1:8080" 4 times.
- `go/providers/openai/openai.go:107` — Define a constant instead of duplicating this literal "ai.openai.LoadModel" 3 times.
- `go/providers/openai/openai.go:138` — Define a constant instead of duplicating this literal "openai-compatible" 6 times.
- `go/providers/openai/openai.go:366` — Define a constant instead of duplicating this literal "ai.openai.doRequest" 5 times.
- `go/providers/openai/openai.go:451` — Define a constant instead of duplicating this literal "ai.openai.provider" 3 times.
- `go/providers/openai/openai_example_test.go:55` — Define a constant instead of duplicating this literal "https://api.example.test" 5 times.
- `go/providers/openai/openai_test.go:49` — Define a constant instead of duplicating this literal "gpt-test" 12 times.
- `go/providers/openai/openai_test.go:80` — Define a constant instead of duplicating this literal "openai-test" 8 times.
- `go/providers/openai/openai_test.go:94` — Define a constant instead of duplicating this literal "LoadModel() error = %s" 5 times.
- `go/providers/openai/openai_test.go:127` — Define a constant instead of duplicating this literal "retrieved context" 4 times.
- `go/providers/openai/openai_test.go:209` — Define a constant instead of duplicating this literal "https://api.example.test" 13 times.
- `go/providers/openai/openai_test.go:295` — Define a constant instead of duplicating this literal "https://api.example.test/" 3 times.

### go:S3776 — Cognitive Complexity of functions should not be too high (12×, code smell)

- `go/cmd/daemon/main.go:38` — Refactor this method to reduce its Cognitive Complexity from 29 to the 15 allowed.
- `go/cmd/embed-bench/main.go:96` — Refactor this method to reduce its Cognitive Complexity from 63 to the 15 allowed.
- `go/cmd/lem-desktop/dashboard.go:176` — Refactor this method to reduce its Cognitive Complexity from 24 to the 15 allowed.
- `go/cmd/metrics/cmd.go:60` — Refactor this method to reduce its Cognitive Complexity from 23 to the 15 allowed.
- `go/cmd/security/cmd_jobs.go:44` — Refactor this method to reduce its Cognitive Complexity from 22 to the 15 allowed.
- `go/cmd/security/cmd_jobs.go:189` — Refactor this method to reduce its Cognitive Complexity from 23 to the 15 allowed.
- `go/cmd/security/cmd_jobs.go:265` — Refactor this method to reduce its Cognitive Complexity from 23 to the 15 allowed.
- `go/cmd/security/cmd_security.go:334` — Refactor this method to reduce its Cognitive Complexity from 18 to the 15 allowed.
- `go/mcp/service.go:290` — Refactor this method to reduce its Cognitive Complexity from 18 to the 15 allowed.
- `go/mcp/tools_external.go:433` — Refactor this method to reduce its Cognitive Complexity from 18 to the 15 allowed.
- `go/mcp/transport_unix.go:14` — Refactor this method to reduce its Cognitive Complexity from 20 to the 15 allowed.
- `go/providers/openai/openai_test.go:19` — Refactor this method to reduce its Cognitive Complexity from 26 to the 15 allowed.

### typescript:S3776 — Cognitive Complexity of functions should not be too high (2×, code smell)

- `go/ui/src/lem-chat.ts:108` — Refactor this function to reduce its Cognitive Complexity from 31 to the 15 allowed.
- `go/ui/src/markdown.ts:25` — Refactor this function to reduce its Cognitive Complexity from 26 to the 15 allowed.

### go:S1186 — Functions should not be empty (1×, code smell)

- `go/mcp/tools_external_test.go:121` — Add a nested comment explaining why this function is empty or complete the implementation.

## MAJOR

### typescript:S2933 — Fields that are only assigned in the constructor should be "readonly" (5×, code smell)

- `go/ui/src/lem-chat.ts:8` — Member 'shadow' is never reassigned; mark it as `readonly`.
- `go/ui/src/lem-chat.ts:12` — Member 'history' is never reassigned; mark it as `readonly`.
- `go/ui/src/lem-input.ts:5` — Member 'shadow' is never reassigned; mark it as `readonly`.
- `go/ui/src/lem-message.ts:10` — Member 'shadow' is never reassigned; mark it as `readonly`.
- `go/ui/src/lem-messages.ts:5` — Member 'shadow' is never reassigned; mark it as `readonly`.

### go:S4144 — Functions should not have identical implementations (3×, code smell)

- `go/cmd/lem-desktop/docker_example_test.go:30` — Update this function so that its implementation is not identical to "ExampleNewDockerService" on line 22.
- `go/cmd/lem-desktop/docker_test.go:285` — Update this function so that its implementation is not identical to "TestDocker_NewDockerService_Ugly" on line 28.
- `go/cmd/lem-desktop/tray_example_test.go:34` — Update this function so that its implementation is not identical to "ExampleNewTrayService" on line 10.

### javascript:S3358 — Ternary operators should not be nested (2×, code smell)

- `go/cmd/lem-desktop/frontend/index.html:318` — Extract this nested ternary operation into an independent statement.
- `go/cmd/lem-desktop/frontend/index.html:367` — Extract this nested ternary operation into an independent statement.

### go:S108 — Nested blocks of code should not be left empty (2×, code smell)

- `go/providers/openai/openai_test.go:196` — Either remove or fill this block of code.
- `go/providers/openai/openai_test.go:545` — Either remove or fill this block of code.

### yaml:DocumentStartCheck — For correct parsing especially in the case of multiple or embedded documents, documents should start with a document start marker (2×, code smell)

- `go/tests/cli/ai/Taskfile.yaml:1` — missing document start "---" (document-start)
- `go/ui/pnpm-lock.yaml:1` — missing document start "---" (document-start)

### javascript:S7785 — Top-level await should be preferred over wrapped async operations (1×, code smell)

- `go/cmd/lem-desktop/frontend/index.html:478` — Prefer top-level await over an async function `refreshAll` call.

### go:S3923 — All branches in a conditional structure should not have exactly the same implementation (1×, bug)

- `go/cmd/lab/cmd_lab.go:161` — Remove this conditional structure or edit its code blocks so that they're not all the same.

### javascript:S7762 — DOM nodes should be removed using "remove()" instead of "removeChild()" (1×, code smell)

- `go/cmd/lem-desktop/frontend/index.html:281` — Prefer `childNode.remove()` over `parentNode.removeChild(childNode)`.

## MINOR

### typescript:S7781 — Strings should use "replaceAll()" instead of "replace()" with global regex (9×, code smell)

- `go/ui/src/markdown.ts:3` — Prefer `String#replaceAll()` over `String#replace()`.
- `go/ui/src/markdown.ts:4` — Prefer `String#replaceAll()` over `String#replace()`.
- `go/ui/src/markdown.ts:5` — Prefer `String#replaceAll()` over `String#replace()`.
- `go/ui/src/markdown.ts:6` — Prefer `String#replaceAll()` over `String#replace()`.
- `go/ui/src/markdown.ts:11` — Prefer `String#replaceAll()` over `String#replace()`.
- `go/ui/src/markdown.ts:12` — Prefer `String#replaceAll()` over `String#replace()`.
- `go/ui/src/markdown.ts:13` — Prefer `String#replaceAll()` over `String#replace()`.
- `go/ui/src/markdown.ts:14` — Prefer `String#replaceAll()` over `String#replace()`.
- `go/ui/src/markdown.ts:15` — Prefer `String#replaceAll()` over `String#replace()`.

### javascript:S7764 — Use "globalThis" instead of "window", "self", or "global" (7×, code smell)

- `go/cmd/lem-desktop/frontend/index.html:424` — Prefer `globalThis` over `window`.
- `go/cmd/lem-desktop/frontend/index.html:440` — Prefer `globalThis` over `window`.
- `go/cmd/lem-desktop/frontend/index.html:453` — Prefer `globalThis` over `window`.
- `go/cmd/lem-desktop/frontend/index.html:455` — Prefer `globalThis` over `window`.
- `go/cmd/lem-desktop/frontend/index.html:465` — Prefer `globalThis` over `window`.
- `go/cmd/lem-desktop/frontend/index.html:467` — Prefer `globalThis` over `window`.
- `go/cmd/lem-desktop/frontend/index.html:469` — Prefer `globalThis` over `window`.

### typescript:S7773 — Number static methods and properties should be preferred over global equivalents (2×, code smell)

- `go/ui/src/lem-chat.ts:89` — Prefer `Number.parseInt` over `parseInt`.
- `go/ui/src/lem-chat.ts:94` — Prefer `Number.parseFloat` over `parseFloat`.

### typescript:S7764 — Use "globalThis" instead of "window", "self", or "global" (1×, code smell)

- `go/ui/src/lem-chat.ts:79` — Prefer `globalThis` over `window`.

### typescript:S7735 — Negated conditions should be avoided when an else clause is present (1×, code smell)

- `go/ui/src/markdown.ts:34` — Unexpected negated condition.

## INFO

### yaml:LineLengthCheck — For readability and maintenance lines should not exceed a certain length (28×, code smell)

- `go/ui/pnpm-lock.yaml:21` — line too long (124 > 80 characters) (line-length)
- `go/ui/pnpm-lock.yaml:27` — line too long (124 > 80 characters) (line-length)
- `go/ui/pnpm-lock.yaml:33` — line too long (124 > 80 characters) (line-length)
- `go/ui/pnpm-lock.yaml:39` — line too long (124 > 80 characters) (line-length)
- `go/ui/pnpm-lock.yaml:45` — line too long (124 > 80 characters) (line-length)
- `go/ui/pnpm-lock.yaml:51` — line too long (124 > 80 characters) (line-length)
- `go/ui/pnpm-lock.yaml:57` — line too long (124 > 80 characters) (line-length)
- `go/ui/pnpm-lock.yaml:63` — line too long (124 > 80 characters) (line-length)
- `go/ui/pnpm-lock.yaml:69` — line too long (124 > 80 characters) (line-length)
- `go/ui/pnpm-lock.yaml:75` — line too long (124 > 80 characters) (line-length)
- `go/ui/pnpm-lock.yaml:81` — line too long (124 > 80 characters) (line-length)
- `go/ui/pnpm-lock.yaml:87` — line too long (124 > 80 characters) (line-length)
- `go/ui/pnpm-lock.yaml:93` — line too long (124 > 80 characters) (line-length)
- `go/ui/pnpm-lock.yaml:99` — line too long (124 > 80 characters) (line-length)
- `go/ui/pnpm-lock.yaml:105` — line too long (124 > 80 characters) (line-length)
- `go/ui/pnpm-lock.yaml:111` — line too long (124 > 80 characters) (line-length)
- `go/ui/pnpm-lock.yaml:117` — line too long (124 > 80 characters) (line-length)
- `go/ui/pnpm-lock.yaml:123` — line too long (124 > 80 characters) (line-length)
- `go/ui/pnpm-lock.yaml:129` — line too long (124 > 80 characters) (line-length)
- `go/ui/pnpm-lock.yaml:135` — line too long (124 > 80 characters) (line-length)
- `go/ui/pnpm-lock.yaml:141` — line too long (124 > 80 characters) (line-length)
- `go/ui/pnpm-lock.yaml:147` — line too long (124 > 80 characters) (line-length)
- `go/ui/pnpm-lock.yaml:153` — line too long (124 > 80 characters) (line-length)
- `go/ui/pnpm-lock.yaml:159` — line too long (124 > 80 characters) (line-length)
- `go/ui/pnpm-lock.yaml:165` — line too long (124 > 80 characters) (line-length)
- `go/ui/pnpm-lock.yaml:171` — line too long (124 > 80 characters) (line-length)
- `go/ui/pnpm-lock.yaml:177` — line too long (124 > 80 characters) (line-length)
- `go/ui/pnpm-lock.yaml:182` — line too long (124 > 80 characters) (line-length)

### go:S1135 — Track uses of "TODO" tags (5×, code smell)

- `go/pkg/api/handlers.go:17` — Complete the task associated to this TODO comment.
- `go/pkg/api/handlers.go:25` — Complete the task associated to this TODO comment.
- `go/pkg/api/handlers.go:33` — Complete the task associated to this TODO comment.
- `go/pkg/api/handlers.go:41` — Complete the task associated to this TODO comment.
- `go/pkg/api/handlers.go:49` — Complete the task associated to this TODO comment.

