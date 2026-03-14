package codeye_test

import (
	"testing"

	"github.com/one710/codeye/internal/permissions"
)

func TestIsReadOnlyMethods(t *testing.T) {
	readOnly := []string{"fs/read_text_file", "terminal/output", "terminal/wait_for_exit"}
	for _, m := range readOnly {
		if !permissions.IsReadOnly(m) {
			t.Errorf("expected %q to be read-only", m)
		}
	}
	writeMethods := []string{
		"fs/write_text_file", "terminal/create", "terminal/kill",
		"terminal/release", "unknown/method",
	}
	for _, m := range writeMethods {
		if permissions.IsReadOnly(m) {
			t.Errorf("expected %q to NOT be read-only", m)
		}
	}
}

func TestIsReadOnlyWithWhitespace(t *testing.T) {
	if !permissions.IsReadOnly("  fs/read_text_file  ") {
		t.Fatal("IsReadOnly should trim whitespace")
	}
}

func TestDecideApproveAll(t *testing.T) {
	methods := []string{"fs/read_text_file", "fs/write_text_file", "terminal/create"}
	for _, m := range methods {
		if got := permissions.Decide(permissions.ApproveAll, m); got != permissions.DecisionApproved {
			t.Errorf("ApproveAll + %s: expected approved, got %s", m, got)
		}
	}
}

func TestDecideApproveReads(t *testing.T) {
	if got := permissions.Decide(permissions.ApproveReads, "fs/read_text_file"); got != permissions.DecisionApproved {
		t.Errorf("expected approved for read, got %s", got)
	}
	if got := permissions.Decide(permissions.ApproveReads, "terminal/output"); got != permissions.DecisionApproved {
		t.Errorf("expected approved for terminal/output, got %s", got)
	}
	if got := permissions.Decide(permissions.ApproveReads, "fs/write_text_file"); got != permissions.DecisionDenied {
		t.Errorf("expected denied for write, got %s", got)
	}
	if got := permissions.Decide(permissions.ApproveReads, "terminal/create"); got != permissions.DecisionDenied {
		t.Errorf("expected denied for terminal/create, got %s", got)
	}
}

func TestDecideDenyAll(t *testing.T) {
	methods := []string{"fs/read_text_file", "fs/write_text_file", "terminal/create"}
	for _, m := range methods {
		if got := permissions.Decide(permissions.DenyAll, m); got != permissions.DecisionDenied {
			t.Errorf("DenyAll + %s: expected denied, got %s", m, got)
		}
	}
}

func TestDecideUnknownMode(t *testing.T) {
	if got := permissions.Decide("something-else", "fs/read_text_file"); got != permissions.DecisionDenied {
		t.Errorf("unknown mode should deny, got %s", got)
	}
}
