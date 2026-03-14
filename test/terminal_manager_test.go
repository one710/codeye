package codeye_test

import (
	"strings"
	"testing"
	"time"

	"github.com/one710/codeye/internal/tools/terminal"
)

func TestWaitReturnsExitCode(t *testing.T) {
	m := terminal.New()
	id, err := m.Create("sh", "-c", "exit 3")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	result, waitErr := m.Wait(id)
	if waitErr == nil {
		t.Fatalf("expected non-nil wait error for non-zero exit")
	}
	if result.ExitCode != 3 {
		t.Fatalf("expected exitCode=3, got %d", result.ExitCode)
	}
}

func TestWaitSuccessfulExit(t *testing.T) {
	m := terminal.New()
	id, err := m.Create("sh", "-c", "exit 0")
	if err != nil {
		t.Fatal(err)
	}
	result, waitErr := m.Wait(id)
	if waitErr != nil {
		t.Fatalf("expected nil error for zero exit, got %v", waitErr)
	}
	if result.ExitCode != 0 {
		t.Fatalf("expected exitCode=0, got %d", result.ExitCode)
	}
}

func TestOutput(t *testing.T) {
	m := terminal.New()
	id, err := m.Create("echo", "hello world")
	if err != nil {
		t.Fatal(err)
	}
	m.Wait(id)
	out, err := m.Output(id)
	if err != nil {
		t.Fatalf("Output: %v", err)
	}
	if !strings.Contains(out, "hello world") {
		t.Fatalf("expected output to contain 'hello world', got %q", out)
	}
}

func TestOutputNotFound(t *testing.T) {
	m := terminal.New()
	_, err := m.Output(999)
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("expected not found error, got %v", err)
	}
}

func TestWaitNotFound(t *testing.T) {
	m := terminal.New()
	_, err := m.Wait(999)
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("expected not found error, got %v", err)
	}
}

func TestKill(t *testing.T) {
	m := terminal.New()
	id, err := m.Create("sleep", "60")
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(50 * time.Millisecond)
	if err := m.Kill(id); err != nil {
		t.Fatalf("Kill: %v", err)
	}
	result, _ := m.Wait(id)
	if result.ExitCode == 0 && result.Signal == "" {
		t.Fatal("expected non-zero exit or signal after kill")
	}
}

func TestKillNotFound(t *testing.T) {
	m := terminal.New()
	err := m.Kill(999)
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("expected not found error, got %v", err)
	}
}

func TestRelease(t *testing.T) {
	m := terminal.New()
	id, err := m.Create("echo", "bye")
	if err != nil {
		t.Fatal(err)
	}
	m.Wait(id)
	m.Release(id)
	_, err = m.Output(id)
	if err == nil {
		t.Fatal("expected not found after release")
	}
}

func TestCreateInvalidCommand(t *testing.T) {
	m := terminal.New()
	_, err := m.Create("nonexistent-binary-xyz-999")
	if err == nil {
		t.Fatal("expected error for invalid command")
	}
}
