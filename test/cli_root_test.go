package codeye_test

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/one710/codeye/internal/cli"
)

func mockAgentPath(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("unable to determine test file path")
	}
	return filepath.Join(filepath.Dir(thisFile), "testdata", "mockagent", "main.go")
}

func extractSessionIDFromJSON(t *testing.T, stdout string) string {
	t.Helper()
	var m map[string]interface{}
	if err := json.Unmarshal([]byte(strings.TrimSpace(stdout)), &m); err != nil {
		t.Fatalf("parse session output: %v", err)
	}
	sid, _ := m["sessionId"].(string)
	if sid == "" {
		t.Fatal("no sessionId in output")
	}
	return sid
}

func TestExecWithCustomAgentCommand(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	cwd := t.TempDir()
	mockPath := mockAgentPath(t)
	if _, err := os.Stat(mockPath); err != nil {
		t.Skip("mock agent not found")
	}
	code, _, stderr := runCLI(t, []string{"codeye", "--cwd", cwd, "--agent", "go run " + mockPath, "exec", "hello from exec"})
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d (stderr=%s)", code, stderr)
	}
}

func TestJSONStrictNoCommandPrintsJSONError(t *testing.T) {
	code, stdout, _ := runCLI(t, []string{"codeye", "--json-strict"})
	if code != 2 {
		t.Fatalf("expected exit code 2, got %d", code)
	}
	if !strings.Contains(stdout, "\"jsonrpc\":\"2.0\"") {
		t.Fatalf("expected json-rpc error output, got: %s", stdout)
	}
}

func TestJSONStrictParseErrorUsesJSONRPCInvalidParams(t *testing.T) {
	code, stdout, _ := runCLI(t, []string{"codeye", "--json-strict", "--unknown-flag"})
	if code != 2 {
		t.Fatalf("expected exit code 2, got %d", code)
	}
	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(strings.TrimSpace(stdout)), &payload); err != nil {
		t.Fatalf("expected JSON payload, got: %s", stdout)
	}
	errObj, _ := payload["error"].(map[string]interface{})
	if codeVal, ok := errObj["code"].(float64); !ok || int(codeVal) != -32602 {
		t.Fatalf("expected json-rpc invalid params (-32602), got: %v", errObj["code"])
	}
}

func TestSessionsListIncludesRemoteWhenAdvertised(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	cwd := t.TempDir()
	mockPath := mockAgentPath(t)
	if _, err := os.Stat(mockPath); err != nil {
		t.Skip("mock agent not found")
	}
	code, stdout, stderr := runCLI(t, []string{"codeye", "--cwd", cwd, "--agent", "go run " + mockPath, "--format", "json", "sessions", "list"})
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d (stderr=%s)", code, stderr)
	}
	if !strings.Contains(stdout, "\"remoteSessions\"") {
		t.Fatalf("expected remoteSessions in output, got: %s", stdout)
	}
}

func TestVersionFlag(t *testing.T) {
	code, stdout, _ := runCLI(t, []string{"codeye", "--version"})
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}
	if !strings.Contains(stdout, "0.") {
		t.Fatalf("expected version string, got %q", stdout)
	}
}

func TestVersionFlagShort(t *testing.T) {
	code, stdout, _ := runCLI(t, []string{"codeye", "-V"})
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}
	if strings.TrimSpace(stdout) == "" {
		t.Fatal("expected version output")
	}
}

func TestNoCommandShowsHelp(t *testing.T) {
	code, stdout, _ := runCLI(t, []string{"codeye"})
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}
	if !strings.Contains(stdout, "codeye") {
		t.Fatalf("expected help text, got %q", stdout)
	}
}

func TestUnknownCommand(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	code, _, stderr := runCLI(t, []string{"codeye", "--agent", "echo", "--cwd", home, "codex", "bogus-command"})
	if code != 2 {
		t.Fatalf("expected exit code 2, got %d", code)
	}
	if !strings.Contains(stderr, "unknown command") {
		t.Fatalf("expected unknown command error, got %q", stderr)
	}
}

func TestUnknownCommandJSONStrict(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	code, stdout, _ := runCLI(t, []string{"codeye", "--json-strict", "--agent", "echo", "--cwd", home, "codex", "bogus-command"})
	if code != 2 {
		t.Fatalf("expected exit code 2, got %d", code)
	}
	if !strings.Contains(stdout, "unknown command") {
		t.Fatalf("expected json error with unknown command, got %q", stdout)
	}
}

func TestPromptMissingText(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	code, _, stderr := runCLI(t, []string{"codeye", "--agent", "echo", "--cwd", home, "prompt"})
	if code != 2 {
		t.Fatalf("expected exit code 2, got %d (stderr=%s)", code, stderr)
	}
}

func TestPromptMissingTextJSONStrict(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	code, stdout, _ := runCLI(t, []string{"codeye", "--json-strict", "--agent", "echo", "--cwd", home, "prompt"})
	if code != 2 {
		t.Fatalf("expected exit code 2, got %d", code)
	}
	if !strings.Contains(stdout, "prompt") && !strings.Contains(stdout, "session-id") {
		t.Fatalf("expected usage error, got %q", stdout)
	}
}

