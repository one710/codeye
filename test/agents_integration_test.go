// Integration tests for cursor and codex-acp agents.
// Run with: go test -v -run TestAgentsIntegration -timeout 15m ./test/
// These tests are skipped when -short is set (CI runs go test -short).
//
// Real-agent tests (CreateSession, Exec, Prompt) skip when binary not in PATH.
// Exec and Prompt can take 1–2 min per agent; use a generous timeout.
package codeye_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/one710/codeye/internal/acp"
	"github.com/one710/codeye/internal/client"
	"github.com/one710/codeye/internal/permissions"
)

type agentDef struct {
	name    string
	command string
	args    []string
}

var realAgents = []agentDef{
	{name: "cursor", command: "agent", args: []string{"acp"}},
	{name: "codex-acp", command: "codex-acp", args: nil},
}

func mockAgentPathForAgents(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("unable to determine test file path")
	}
	return filepath.Join(filepath.Dir(thisFile), "testdata", "mockagent", "main.go")
}

func newClientWithCwd(t *testing.T, cwd string, cmd string, args []string, mode permissions.Mode, opts ...func(*client.Options)) *client.Client {
	t.Helper()
	if args == nil {
		args = []string{}
	}
	o := client.Options{
		Command:              cmd,
		Args:                 args,
		Cwd:                  cwd,
		Mode:                 mode,
		InitializeTimeout:    20 * time.Second,
		NewSessionTimeout:    60 * time.Second,
		NonInteractivePolicy: permissions.NonInteractiveDeny,
	}
	for _, fn := range opts {
		fn(&o)
	}
	return client.New(o)
}

func TestAgentsIntegration_CreateSession(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	for _, ag := range realAgents {
		ag := ag
		t.Run(ag.name, func(t *testing.T) {
			if _, err := exec.LookPath(ag.command); err != nil {
				t.Skipf("%s not in PATH", ag.command)
			}
			cwd := t.TempDir()
			c := newClientWithCwd(t, cwd, ag.command, ag.args, permissions.ApproveAll)
			ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
			defer cancel()
			if err := c.Start(ctx); err != nil {
				t.Fatalf("Start: %v", err)
			}
			defer c.Close()
			sid, err := c.CreateSession(ctx, cwd)
			if err != nil {
				t.Fatalf("CreateSession: %v", err)
			}
			if sid == "" {
				t.Fatal("expected non-empty session id")
			}
		})
	}
}

func TestAgentsIntegration_Exec(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	for _, ag := range realAgents {
		ag := ag
		t.Run(ag.name, func(t *testing.T) {
			if _, err := exec.LookPath(ag.command); err != nil {
				t.Skipf("%s not in PATH", ag.command)
			}
			cwd := t.TempDir()
			c := newClientWithCwd(t, cwd, ag.command, ag.args, permissions.ApproveAll)
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
			defer cancel()
			if err := c.Start(ctx); err != nil {
				t.Fatalf("Start: %v", err)
			}
			defer c.Close()
			sid, err := c.CreateSession(ctx, cwd)
			if err != nil {
				t.Fatalf("CreateSession: %v", err)
			}
			ctxPrompt, cancelPrompt := context.WithTimeout(ctx, 2*time.Minute)
			defer cancelPrompt()
			result, err := c.Prompt(ctxPrompt, sid, acp.TextPrompt("Reply with exactly: OK"))
			if err != nil {
				t.Fatalf("Prompt: %v", err)
			}
			if result.StopReason == "" {
				t.Logf("Prompt OK (no stopReason), text=%q", result.Text)
			} else {
				t.Logf("Prompt OK, stopReason=%s", result.StopReason)
			}
		})
	}
}

func TestAgentsIntegration_Prompt(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	for _, ag := range realAgents {
		ag := ag
		t.Run(ag.name, func(t *testing.T) {
			if _, err := exec.LookPath(ag.command); err != nil {
				t.Skipf("%s not in PATH", ag.command)
			}
			cwd := t.TempDir()
			c := newClientWithCwd(t, cwd, ag.command, ag.args, permissions.ApproveAll)
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
			defer cancel()
			if err := c.Start(ctx); err != nil {
				t.Fatalf("Start: %v", err)
			}
			defer c.Close()
			sid, err := c.CreateSession(ctx, cwd)
			if err != nil {
				t.Fatalf("CreateSession: %v", err)
			}
			ctxPrompt, cancelPrompt := context.WithTimeout(ctx, 2*time.Minute)
			defer cancelPrompt()
			_, err = c.Prompt(ctxPrompt, sid, acp.TextPrompt("What is 2+2? Reply with one word."))
			if err != nil {
				t.Fatalf("Prompt: %v", err)
			}
		})
	}
}

