package codeye_test

import (
	"context"
	"testing"
	"time"

	"github.com/one710/codeye/internal/acp"
	"github.com/one710/codeye/internal/client"
	"github.com/one710/codeye/internal/permissions"
	"github.com/one710/codeye/internal/queue"
	"github.com/one710/codeye/internal/session"
	"github.com/one710/codeye/internal/session/persistence"
)

func binaryClientFactory(t *testing.T, cwd string) func() *client.Client {
	t.Helper()
	if mockAgentBinary == "" {
		t.Skip("mock agent binary not built")
	}
	return func() *client.Client {
		return client.New(client.Options{
			Command:              mockAgentBinary,
			Cwd:                  cwd,
			Mode:                 permissions.ApproveAll,
			InitializeTimeout:    10 * time.Second,
			NewSessionTimeout:    10 * time.Second,
			NonInteractivePolicy: permissions.NonInteractiveDeny,
		})
	}
}

func TestRunWorkingSessionLifecycle(t *testing.T) {
	if mockAgentBinary == "" {
		t.Skip("mock agent binary not built")
	}
	root := t.TempDir()
	cwd := t.TempDir()
	socket := shortSocketForRuntime(t)
	repo := persistence.New(root)

	cf := binaryClientFactory(t, cwd)
	rt := session.NewRuntime(repo, socket, 5*time.Second, 4, cf)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	rec, err := rt.CreateSession(ctx, "test-agent", cwd, "")
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	wsCtx, wsCancel := context.WithCancel(context.Background())
	defer wsCancel()
	errCh := make(chan error, 1)
	go func() { errCh <- rt.RunWorkingSession(wsCtx, rec) }()

	for i := 0; i < 50; i++ {
		time.Sleep(100 * time.Millisecond)
		resp, err := queue.Send(socket, queue.Request{Command: queue.CmdHealth}, 200*time.Millisecond)
		if err == nil && resp.OK {
			goto healthy
		}
	}
	t.Fatal("working session never became healthy")

healthy:
	resp, err := queue.Send(socket, queue.Request{
		Command:   queue.CmdPrompt,
		SessionID: rec.ACPSession,
		Prompt:    "hello",
	}, 10*time.Second)
	if err != nil {
		t.Fatalf("Prompt: %v", err)
	}
	if !resp.OK {
		t.Fatalf("expected OK, got: %s", resp.Message)
	}
	if resp.StopReason != "end_turn" {
		t.Fatalf("expected end_turn, got %s", resp.StopReason)
	}

	resp, err = queue.Send(socket, queue.Request{Command: queue.CmdCancel, SessionID: rec.ACPSession}, time.Second)
	if err != nil {
		t.Fatalf("Cancel: %v", err)
	}
	if !resp.OK {
		t.Fatalf("Cancel: %s", resp.Message)
	}

	resp, err = queue.Send(socket, queue.Request{Command: queue.CmdSetMode, SessionID: rec.ACPSession, Mode: "plan"}, time.Second)
	if err != nil {
		t.Fatalf("SetMode: %v", err)
	}
	if !resp.OK {
		t.Fatalf("SetMode: %s", resp.Message)
	}

	resp, err = queue.Send(socket, queue.Request{
		Command:   queue.CmdSetConfigOption,
		SessionID: rec.ACPSession,
		Key:       "reasoning",
		Value:     "high",
	}, time.Second)
	if err != nil {
		t.Fatalf("SetConfigOption: %v", err)
	}
	if !resp.OK {
		t.Fatalf("SetConfigOption: %s", resp.Message)
	}

	wsCancel()
	select {
	case <-errCh:
	case <-time.After(3 * time.Second):
	}
}

func TestEnsureWorkingSessionHealthyShortCircuit(t *testing.T) {
	t.Skip("prompt no longer uses queue; always in-process and requires ClientFactory")
	socket := shortSocketForRuntime(t)
	srv := &queue.Server{
		SocketPath: socket,
		TTL:        5 * time.Second,
		Handler:    stubHandler{},
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
	repo.Save(rec)

	stopReason, err := rt.Prompt(ctx, rec, acp.TextPrompt("test"))
	if err != nil {
		t.Fatalf("Prompt should succeed when queue is already healthy: %v", err)
	}
	if stopReason != "end_turn" {
		t.Fatalf("expected end_turn, got %s", stopReason)
	}
}

func TestCancelErrorPath(t *testing.T) {
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

func TestSetModeErrorPath(t *testing.T) {
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

func TestSetConfigOptionErrorPath(t *testing.T) {
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

func TestCreateSessionClientStartFails(t *testing.T) {
	root := t.TempDir()
	repo := persistence.New(root)
	rt := session.NewRuntime(repo, shortSocketForRuntime(t), 5*time.Second, 4, func() *client.Client {
		return client.New(client.Options{})
	})
	_, err := rt.CreateSession(context.Background(), "test", root, "")
	if err == nil {
		t.Fatal("expected error for empty command")
	}
}

func TestRunOnceClientStartFails(t *testing.T) {
	root := t.TempDir()
	repo := persistence.New(root)
	rt := session.NewRuntime(repo, shortSocketForRuntime(t), 5*time.Second, 4, func() *client.Client {
		return client.New(client.Options{})
	})
	_, err := rt.RunOnce(context.Background(), root, acp.TextPrompt("hello"))
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestListRemoteSessionsClientStartFails(t *testing.T) {
	root := t.TempDir()
	repo := persistence.New(root)
	rt := session.NewRuntime(repo, shortSocketForRuntime(t), 5*time.Second, 4, func() *client.Client {
		return client.New(client.Options{})
	})
	_, err := rt.ListRemoteSessions(context.Background(), root)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestPromptRespNotOK(t *testing.T) {
	t.Skip("prompt no longer uses queue; always in-process and requires ClientFactory")
	socket := shortSocketForRuntime(t)
	srv := &queue.Server{
		SocketPath: socket,
		TTL:        5 * time.Second,
		Handler:    failHandler{},
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
	_, err := rt.Prompt(ctx, rec, acp.TextPrompt("hello"))
	if err == nil {
		t.Fatal("expected error for failed prompt")
	}
}

func TestCancelRespNotOK(t *testing.T) {
	t.Skip("cancel no longer uses queue; always in-process and requires ClientFactory")
	socket := shortSocketForRuntime(t)
	srv := &queue.Server{
		SocketPath: socket,
		TTL:        5 * time.Second,
		Handler:    failHandler{},
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
	err := rt.Cancel(ctx, rec)
	if err == nil {
		t.Fatal("expected error for failed cancel")
	}
}

func TestSetModeRespNotOK(t *testing.T) {
	t.Skip("set-mode no longer uses queue; always in-process and requires ClientFactory")
	socket := shortSocketForRuntime(t)
	srv := &queue.Server{
		SocketPath: socket,
		TTL:        5 * time.Second,
		Handler:    failHandler{},
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
	err := rt.SetMode(ctx, rec, "plan")
	if err == nil {
		t.Fatal("expected error for failed set_mode")
	}
}

func TestSetConfigOptionRespNotOK(t *testing.T) {
	t.Skip("set no longer uses queue; always in-process and requires ClientFactory")
	socket := shortSocketForRuntime(t)
	srv := &queue.Server{
		SocketPath: socket,
		TTL:        5 * time.Second,
		Handler:    failHandler{},
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
	err := rt.SetConfigOption(ctx, rec, "k", "v")
	if err == nil {
		t.Fatal("expected error for failed set_config_option")
	}
}