func TestExecMissingText(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	code, _, _ := runCLI(t, []string{"codeye", "--agent", "echo", "--cwd", home, "exec"})
	if code != 2 {
		t.Fatalf("expected exit code 2, got %d", code)
	}
}

func TestSetModeMissingArg(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	code, _, stderr := runCLI(t, []string{"codeye", "--agent", "echo", "--cwd", home, "set-mode"})
	if code != 2 {
		t.Fatalf("expected exit code 2, got %d", code)
	}
	if !strings.Contains(stderr, "set-mode") {
		t.Fatalf("expected usage error, got %q", stderr)
	}
}

func TestSetMissingArgs(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	code, _, stderr := runCLI(t, []string{"codeye", "--agent", "echo", "--cwd", home, "set", "only-key"})
	if code != 2 {
		t.Fatalf("expected exit code 2, got %d", code)
	}
	if !strings.Contains(stderr, "set") && !strings.Contains(stderr, "session-id") {
		t.Fatalf("expected usage error, got %q", stderr)
	}
}

func TestCancelNoSession(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	code, _, _ := runCLI(t, []string{"codeye", "--agent", "echo", "--format", "json", "--cwd", home, "cancel", "nonexistent-session-id"})
	if code != 1 {
		t.Fatalf("expected exit 1 for cancel with non-existent session, got %d", code)
	}
}

func TestStatusNoSession(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	code, stdout, _ := runCLI(t, []string{"codeye", "--agent", "echo", "--format", "json", "--cwd", home, "status", "nonexistent-session-id"})
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}
	if !strings.Contains(stdout, "\"status\":\"not-found\"") {
		t.Fatalf("expected not-found status, got %q", stdout)
	}
}

func TestConfigCommand(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	code, stdout, _ := runCLI(t, []string{"codeye", "--agent", "echo", "--cwd", home, "config"})
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}
	if !strings.Contains(stdout, "cursor") {
		t.Fatalf("expected config to mention cursor, got %q", stdout)
	}
}

func TestConfigCommandJSON(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	code, stdout, _ := runCLI(t, []string{"codeye", "--format", "json", "--agent", "echo", "--cwd", home, "config"})
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(strings.TrimSpace(stdout)), &parsed); err != nil {
		t.Fatalf("expected valid JSON, got: %s", stdout)
	}
	if parsed["defaultAgent"] != "cursor" {
		t.Fatalf("expected defaultAgent=cursor, got %v", parsed["defaultAgent"])
	}
}

func TestSessionsShowNoSession(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	code, _, stderr := runCLI(t, []string{"codeye", "--agent", "echo", "--cwd", home, "sessions", "show", "nonexistent-session-id"})
	if code != 1 {
		t.Fatalf("expected exit code 1, got %d", code)
	}
	if !strings.Contains(stderr, "not found") && !strings.Contains(stderr, "no session") {
		t.Fatalf("expected session not found, got %q", stderr)
	}
}

func TestSessionsCloseNoSession(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	code, _, stderr := runCLI(t, []string{"codeye", "--agent", "echo", "--cwd", home, "sessions", "close", "nonexistent-session-id"})
	if code != 1 {
		t.Fatalf("expected exit code 1, got %d", code)
	}
	if !strings.Contains(stderr, "not found") && !strings.Contains(stderr, "no session") {
		t.Fatalf("expected session not found, got %q", stderr)
	}
}

func TestSessionsUnknownSubcommand(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	code, _, stderr := runCLI(t, []string{"codeye", "--agent", "echo", "--cwd", home, "sessions", "bogus"})
	if code != 2 {
		t.Fatalf("expected exit code 2, got %d", code)
	}
	if !strings.Contains(stderr, "unknown sessions command") {
		t.Fatalf("expected unknown sessions command, got %q", stderr)
	}
}

func TestSetModeNoSession(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	code, _, stderr := runCLI(t, []string{"codeye", "--agent", "echo", "--cwd", home, "set-mode", "nonexistent-session-id", "plan"})
	if code != 1 {
		t.Fatalf("expected exit code 1, got %d", code)
	}
	if !strings.Contains(stderr, "not found") && !strings.Contains(stderr, "no session") {
		t.Fatalf("expected session not found, got %q", stderr)
	}
}

func TestSetNoSession(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	code, _, stderr := runCLI(t, []string{"codeye", "--agent", "echo", "--cwd", home, "set", "nonexistent-session-id", "key", "val"})
	if code != 1 {
		t.Fatalf("expected exit code 1, got %d", code)
	}
	if !strings.Contains(stderr, "not found") && !strings.Contains(stderr, "no session") {
		t.Fatalf("expected session not found, got %q", stderr)
	}
}

