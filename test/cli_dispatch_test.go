package codeye_test

import (
	"crypto/sha1"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/one710/codeye/internal/session/persistence"
)

func cliSocketPath(root, agent, cwd string) string {
	h := sha1.Sum([]byte(agent + "|" + cwd))
	return filepath.Join(root, "queue-"+hex.EncodeToString(h[:8])+".sock")
}

func createSessionRecord(t *testing.T, root, agent, cwd string) persistence.Record {
	t.Helper()
	repo := persistence.New(root)
	rec := persistence.Record{RecordID: "test-rec", ACPSession: "test-session", Agent: agent, Cwd: cwd}
	if err := repo.Save(rec); err != nil {
		t.Fatal(err)
	}
	return rec
}

func TestCancelWithSessionNoQueue(t *testing.T) {
	home := shortHome(t)
	cwd, _ := os.MkdirTemp("/tmp", "cye-cwd-")
	t.Cleanup(func() { os.RemoveAll(cwd) })
	root := filepath.Join(home, ".codeye")
	os.MkdirAll(root, 0o755)
	rec := createSessionRecord(t, root, "codex", cwd)

	code, _, _ := runCLI(t, []string{"codeye", "--agent", "echo", "--cwd", cwd, "cancel", rec.RecordID})
	if code != 1 {
		t.Fatalf("expected exit 1 for cancel failure, got %d", code)
	}
}

func TestStatusWithSessionExists(t *testing.T) {
	home := shortHome(t)
	cwd, _ := os.MkdirTemp("/tmp", "cye-cwd-")
	t.Cleanup(func() { os.RemoveAll(cwd) })
	root := filepath.Join(home, ".codeye")
	os.MkdirAll(root, 0o755)
	rec := createSessionRecord(t, root, "codex", cwd)

	code, stdout, _ := runCLI(t, []string{"codeye", "--agent", "echo", "--format", "json", "--cwd", cwd, "status", rec.RecordID})
	if code != 0 {
		t.Fatalf("expected exit 0, got %d (stdout=%s)", code, stdout)
	}
	// ACP: status is "session" when a session record exists (no alive/dead/pid).
	if !strings.Contains(stdout, "\"status\":\"session\"") {
		t.Fatalf("expected status session, got %q", stdout)
	}
}

func TestSetModeWithSessionNoQueue(t *testing.T) {
	t.Skip("set-mode now in-process; can take full agent timeout")
	home := shortHome(t)
	cwd, _ := os.MkdirTemp("/tmp", "cye-cwd-")
	t.Cleanup(func() { os.RemoveAll(cwd) })
	root := filepath.Join(home, ".codeye")
	os.MkdirAll(root, 0o755)
	rec := createSessionRecord(t, root, "codex", cwd)
	code, _, _ := runCLI(t, []string{"codeye", "--agent", "echo", "--cwd", cwd, "set-mode", rec.RecordID, "plan"})
	if code != 1 {
		t.Fatalf("expected exit 1, got %d", code)
	}
}

func TestSetWithSessionNoQueue(t *testing.T) {
	t.Skip("set now in-process; can take full agent timeout")
	home := shortHome(t)
	cwd, _ := os.MkdirTemp("/tmp", "cye-cwd-")
	t.Cleanup(func() { os.RemoveAll(cwd) })
	root := filepath.Join(home, ".codeye")
	os.MkdirAll(root, 0o755)
	rec := createSessionRecord(t, root, "codex", cwd)
	code, _, _ := runCLI(t, []string{"codeye", "--agent", "echo", "--cwd", cwd, "set", rec.RecordID, "k", "v"})
	if code != 1 {
		t.Fatalf("expected exit 1, got %d", code)
	}
}

func TestConfigCommandTextFormat(t *testing.T) {
	shortHome(t)
	code, stdout, _ := runCLI(t, []string{"codeye", "--agent", "echo", "--cwd", "/tmp", "config"})
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	if !strings.Contains(stdout, "defaultAgent: cursor") {
		t.Fatalf("expected text config output, got %q", stdout)
	}
}

