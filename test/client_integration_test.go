package codeye_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/one710/codeye/internal/client"
	"github.com/one710/codeye/internal/permissions"
)

func mockAgentPathForClient(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("unable to determine test file path")
	}
	return filepath.Join(filepath.Dir(thisFile), "testdata", "mockagent", "main.go")
}

func newTestClient(t *testing.T, opts ...func(*client.Options)) *client.Client {
	t.Helper()
	mockPath := mockAgentPathForClient(t)
	if _, err := os.Stat(mockPath); err != nil {
		t.Skip("mock agent not found")
	}
	cwd := t.TempDir()
	o := client.Options{
		Command:              "go",
		Args:                 []string{"run", mockPath},
		Cwd:                  cwd,
		Mode:                 permissions.ApproveAll,
		InitializeTimeout:    10 * time.Second,
		NewSessionTimeout:    10 * time.Second,
		NonInteractivePolicy: permissions.NonInteractiveDeny,
	}
	for _, fn := range opts {
		fn(&o)
	}
	return client.New(o)
}

func TestClientLifecycleWithMockAgent(t *testing.T) {
	c := newTestClient(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := c.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer c.Close()
	sid, err := c.CreateSession(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	if sid == "" {
		t.Fatal("expected session id")
	}
	result, err := c.Prompt(ctx, sid, "hello")
	if err != nil {
		t.Fatalf("Prompt: %v", err)
	}
	if result.StopReason != "end_turn" {
		t.Fatalf("expected end_turn, got %s", result.StopReason)
	}
}

func TestLoadSessionReplayDrainSuppressesReplayUpdates(t *testing.T) {
	var updates int32
	c := newTestClient(t, func(o *client.Options) {
		o.OnSessionUpdate = func(_ json.RawMessage) { atomic.AddInt32(&updates, 1) }
	})
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := c.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer c.Close()
	sid, _ := c.CreateSession(ctx, t.TempDir())
	if err := c.LoadSession(ctx, sid, t.TempDir()); err != nil {
		t.Fatalf("LoadSession: %v", err)
	}
	if got := atomic.LoadInt32(&updates); got != 0 {
		t.Fatalf("expected replay updates suppressed, got %d", got)
	}
	if _, err := c.Prompt(ctx, sid, "live"); err != nil {
		t.Fatalf("Prompt: %v", err)
	}
	if got := atomic.LoadInt32(&updates); got == 0 {
		t.Fatal("expected live update after prompt")
	}
}

func TestOptionalCapabilityGating(t *testing.T) {
	prev := os.Getenv("MOCK_CAPS")
	t.Cleanup(func() { os.Setenv("MOCK_CAPS", prev) })
	os.Setenv("MOCK_CAPS", "load")

	c := newTestClient(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	c.Start(ctx)
	defer c.Close()
	sid, _ := c.CreateSession(ctx, t.TempDir())

	_, err := c.ListSessions(ctx, t.TempDir())
	if err == nil || !strings.Contains(err.Error(), "session/list") {
		t.Fatalf("expected list cap error, got %v", err)
	}
	if err := c.SetMode(ctx, sid, "plan"); err == nil || !strings.Contains(err.Error(), "set_mode") {
		t.Fatalf("expected set_mode cap error, got %v", err)
	}
	if err := c.SetConfigOption(ctx, sid, "k", "v"); err == nil || !strings.Contains(err.Error(), "set_config") {
		t.Fatalf("expected set_config cap error, got %v", err)
	}
}

func TestListSessionsWhenCapabilityAdvertised(t *testing.T) {
	prev := os.Getenv("MOCK_CAPS")
	t.Cleanup(func() { os.Setenv("MOCK_CAPS", prev) })
	os.Setenv("MOCK_CAPS", "list,load,set-mode,set-config")

	c := newTestClient(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	c.Start(ctx)
	defer c.Close()
	sessions, err := c.ListSessions(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}
	if len(sessions) == 0 || sessions[0] != "mock-session" {
		t.Fatalf("unexpected sessions: %#v", sessions)
	}
}

func TestSetModeWhenCapabilityAdvertised(t *testing.T) {
	c := newTestClient(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	c.Start(ctx)
	defer c.Close()
	sid, _ := c.CreateSession(ctx, t.TempDir())
	if err := c.SetMode(ctx, sid, "plan"); err != nil {
		t.Fatalf("SetMode: %v", err)
	}
}

func TestSetConfigOptionWhenCapabilityAdvertised(t *testing.T) {
	c := newTestClient(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	c.Start(ctx)
	defer c.Close()
	sid, _ := c.CreateSession(ctx, t.TempDir())
	if err := c.SetConfigOption(ctx, sid, "reasoning_effort", "high"); err != nil {
		t.Fatalf("SetConfigOption: %v", err)
	}
}

func TestCancelSendsNotification(t *testing.T) {
	c := newTestClient(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	c.Start(ctx)
	defer c.Close()
	sid, _ := c.CreateSession(ctx, t.TempDir())
	if err := c.Cancel(ctx, sid); err != nil {
		t.Fatalf("Cancel: %v", err)
	}
}

func TestAuthenticatePolicyFailWithoutCredentials(t *testing.T) {
	prev := setAuthEnv(t)
	t.Cleanup(func() { restoreAuthEnv(prev) })

	c := newTestClient(t, func(o *client.Options) {
		o.AuthPolicy = "fail"
	})
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	err := c.Start(ctx)
	if err == nil || !strings.Contains(err.Error(), "no credentials found") {
		t.Fatalf("expected auth fail error, got %v", err)
	}
}

func TestAuthenticateWithCredentials(t *testing.T) {
	prev := setAuthEnv(t)
	t.Cleanup(func() { restoreAuthEnv(prev) })

	c := newTestClient(t, func(o *client.Options) {
		o.AuthPolicy = "fail"
		o.AuthCredentials = map[string]string{"api_key": "secret"}
	})
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := c.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer c.Close()
	sid, err := c.CreateSession(ctx, t.TempDir())
	if err != nil || sid == "" {
		t.Fatalf("CreateSession after auth: %v", err)
	}
}

func TestToolHandlingDuringPrompt(t *testing.T) {
	toolDir := t.TempDir()
	os.WriteFile(filepath.Join(toolDir, "readable.txt"), []byte("test-content"), 0o644)

	prev := os.Getenv("MOCK_TOOL_DIR")
	t.Cleanup(func() { os.Setenv("MOCK_TOOL_DIR", prev) })
	os.Setenv("MOCK_TOOL_DIR", toolDir)

	c := newTestClient(t, func(o *client.Options) {
		o.Cwd = toolDir
	})
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	if err := c.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer c.Close()
	sid, _ := c.CreateSession(ctx, toolDir)
	result, err := c.Prompt(ctx, sid, "test-tools")
	if err != nil {
		t.Fatalf("Prompt with tools: %v", err)
	}
	if result.StopReason != "end_turn" {
		t.Fatalf("expected end_turn, got %s", result.StopReason)
	}
	written, err := os.ReadFile(filepath.Join(toolDir, "written.txt"))
	if err != nil {
		t.Fatalf("expected written.txt to exist: %v", err)
	}
	if string(written) != "agent-wrote" {
		t.Fatalf("expected 'agent-wrote', got %q", string(written))
	}
}

func TestPromptEmptyStopReasonDefaultsToEndTurn(t *testing.T) {
	c := newTestClient(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := c.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer c.Close()
	sid, err := c.CreateSession(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	result, err := c.Prompt(ctx, sid, "no-stop-reason")
	if err != nil {
		t.Fatalf("Prompt: %v", err)
	}
	if result.StopReason != "end_turn" {
		t.Fatalf("expected end_turn default, got %s", result.StopReason)
	}
}

func TestStartWithEmptyCommand(t *testing.T) {
	c := client.New(client.Options{})
	err := c.Start(context.Background())
	if err == nil || !strings.Contains(err.Error(), "command is required") {
		t.Fatalf("expected command required error, got %v", err)
	}
}

func setAuthEnv(t *testing.T) [3]string {
	t.Helper()
	prev := [3]string{
		os.Getenv("MOCK_REQUIRE_AUTH"),
		os.Getenv("MOCK_AUTH_METHOD"),
		os.Getenv("MOCK_AUTH_CREDENTIAL"),
	}
	os.Setenv("MOCK_REQUIRE_AUTH", "1")
	os.Setenv("MOCK_AUTH_METHOD", "api_key")
	os.Setenv("MOCK_AUTH_CREDENTIAL", "secret")
	return prev
}

func restoreAuthEnv(prev [3]string) {
	os.Setenv("MOCK_REQUIRE_AUTH", prev[0])
	os.Setenv("MOCK_AUTH_METHOD", prev[1])
	os.Setenv("MOCK_AUTH_CREDENTIAL", prev[2])
}
