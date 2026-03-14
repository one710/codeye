package codeye_test

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/one710/codeye/internal/client"
	"github.com/one710/codeye/internal/permissions"
)

func TestClientPromptWithEmptyStopReason(t *testing.T) {
	c := newTestClient(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := c.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer c.Close()
	sid, _ := c.CreateSession(ctx, t.TempDir())
	result, err := c.Prompt(ctx, sid, "hello")
	if err != nil {
		t.Fatalf("Prompt: %v", err)
	}
	if result.StopReason != "end_turn" {
		t.Fatalf("expected default end_turn, got %s", result.StopReason)
	}
}

func TestClientContextCancelDuringCall(t *testing.T) {
	c := newTestClient(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := c.Start(ctx); err != nil {
		t.Fatal(err)
	}
	defer c.Close()

	deadCtx, deadCancel := context.WithCancel(ctx)
	deadCancel()
	_, err := c.CreateSession(deadCtx, t.TempDir())
	if err == nil {
		t.Fatal("expected error for cancelled context")
	}
}

func TestClientAuthCredentialsFromEnv(t *testing.T) {
	prev := os.Getenv("MOCK_REQUIRE_AUTH")
	os.Setenv("MOCK_REQUIRE_AUTH", "1")
	t.Cleanup(func() { os.Setenv("MOCK_REQUIRE_AUTH", prev) })

	prevCred := os.Getenv("API_KEY")
	os.Setenv("API_KEY", "secret")
	t.Cleanup(func() { os.Setenv("API_KEY", prevCred) })

	c := newTestClient(t, func(o *client.Options) {
		o.AuthPolicy = "fail"
	})
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := c.Start(ctx); err != nil {
		t.Fatalf("Start should succeed with env credential: %v", err)
	}
	c.Close()
}

func TestClientAuthCredentialsFromCodeyeEnv(t *testing.T) {
	prev := os.Getenv("MOCK_REQUIRE_AUTH")
	os.Setenv("MOCK_REQUIRE_AUTH", "1")
	t.Cleanup(func() { os.Setenv("MOCK_REQUIRE_AUTH", prev) })

	prevCred := os.Getenv("CODEYE_AUTH_API_KEY")
	os.Setenv("CODEYE_AUTH_API_KEY", "secret")
	t.Cleanup(func() { os.Setenv("CODEYE_AUTH_API_KEY", prevCred) })

	c := newTestClient(t, func(o *client.Options) {
		o.AuthPolicy = "fail"
	})
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := c.Start(ctx); err != nil {
		t.Fatalf("Start should succeed with CODEYE_AUTH_ env: %v", err)
	}
	c.Close()
}

func TestClientAuthPolicySkip(t *testing.T) {
	prev := os.Getenv("MOCK_REQUIRE_AUTH")
	os.Setenv("MOCK_REQUIRE_AUTH", "1")
	t.Cleanup(func() { os.Setenv("MOCK_REQUIRE_AUTH", prev) })

	c := newTestClient(t, func(o *client.Options) {
		o.AuthPolicy = "skip"
	})
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := c.Start(ctx); err != nil {
		t.Fatalf("Start should succeed with skip policy: %v", err)
	}
	c.Close()
}

func TestClientLoadSessionWithoutCapability(t *testing.T) {
	prev := os.Getenv("MOCK_CAPS")
	os.Setenv("MOCK_CAPS", "list")
	t.Cleanup(func() { os.Setenv("MOCK_CAPS", prev) })

	c := newTestClient(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	c.Start(ctx)
	defer c.Close()
	err := c.LoadSession(ctx, "s1", t.TempDir())
	if err == nil || !strings.Contains(err.Error(), "session/load") {
		t.Fatalf("expected load cap error, got %v", err)
	}
}

func TestClientToolHandlingDenyAll(t *testing.T) {
	toolDir := t.TempDir()
	os.WriteFile(toolDir+"/readable.txt", []byte("content"), 0o644)

	prev := os.Getenv("MOCK_TOOL_DIR")
	os.Setenv("MOCK_TOOL_DIR", toolDir)
	t.Cleanup(func() { os.Setenv("MOCK_TOOL_DIR", prev) })

	c := newTestClient(t, func(o *client.Options) {
		o.Cwd = toolDir
		o.Mode = permissions.DenyAll
	})
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	c.Start(ctx)
	defer c.Close()
	sid, _ := c.CreateSession(ctx, toolDir)
	result, err := c.Prompt(ctx, sid, "test-tools")
	if err != nil {
		t.Fatalf("Prompt should succeed even with deny-all: %v", err)
	}
	if result.StopReason != "end_turn" {
		t.Fatalf("expected end_turn, got %s", result.StopReason)
	}
}