func TestDetectRequestedFormatJSONFlag(t *testing.T) {
	code, stdout, _ := runCLI(t, []string{"codeye", "--format", "json", "--bogus-flag"})
	if code != 2 {
		t.Fatalf("expected 2, got %d", code)
	}
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(strings.TrimSpace(stdout)), &parsed); err != nil {
		t.Fatalf("expected JSON output for --format json error, got: %s", stdout)
	}
}

func TestDetectRequestedFormatQuietFlag(t *testing.T) {
	code, stdout, _ := runCLI(t, []string{"codeye", "--format", "quiet", "--bogus-flag"})
	if code != 2 {
		t.Fatalf("expected 2, got %d", code)
	}
	if strings.TrimSpace(stdout) != "" {
		t.Fatalf("quiet format should produce no stdout, got %q", stdout)
	}
}

func TestParseGlobalsAllFlags(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	code, _, _ := runCLI(t, []string{"codeye", "--cwd", home, "--format", "quiet", "--approve-all", "--agent", "echo", "status", "nonexistent"})
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}
}

func TestParseGlobalsDenyAll(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	code, _, _ := runCLI(t, []string{"codeye", "--deny-all", "--agent", "echo", "--cwd", home, "status", "nonexistent"})
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}
}

func TestParseGlobalsApproveReads(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	code, _, _ := runCLI(t, []string{"codeye", "--approve-reads", "--agent", "echo", "--cwd", home, "status", "nonexistent"})
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}
}

func TestAgentRoutingDefaultsToPrompt(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	code, _, _ := runCLI(t, []string{"codeye", "--agent", "echo", "--cwd", home, "codex"})
	if code != 2 {
		t.Fatalf("expected exit 2 (prompt requires text), got %d", code)
	}
}

func TestSessionsNewAndShowWithMockAgent(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	mockPath := mockAgentPath(t)
	if _, err := os.Stat(mockPath); err != nil {
		t.Skip("mock agent not found")
	}
	cwd := t.TempDir()

	code, stdout, stderr := runCLI(t, []string{"codeye", "--cwd", cwd, "--agent", "go run " + mockPath, "--format", "json", "sessions", "new"})
	if code != 0 {
		t.Fatalf("sessions new: exit %d (stderr=%s)", code, stderr)
	}
	if !strings.Contains(stdout, "session_ready") && !(strings.Contains(stdout, "Session") && (strings.Contains(stdout, "ready") || strings.Contains(stdout, "created"))) {
		t.Fatalf("expected session ready/created message, got %q", stdout)
	}
	sessionID := extractSessionIDFromJSON(t, stdout)

	code, stdout, stderr = runCLI(t, []string{"codeye", "--cwd", cwd, "--agent", "go run " + mockPath, "--format", "json", "sessions", "show", sessionID})
	if code != 0 {
		t.Fatalf("sessions show: exit %d (stderr=%s)", code, stderr)
	}

	code, stdout, stderr = runCLI(t, []string{"codeye", "--cwd", cwd, "--agent", "go run " + mockPath, "--format", "json", "sessions", "close", sessionID})
	if code != 0 {
		t.Fatalf("sessions close: exit %d (stderr=%s)", code, stderr)
	}
	if !strings.Contains(stdout, "session_closed") && !strings.Contains(stdout, "Session closed") {
		t.Fatalf("expected Session closed message, got %q", stdout)
	}
}

func TestSessionsShowTextFormat(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	mockPath := mockAgentPath(t)
	if _, err := os.Stat(mockPath); err != nil {
		t.Skip("mock agent not found")
	}
	cwd := t.TempDir()
	code, newOut, _ := runCLI(t, []string{"codeye", "--cwd", cwd, "--agent", "go run " + mockPath, "--format", "json", "sessions", "new"})
	if code != 0 {
		t.Skip("sessions new failed")
	}
	sessionID := extractSessionIDFromJSON(t, newOut)
	code, stdout, _ := runCLI(t, []string{"codeye", "--cwd", cwd, "--agent", "go run " + mockPath, "sessions", "show", sessionID})
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	if !strings.Contains(stdout, "id:") {
		t.Fatalf("expected text format with id:, got %q", stdout)
	}
}

func TestDetectRequestedFormatJSON(t *testing.T) {
	code, stdout, _ := runCLI(t, []string{"codeye", "--format", "json-strict"})
	if code != 2 {
		t.Fatalf("expected 2, got %d", code)
	}
	if !strings.Contains(stdout, "jsonrpc") {
		t.Fatalf("expected json output, got %q", stdout)
	}
}

func runCLI(t *testing.T, argv []string) (int, string, string) {
	t.Helper()
	origOut := os.Stdout
	origErr := os.Stderr
	rOut, wOut, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	rErr, wErr, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = wOut
	os.Stderr = wErr
	code := cli.Run(argv)
	_ = wOut.Close()
	_ = wErr.Close()
	outB, _ := io.ReadAll(rOut)
	errB, _ := io.ReadAll(rErr)
	os.Stdout = origOut
	os.Stderr = origErr
	return code, string(outB), string(errB)
}
