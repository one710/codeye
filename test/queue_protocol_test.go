package codeye_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/one710/codeye/internal/queue"
)

type stubHandler struct{}

func (stubHandler) Prompt(_ context.Context, _, _ string) (queue.PromptResult, error) {
	return queue.PromptResult{StopReason: "end_turn"}, nil
}
func (stubHandler) Cancel(_ context.Context, _ string) error     { return nil }
func (stubHandler) SetMode(_ context.Context, _, _ string) error { return nil }
func (stubHandler) SetConfigOption(_ context.Context, _, _, _ string) error {
	return nil
}

type failHandler struct{}

func (failHandler) Prompt(_ context.Context, _, _ string) (queue.PromptResult, error) {
	return queue.PromptResult{}, fmt.Errorf("prompt failed")
}
func (failHandler) Cancel(_ context.Context, _ string) error { return fmt.Errorf("cancel failed") }
func (failHandler) SetMode(_ context.Context, _, _ string) error {
	return fmt.Errorf("set mode failed")
}
func (failHandler) SetConfigOption(_ context.Context, _, _, _ string) error {
	return fmt.Errorf("set config failed")
}

func shortSocket(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("/tmp", "codeye-t-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	return filepath.Join(dir, "q.sock")
}

func startServer(t *testing.T, handler queue.Handler, maxDepth int) string {
	t.Helper()
	socket := shortSocket(t)
	srv := &queue.Server{
		SocketPath: socket,
		TTL:        5 * time.Second,
		Handler:    handler,
		MaxDepth:   maxDepth,
	}
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go func() { _ = srv.Run(ctx) }()
	time.Sleep(50 * time.Millisecond)
	return socket
}

func TestServerClientRoundTrip(t *testing.T) {
	socket := startServer(t, stubHandler{}, 4)
	resp, err := queue.Send(socket, queue.Request{Command: queue.CmdHealth}, time.Second)
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if !resp.OK {
		t.Fatalf("expected OK response")
	}
}

func TestPromptCarriesStopReason(t *testing.T) {
	socket := startServer(t, stubHandler{}, 4)
	resp, err := queue.Send(socket, queue.Request{Command: queue.CmdPrompt, SessionID: "s1", Prompt: "hi"}, time.Second)
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if !resp.OK {
		t.Fatalf("expected OK response")
	}
	if resp.StopReason != "end_turn" {
		t.Fatalf("expected stopReason=end_turn, got %q", resp.StopReason)
	}
}

func TestCancelCommand(t *testing.T) {
	socket := startServer(t, stubHandler{}, 4)
	resp, err := queue.Send(socket, queue.Request{Command: queue.CmdCancel, SessionID: "s1"}, time.Second)
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if !resp.OK {
		t.Fatal("expected OK for cancel")
	}
}

func TestSetModeCommand(t *testing.T) {
	socket := startServer(t, stubHandler{}, 4)
	resp, err := queue.Send(socket, queue.Request{Command: queue.CmdSetMode, SessionID: "s1", Mode: "plan"}, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if !resp.OK {
		t.Fatal("expected OK for set_mode")
	}
}

func TestSetConfigOptionCommand(t *testing.T) {
	socket := startServer(t, stubHandler{}, 4)
	resp, err := queue.Send(socket, queue.Request{Command: queue.CmdSetConfigOption, SessionID: "s1", Key: "k", Value: "v"}, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if !resp.OK {
		t.Fatal("expected OK for set_config_option")
	}
}

func TestQueueUnknownCommand(t *testing.T) {
	socket := startServer(t, stubHandler{}, 4)
	resp, err := queue.Send(socket, queue.Request{Command: "bogus"}, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if resp.OK {
		t.Fatal("expected failure for unknown command")
	}
}

func TestPromptFailureReturnsError(t *testing.T) {
	socket := startServer(t, failHandler{}, 4)
	resp, err := queue.Send(socket, queue.Request{Command: queue.CmdPrompt, SessionID: "s1", Prompt: "hi"}, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if resp.OK {
		t.Fatal("expected failure")
	}
	if resp.Code != "QUEUE_RUNTIME" {
		t.Fatalf("expected code QUEUE_RUNTIME, got %s", resp.Code)
	}
}

func TestCancelFailureReturnsError(t *testing.T) {
	socket := startServer(t, failHandler{}, 4)
	resp, err := queue.Send(socket, queue.Request{Command: queue.CmdCancel, SessionID: "s1"}, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if resp.OK {
		t.Fatal("expected failure")
	}
}

func TestSetModeFailureReturnsError(t *testing.T) {
	socket := startServer(t, failHandler{}, 4)
	resp, err := queue.Send(socket, queue.Request{Command: queue.CmdSetMode, SessionID: "s1", Mode: "x"}, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if resp.OK {
		t.Fatal("expected failure")
	}
}

func TestSetConfigFailureReturnsError(t *testing.T) {
	socket := startServer(t, failHandler{}, 4)
	resp, err := queue.Send(socket, queue.Request{Command: queue.CmdSetConfigOption, SessionID: "s1", Key: "k", Value: "v"}, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if resp.OK {
		t.Fatal("expected failure")
	}
}

type slowHandler struct{}

func (slowHandler) Prompt(_ context.Context, _, _ string) (queue.PromptResult, error) {
	time.Sleep(2 * time.Second)
	return queue.PromptResult{StopReason: "end_turn"}, nil
}
func (slowHandler) Cancel(_ context.Context, _ string) error                { return nil }
func (slowHandler) SetMode(_ context.Context, _, _ string) error            { return nil }
func (slowHandler) SetConfigOption(_ context.Context, _, _, _ string) error { return nil }

func TestQueueOverload(t *testing.T) {
	socket := startServer(t, slowHandler{}, 1)
	go func() {
		queue.Send(socket, queue.Request{Command: queue.CmdPrompt, SessionID: "s1", Prompt: "hi"}, 5*time.Second)
	}()
	time.Sleep(100 * time.Millisecond)
	resp, err := queue.Send(socket, queue.Request{Command: queue.CmdPrompt, SessionID: "s2", Prompt: "hi"}, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if resp.OK {
		t.Fatal("expected overload rejection")
	}
	if resp.Code != "QUEUE_ERROR" {
		t.Fatalf("expected QUEUE_ERROR, got %s", resp.Code)
	}
	if !resp.Retryable {
		t.Fatal("overload should be retryable")
	}
}

func TestRequestIDCarriedThrough(t *testing.T) {
	socket := startServer(t, stubHandler{}, 4)
	resp, err := queue.Send(socket, queue.Request{RequestID: "req-42", Command: queue.CmdHealth}, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if resp.RequestID != "req-42" {
		t.Fatalf("expected request ID req-42, got %s", resp.RequestID)
	}
}

func TestAutoGeneratedRequestID(t *testing.T) {
	socket := startServer(t, stubHandler{}, 4)
	resp, err := queue.Send(socket, queue.Request{Command: queue.CmdHealth}, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if resp.RequestID == "" {
		t.Fatal("expected auto-generated request ID")
	}
}

func TestSendToNonexistentSocket(t *testing.T) {
	_, err := queue.Send("/nonexistent.sock", queue.Request{Command: queue.CmdHealth}, 100*time.Millisecond)
	if err == nil {
		t.Fatal("expected error for nonexistent socket")
	}
}

func TestServerPIDInResponse(t *testing.T) {
	socket := startServer(t, stubHandler{}, 4)
	resp, err := queue.Send(socket, queue.Request{Command: queue.CmdHealth}, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if resp.PID == 0 {
		t.Fatal("expected non-zero PID")
	}
}
