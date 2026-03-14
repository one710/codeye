package permissions

import "strings"

type Mode string

const (
	ApproveAll   Mode = "approve-all"
	ApproveReads Mode = "approve-reads"
	DenyAll      Mode = "deny-all"
	Ask          Mode = "ask"
)

type NonInteractive string

const (
	NonInteractiveDeny NonInteractive = "deny"
	NonInteractiveFail NonInteractive = "fail"
)

type Decision string

const (
	DecisionApproved  Decision = "approved"
	DecisionDenied    Decision = "denied"
	DecisionCancelled Decision = "cancelled"
)

func IsReadOnly(method string) bool {
	m := strings.TrimSpace(method)
	switch m {
	case "fs/read_text_file", "terminal/output", "terminal/wait_for_exit":
		return true
	default:
		return false
	}
}

func Decide(mode Mode, method string) Decision {
	switch mode {
	case ApproveAll:
		return DecisionApproved
	case ApproveReads:
		if IsReadOnly(method) {
			return DecisionApproved
		}
		return DecisionDenied
	default:
		return DecisionDenied
	}
}