func TestAgentsIntegration_FollowUpPrompt(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	// Test that session history is preserved: first prompt sets context, second prompt
	// asks for it. Validates session/load or in-process history is used.
	for _, ag := range realAgents {
		ag := ag
		t.Run(ag.name, func(t *testing.T) {
			if _, err := exec.LookPath(ag.command); err != nil {
				t.Skipf("%s not in PATH", ag.command)
			}
			cwd := t.TempDir()
			c := newClientWithCwd(t, cwd, ag.command, ag.args, permissions.ApproveAll)
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
			defer cancel()
			if err := c.Start(ctx); err != nil {
				t.Fatalf("Start: %v", err)
			}
			defer c.Close()
			sid, err := c.CreateSession(ctx, cwd)
			if err != nil {
				t.Fatalf("CreateSession: %v", err)
			}
			ctxPrompt, cancelPrompt := context.WithTimeout(ctx, 2*time.Minute)
			defer cancelPrompt()

			// First prompt: establish context the agent must remember
			_, err = c.Prompt(ctxPrompt, sid, acp.TextPrompt("Remember this secret code: CODESESSION99. You will be asked for it next. Reply with OK."))
			if err != nil {
				t.Fatalf("First prompt: %v", err)
			}

			// Second prompt: ask for the remembered context
			result, err := c.Prompt(ctxPrompt, sid, acp.TextPrompt("What was the secret code I asked you to remember? Reply with only the code, nothing else."))
			if err != nil {
				t.Fatalf("Follow-up prompt: %v", err)
			}
			if !strings.Contains(strings.ToUpper(result.Text), "CODESESSION99") {
				t.Fatalf("expected follow-up response to contain CODESESSION99 (session history used), got text=%q", result.Text)
			}
			t.Logf("Follow-up OK, agent used session history")
		})
	}
}

func TestAgentsIntegration_PermissionModes(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	mockPath := mockAgentPathForAgents(t)
	if _, err := os.Stat(mockPath); err != nil {
		t.Skip("mock agent not found")
	}

	prev := os.Getenv("MOCK_TOOL_DIR")
	t.Cleanup(func() { os.Setenv("MOCK_TOOL_DIR", prev) })

	modes := []struct {
		name permissions.Mode
	}{
		{permissions.ApproveAll},
		{permissions.ApproveReads},
		{permissions.DenyAll},
		{permissions.Ask},
	}

	for _, m := range modes {
		m := m
		t.Run(string(m.name), func(t *testing.T) {
			// Use fresh toolDir per run for approve-all write check
			runDir := t.TempDir()
			os.WriteFile(filepath.Join(runDir, "readable.txt"), []byte("test-content"), 0o644)
			prevRun := os.Getenv("MOCK_TOOL_DIR")
			os.Setenv("MOCK_TOOL_DIR", runDir)
			t.Cleanup(func() { os.Setenv("MOCK_TOOL_DIR", prevRun) })

			o := client.Options{
				Command:              "go",
				Args:                 []string{"run", mockPath},
				Cwd:                  runDir,
				Mode:                 m.name,
				InitializeTimeout:    15 * time.Second,
				NewSessionTimeout:    30 * time.Second,
				NonInteractivePolicy: permissions.NonInteractiveDeny,
			}
			if m.name == permissions.Ask {
				o.OnPermissionRequest = func(req client.PermissionRequest) string {
					for _, opt := range req.Options {
						if strings.Contains(strings.ToLower(opt.Kind), "allow") {
							return opt.OptionID
						}
					}
					return ""
				}
			}
			c := client.New(o)

			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()
			if err := c.Start(ctx); err != nil {
				t.Fatalf("Start: %v", err)
			}
			defer c.Close()
			sid, err := c.CreateSession(ctx, runDir)
			if err != nil {
				t.Fatalf("CreateSession: %v", err)
			}
			result, err := c.Prompt(ctx, sid, acp.TextPrompt("test-tools"))
			if err != nil {
				t.Fatalf("Prompt: %v", err)
			}
			if result.StopReason != "end_turn" {
				t.Fatalf("expected end_turn, got %s", result.StopReason)
			}
			if m.name == permissions.ApproveAll {
				written, err := os.ReadFile(filepath.Join(runDir, "written.txt"))
				if err != nil {
					t.Fatalf("approve-all: expected written.txt: %v", err)
				}
				if string(written) != "agent-wrote" {
					t.Fatalf("approve-all: expected 'agent-wrote', got %q", string(written))
				}
			}
		})
	}
}
