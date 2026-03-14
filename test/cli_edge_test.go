package codeye_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExecWithBadAgentNotInstalled(t *testing.T) {
	shortHome(t)
	code, _, stderr := runCLI(t, []string{"codeye", "--agent", "/nonexistent-binary-xyz", "--cwd", "/tmp", "exec", "hello"})
	if code == 0 {
		t.Fatalf("expected non-zero exit for bad agent, got 0 (stderr=%s)", stderr)
	}
}

func TestSessionsNewWithBadAgent(t *testing.T) {
	shortHome(t)
	code, _, _ := runCLI(t, []string{"codeye", "--agent", "/nonexistent-binary-xyz", "--cwd", "/tmp", "sessions", "new"})
	if code == 0 {
		t.Fatal("expected non-zero exit for bad agent")
	}
}

func TestSessionsNoSubcommandActsAsList(t *testing.T) {
	t.Skip("sessions list calls ListRemoteSessions (agent); can block on agent response")
	home := shortHome(t)
	code, stdout, _ := runCLI(t, []string{"codeye", "--agent", "echo", "--format", "json", "--cwd", home, "sessions"})
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	if !strings.Contains(stdout, "Sessions") && !strings.Contains(stdout, "No sessions") {
		t.Fatalf("expected sessions list output, got %q", stdout)
	}
}

func TestSessionsNewCreatesNew(t *testing.T) {
	shortHome(t)
	mockPath := mockAgentPath(t)
	if _, err := os.Stat(mockPath); err != nil {
		t.Skip("mock agent not found")
	}
	cwd, _ := os.MkdirTemp("/tmp", "cye-cwd-")
	t.Cleanup(func() { os.RemoveAll(cwd) })

	code, stdout, stderr := runCLI(t, []string{"codeye", "--cwd", cwd, "--agent", "go run " + mockPath, "--format", "json", "sessions", "new"})
	if code != 0 {
		t.Fatalf("sessions new: exit %d (stderr=%s)", code, stderr)
	}
	if !strings.Contains(stdout, "session_ready") && !(strings.Contains(stdout, "Session") && (strings.Contains(stdout, "ready") || strings.Contains(stdout, "created"))) {
		t.Fatalf("expected session ready/created message, got %q", stdout)
	}
}

func TestConfigLoadErrorInCLI(t *testing.T) {
	shortHome(t)
	cwd, _ := os.MkdirTemp("/tmp", "cye-cwd-")
	t.Cleanup(func() { os.RemoveAll(cwd) })
	os.WriteFile(filepath.Join(cwd, ".codeye.json"), []byte("{invalid json"), 0o644)

	code, _, stderr := runCLI(t, []string{"codeye", "--cwd", cwd, "config"})
	if code != 1 {
		t.Fatalf("expected exit 1 for bad config, got %d (stderr=%s)", code, stderr)
	}
}

func TestConfigLoadErrorJSONStrict(t *testing.T) {
	shortHome(t)
	cwd, _ := os.MkdirTemp("/tmp", "cye-cwd-")
	t.Cleanup(func() { os.RemoveAll(cwd) })
	os.WriteFile(filepath.Join(cwd, ".codeye.json"), []byte("{invalid json"), 0o644)

	code, stdout, _ := runCLI(t, []string{"codeye", "--json-strict", "--cwd", cwd, "config"})
	if code != 1 {
		t.Fatalf("expected exit 1, got %d", code)
	}
	if !strings.Contains(stdout, "jsonrpc") {
		t.Fatalf("expected JSON-RPC error, got %q", stdout)
	}
}

func TestSessionsCloseWithSession(t *testing.T) {
	home := shortHome(t)
	cwd, _ := os.MkdirTemp("/tmp", "cye-cwd-")
	t.Cleanup(func() { os.RemoveAll(cwd) })
	root := filepath.Join(home, ".codeye")
	os.MkdirAll(root, 0o755)
	rec := createSessionRecord(t, root, "codex", cwd)

	code, stdout, _ := runCLI(t, []string{"codeye", "--agent", "echo", "--format", "json", "--cwd", cwd, "sessions", "close", rec.RecordID})
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	if !strings.Contains(stdout, "session_closed") && !strings.Contains(stdout, "Session closed") {
		t.Fatalf("expected Session closed message, got %q", stdout)
	}
}

func TestSessionsShowWithSessionJSON(t *testing.T) {
	home := shortHome(t)
	cwd, _ := os.MkdirTemp("/tmp", "cye-cwd-")
	t.Cleanup(func() { os.RemoveAll(cwd) })
	root := filepath.Join(home, ".codeye")
	os.MkdirAll(root, 0o755)
	rec := createSessionRecord(t, root, "codex", cwd)

	code, stdout, _ := runCLI(t, []string{"codeye", "--agent", "echo", "--format", "json", "--cwd", cwd, "sessions", "show", rec.RecordID})
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	if !strings.Contains(stdout, "test-rec") {
		t.Fatalf("expected record id, got %q", stdout)
	}
}

func TestSessionsShowWithSessionText(t *testing.T) {
	home := shortHome(t)
	cwd, _ := os.MkdirTemp("/tmp", "cye-cwd-")
	t.Cleanup(func() { os.RemoveAll(cwd) })
	root := filepath.Join(home, ".codeye")
	os.MkdirAll(root, 0o755)
	rec := createSessionRecord(t, root, "codex", cwd)

	code, stdout, _ := runCLI(t, []string{"codeye", "--agent", "echo", "--cwd", cwd, "sessions", "show", rec.RecordID})
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	if !strings.Contains(stdout, "id: test-rec") {
		t.Fatalf("expected text format, got %q", stdout)
	}
}