func TestSessionsListEmptyJSON(t *testing.T) {
	home := shortHome(t)
	code, stdout, _ := runCLI(t, []string{"codeye", "--agent", "echo", "--format", "json", "--cwd", home, "sessions", "list"})
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	if !strings.Contains(stdout, "sessions_list") && !strings.Contains(stdout, "Sessions") && !strings.Contains(stdout, "No sessions") {
		t.Fatalf("expected sessions list output, got %q", stdout)
	}
}

func TestSessionsListWithLocalSession(t *testing.T) {
	home := shortHome(t)
	cwd, _ := os.MkdirTemp("/tmp", "cye-cwd-")
	t.Cleanup(func() { os.RemoveAll(cwd) })
	root := filepath.Join(home, ".codeye")
	os.MkdirAll(root, 0o755)
	createSessionRecord(t, root, "codex", cwd)

	code, stdout, _ := runCLI(t, []string{"codeye", "--agent", "echo", "--format", "json", "--cwd", cwd, "sessions"})
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	if !strings.Contains(stdout, "sessions_list") && !strings.Contains(stdout, "Sessions") && !strings.Contains(stdout, "No sessions") {
		t.Fatalf("expected sessions list output, got %q", stdout)
	}
}

func TestDispatchPromptWithNoSessionNoQueue(t *testing.T) {
	t.Skip("prompt now in-process; depends on agent response time")
	shortHome(t)
	cwd, _ := os.MkdirTemp("/tmp", "cye-cwd-")
	t.Cleanup(func() { os.RemoveAll(cwd) })
	code, _, _ := runCLI(t, []string{"codeye", "--agent", "echo", "--cwd", cwd, "prompt", "hello"})
	if code != 1 {
		t.Fatalf("expected exit 1 for no session/queue, got %d", code)
	}
}

func TestDispatchExecWithNoQueue(t *testing.T) {
	t.Skip("exec now in-process; depends on agent response time")
	shortHome(t)
	cwd, _ := os.MkdirTemp("/tmp", "cye-cwd-")
	t.Cleanup(func() { os.RemoveAll(cwd) })
	code, _, _ := runCLI(t, []string{"codeye", "--agent", "echo", "--cwd", cwd, "exec", "hello"})
	if code != 1 {
		t.Fatalf("expected exit 1, got %d", code)
	}
}

func TestDispatchConfigWithCustomConfig(t *testing.T) {
	shortHome(t)
	cwd, _ := os.MkdirTemp("/tmp", "cye-cwd-")
	t.Cleanup(func() { os.RemoveAll(cwd) })
	os.WriteFile(filepath.Join(cwd, ".codeye.json"), []byte(`{"defaultAgent":"claude"}`), 0o644)

	code, stdout, _ := runCLI(t, []string{"codeye", "--agent", "echo", "--format", "json", "--cwd", cwd, "config"})
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	if !strings.Contains(stdout, "claude") {
		t.Fatalf("expected claude from project config, got %q", stdout)
	}
}

func TestSessionsNewWithBadACPAgent(t *testing.T) {
	t.Skip("sessions new calls agent in-process; echo agent can take full NewSessionTimeout before failing")
	shortHome(t)
	cwd, _ := os.MkdirTemp("/tmp", "cye-cwd-")
	t.Cleanup(func() { os.RemoveAll(cwd) })
	code, _, _ := runCLI(t, []string{"codeye", "--agent", "echo", "--cwd", cwd, "sessions", "new"})
	if code != 1 {
		t.Fatalf("expected exit 1 for bad ACP agent, got %d", code)
	}
}

func TestResolveErrorInCLI(t *testing.T) {
	shortHome(t)
	cwd, _ := os.MkdirTemp("/tmp", "cye-cwd-")
	t.Cleanup(func() { os.RemoveAll(cwd) })
	os.WriteFile(filepath.Join(cwd, ".codeye.json"), []byte(`{"defaultAgent":"nonexistent"}`), 0o644)
	code, _, _ := runCLI(t, []string{"codeye", "--cwd", cwd, "prompt", "abc", "hello"})
	if code != 2 {
		t.Fatalf("expected exit 2 for unknown agent, got %d", code)
	}
}
