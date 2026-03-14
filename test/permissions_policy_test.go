package codeye_test

import (
	"testing"

	"github.com/one710/codeye/internal/permissions"
)

func TestDecide(t *testing.T) {
	if got := permissions.Decide(permissions.ApproveAll, "fs/write_text_file"); got != permissions.DecisionApproved {
		t.Fatalf("approve-all expected approved, got %s", got)
	}
	if got := permissions.Decide(permissions.ApproveReads, "fs/read_text_file"); got != permissions.DecisionApproved {
		t.Fatalf("approve-reads read expected approved, got %s", got)
	}
	if got := permissions.Decide(permissions.ApproveReads, "fs/write_text_file"); got != permissions.DecisionDenied {
		t.Fatalf("approve-reads write expected denied, got %s", got)
	}
	if got := permissions.Decide(permissions.DenyAll, "fs/read_text_file"); got != permissions.DecisionDenied {
		t.Fatalf("deny-all expected denied, got %s", got)
	}
}
