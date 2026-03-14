package codeye_test

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/one710/codeye/internal/client"
	"github.com/one710/codeye/internal/permissions"
	"github.com/one710/codeye/internal/queue"
	"github.com/one710/codeye/internal/session"
	"github.com/one710/codeye/internal/session/persistence"
)

func mockAgentPathForRuntime(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("unable to determine test file path")
	}
	return filepath.Join(filepath.Dir(thisFile), "testdata", "mockagent", "main.go")
}

func clientFactory(t *testing.T, cwd string) func() *client.Client {
	t.Helper()
	mockPath := mockAgentPathForRuntime(t)
	if _, err := os.Stat(mockPath); err != nil {
		t.Skip("mock agent not found")
	}
	return func() *client.Client {
		return client.New(client.Options{
			Command:              "go",
			Args:                 []string{"run", mockPath},
			Cwd:                  cwd,
			Mode:                 permissions.ApproveAll,
			InitializeTimeout:    10 * time.Second,
			NewSessionTimeout:    10 * time.Second,
			NonInteractivePolicy: permissions.NonInteractiveDeny,
		})
	}
}

func shortSocketForRuntime(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("/tmp", "codeye-rt-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	return filepath.Join(dir, "q.sock")
}

func TestCreateSessionCreatesNew(t *testing.T) {
	root := t.TempDir()
	cwd := t.TempDir()
	repo := persistence.New(root)
	rt := session.NewRuntime(repo, shortSocketForRuntime(t), 5*time.Second, 4, clientFactory(t, cwd))

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	rec, err := rt.CreateSession(ctx, "test-agent", cwd, "")
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	if rec.RecordID == "" || rec.ACPSession == "" {
		t.Fatal("expected non-empty record and ACP session IDs")
	}
}

func TestRunOnce(t *testing.T) {
	root := t.TempDir()
	cwd := t.TempDir()
	repo := persistence.New(root)
	rt := session.NewRuntime(repo, shortSocketForRuntime(t), 5*time.Second, 4, clientFactory(t, cwd))

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	stopReason, err := rt.RunOnce(ctx, cwd, "hello")
	if err != nil {
		t.Fatalf("RunOnce: %v", err)
	}
	if stopReason != "end_turn" {
		t.Fatalf("expected end_turn, got %s", stopReason)
	}
}

func TestListRemoteSessions(t *testing.T) {
	root := t.TempDir()
	cwd := t.TempDir()
	repo := persistence.New(root)
	rt := session.NewRuntime(repo, shortSocketForRuntime(t), 5*time.Second, 4, clientFactory(t, cwd))

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	sessions, err := rt.ListRemoteSessions(ctx, cwd)
	if err != nil {
		t.Fatalf("ListRemoteSessions: %v", err)
	}
	if len(sessions) == 0 {
		t.Fatal("expected at least one remote session")
	}
}

func TestDefaultRoot(t *testing.T) {
	root, err := session.DefaultRoot()
	if err != nil {
		t.Fatal(err)
	}
	if root == "" {
		t.Fatal("expected non-empty root")
	}
	info, err := os.Stat(root)
	if err != nil {
		t.Fatal(err)
	}
	if !info.IsDir() {
		t.Fatal("expected directory")
	}
}

func startQueueServer(t *testing.T) (string, *persistence.Repo) {
	t.Helper()
	root := t.TempDir()
	socket := shortSocketForRuntime(t)
	repo := persistence.New(root)

	srv := &queue.Server{
		SocketPath: socket,
		TTL:        5 * time.Second,
		Handler:    stubHandler{},
		MaxDepth:   4,
	}
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go func() { _ = srv.Run(ctx) }()
	time.Sleep(50 * time.Millisecond)
	return socket, repo
}

func TestPromptThroughQueue(t *testing.T) {
	t.Skip("prompt no longer uses queue; always in-process and requires ClientFactory")
	socket, repo := startQueueServer(t)
	rt := session.NewRuntime(repo, socket, 5*time.Second, 4, nil)
	rec := persistence.Record{RecordID: "r1", ACPSession: "s1", Agent: "test", Cwd: t.TempDir()}
	repo.Save(rec)

	stopReason, err := rt.Prompt(context.Background(), rec, "hello")
	if err != nil {
		t.Fatalf("Prompt: %v", err)
	}
	if stopReason != "end_turn" {
		t.Fatalf("expected end_turn, got %s", stopReason)
	}
}

func TestCancelThroughQueue(t *testing.T) {
	t.Skip("cancel no longer uses queue; always in-process and requires ClientFactory")
	socket, repo := startQueueServer(t)
	rt := session.NewRuntime(repo, socket, 5*time.Second, 4, nil)
	rec := persistence.Record{RecordID: "r1", ACPSession: "s1", Agent: "test", Cwd: t.TempDir()}
	if err := rt.Cancel(context.Background(), rec); err != nil {
		t.Fatalf("Cancel: %v", err)
	}
}

func TestSetModeThroughQueue(t *testing.T) {
	t.Skip("set-mode no longer uses queue; always in-process and requires ClientFactory")
	socket, repo := startQueueServer(t)
	rt := session.NewRuntime(repo, socket, 5*time.Second, 4, nil)
	rec := persistence.Record{RecordID: "r1", ACPSession: "s1", Agent: "test", Cwd: t.TempDir()}
	if err := rt.SetMode(context.Background(), rec, "plan"); err != nil {
		t.Fatalf("SetMode: %v", err)
	}
}

func TestSetConfigOptionThroughQueue(t *testing.T) {
	t.Skip("set no longer uses queue; always in-process and requires ClientFactory")
	socket, repo := startQueueServer(t)
	rt := session.NewRuntime(repo, socket, 5*time.Second, 4, nil)
	rec := persistence.Record{RecordID: "r1", ACPSession: "s1", Agent: "test", Cwd: t.TempDir()}
	if err := rt.SetConfigOption(context.Background(), rec, "key", "val"); err != nil {
		t.Fatalf("SetConfigOption: %v", err)
	}
}

func TestPromptEmptyStopReasonDefault(t *testing.T) {
	t.Skip("prompt no longer uses queue; always in-process and requires ClientFactory")
	socket := shortSocketForRuntime(t)
	srv := &queue.Server{
		SocketPath: socket,
		TTL:        5 * time.Second,
		Handler:    emptyStopReasonHandler{},
		MaxDepth:   4,
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = srv.Run(ctx) }()
	time.Sleep(50 * time.Millisecond)

	root := t.TempDir()
	repo := persistence.New(root)
	rt := session.NewRuntime(repo, socket, 5*time.Second, 4, nil)
	rec := persistence.Record{RecordID: "r1", ACPSession: "s1", Agent: "test", Cwd: root}
	_ = repo.Save(rec)

	stopReason, err := rt.Prompt(ctx, rec, "hello")
	if err != nil {
		t.Fatalf("Prompt: %v", err)
	}
	if stopReason != "end_turn" {
		t.Fatalf("expected end_turn default, got %s", stopReason)
	}
}

type emptyStopReasonHandler struct{}

func (emptyStopReasonHandler) Prompt(_ context.Context, _, _ string) (queue.PromptResult, error) {
	return queue.PromptResult{StopReason: ""}, nil
}
func (emptyStopReasonHandler) Cancel(_ context.Context, _ string) error     { return nil }
func (emptyStopReasonHandler) SetMode(_ context.Context, _, _ string) error { return nil }
func (emptyStopReasonHandler) SetConfigOption(_ context.Context, _, _, _ string) error {
	return nil
}

func TestCancelErrorNoQueue(t *testing.T) {
	t.Skip("cancel no longer uses queue; always in-process and requires ClientFactory")
	root := t.TempDir()
	socket := shortSocketForRuntime(t)
	repo := persistence.New(root)
	rt := session.NewRuntime(repo, socket, 5*time.Second, 4, nil)
	rec := persistence.Record{RecordID: "r1", ACPSession: "s1", Agent: "test", Cwd: root}
	err := rt.Cancel(context.Background(), rec)
	if err == nil {
		t.Fatal("expected error when queue is unavailable")
	}
}

func TestSetModeErrorNoQueue(t *testing.T) {
	t.Skip("set-mode no longer uses queue; always in-process and requires ClientFactory")
	root := t.TempDir()
	socket := shortSocketForRuntime(t)
	repo := persistence.New(root)
	rt := session.NewRuntime(repo, socket, 5*time.Second, 4, nil)
	rec := persistence.Record{RecordID: "r1", ACPSession: "s1", Agent: "test", Cwd: root}
	err := rt.SetMode(context.Background(), rec, "plan")
	if err == nil {
		t.Fatal("expected error when queue is unavailable")
	}
}

func TestSetConfigOptionErrorNoQueue(t *testing.T) {
	t.Skip("set no longer uses queue; always in-process and requires ClientFactory")
	root := t.TempDir()
	socket := shortSocketForRuntime(t)
	repo := persistence.New(root)
	rt := session.NewRuntime(repo, socket, 5*time.Second, 4, nil)
	rec := persistence.Record{RecordID: "r1", ACPSession: "s1", Agent: "test", Cwd: root}
	err := rt.SetConfigOption(context.Background(), rec, "k", "v")
	if err == nil {
		t.Fatal("expected error when queue is unavailable")
	}
}
