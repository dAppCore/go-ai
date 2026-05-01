package security

import (
	"testing"

	core "dappco.re/go"
	"dappco.re/go/cli/pkg/cli"
)

func withSecurityTempHome(t *testing.T) string {
	t.Helper()

	tempHome := t.TempDir()
	t.Setenv("CORE_HOME", tempHome)
	t.Setenv("HOME", tempHome)
	t.Setenv("DIR_HOME", "")
	return tempHome
}

func withFakeGitHubCLI(t *testing.T) {
	t.Helper()

	withFakeGitHubScript(t, "#!/bin/sh\nexit 0\n")
}

func withFakeGitHubScript(t *testing.T, script string) {
	t.Helper()

	binDir := t.TempDir()
	ghPath := core.PathJoin(binDir, "gh")
	if r := core.WriteFile(ghPath, []byte(script), 0o755); !r.OK {
		t.Fatalf("write fake gh: %v", r.Error())
	}

	path := core.Getenv("PATH")
	if path == "" {
		t.Setenv("PATH", binDir)
		return
	}
	t.Setenv("PATH", binDir+string(core.PathListSeparator)+path)
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()

	buf := core.NewBuilder()
	cli.SetStdout(buf)
	defer cli.SetStdout(nil)

	fn()

	return buf.String()
}

func stubGitHubAPI(t *testing.T, fn func(endpoint string) ([]byte, error)) {
	t.Helper()

	original := callGitHubAPIRequest
	callGitHubAPIRequest = func(endpoint string) core.Result {
		output, err := fn(endpoint)
		if err != nil {
			return core.Fail(err)
		}
		return core.Ok(output)
	}
	t.Cleanup(func() {
		callGitHubAPIRequest = original
	})
}

func normalizeWhitespace(s string) string {
	return core.Join(" ", securityFields(s)...)
}

func writeSecurityRegistry(t *testing.T, org string, repoNames ...string) string {
	t.Helper()

	registryDir := t.TempDir()
	registryPath := core.PathJoin(registryDir, "repos.yaml")

	builder := core.NewBuilder()
	builder.WriteString("version: 1\n")
	builder.WriteString("org: " + org + "\n")
	builder.WriteString("base_path: " + registryDir + "\n")
	builder.WriteString("repos:\n")
	for _, repoName := range repoNames {
		builder.WriteString("  " + repoName + ":\n")
		builder.WriteString("    type: module\n")
	}

	if r := core.WriteFile(registryPath, []byte(builder.String()), 0o644); !r.OK {
		t.Fatalf("write registry: %v", r.Error())
	}

	return registryPath
}

func securityFields(value string) []string {
	var out []string
	start := -1
	for i, r := range value {
		if core.IsSpace(r) {
			if start >= 0 {
				out = append(out, value[start:i])
				start = -1
			}
			continue
		}
		if start < 0 {
			start = i
		}
	}
	if start >= 0 {
		out = append(out, value[start:])
	}
	return out
}

func securityIndex(value, needle string) int {
	if needle == "" {
		return 0
	}
	parts := core.SplitN(value, needle, 2)
	if len(parts) != 2 {
		return -1
	}
	return len(parts[0])
}
