package codeye_test

import (
	"fmt"
	"testing"

	cerr "github.com/one710/codeye/internal/errors"
)

func TestOutputErrorMessage(t *testing.T) {
	e := &cerr.OutputError{Code: cerr.CodeUsage, Message: "bad flag"}
	if got := e.Error(); got != "bad flag" {
		t.Fatalf("expected 'bad flag', got %q", got)
	}
}

func TestOutputErrorMessageWithWrapped(t *testing.T) {
	inner := fmt.Errorf("root cause")
	e := &cerr.OutputError{Code: cerr.CodeRuntime, Message: "operation failed", Err: inner}
	if got := e.Error(); got != "operation failed: root cause" {
		t.Fatalf("unexpected error string: %q", got)
	}
}

func TestWrap(t *testing.T) {
	inner := fmt.Errorf("io error")
	err := cerr.Wrap(cerr.CodeTimeout, "timed out", inner)
	var oe *cerr.OutputError
	if !cerr.As(err, &oe) {
		t.Fatal("expected OutputError")
	}
	if oe.Code != cerr.CodeTimeout {
		t.Fatalf("expected code %s, got %s", cerr.CodeTimeout, oe.Code)
	}
	if oe.Err != inner {
		t.Fatal("expected inner error to be preserved")
	}
}

func TestExitCodeMapping(t *testing.T) {
	cases := []struct {
		code     string
		expected int
	}{
		{cerr.CodeUsage, 2},
		{cerr.CodePermissionDenied, 4},
		{cerr.CodeTimeout, 124},
		{cerr.CodeAgentUnavailable, 10},
		{cerr.CodeRuntime, 1},
	}
	for _, tc := range cases {
		err := cerr.Wrap(tc.code, "msg", nil)
		if got := cerr.ExitCode(err); got != tc.expected {
			t.Errorf("ExitCode(%s) = %d, want %d", tc.code, got, tc.expected)
		}
	}
	if cerr.ExitCode(nil) != 0 {
		t.Error("ExitCode(nil) should be 0")
	}
	if cerr.ExitCode(fmt.Errorf("plain")) != 1 {
		t.Error("ExitCode(plain error) should be 1")
	}
}

func TestJSONRPCCodeMapping(t *testing.T) {
	cases := []struct {
		code     string
		expected int
	}{
		{cerr.CodeUsage, -32602},
		{cerr.CodePermissionDenied, -32000},
		{cerr.CodeTimeout, -32603},
		{cerr.CodeAgentUnavailable, -32603},
		{cerr.CodeRuntime, -32603},
	}
	for _, tc := range cases {
		err := cerr.Wrap(tc.code, "msg", nil)
		if got := cerr.JSONRPCCode(err); got != tc.expected {
			t.Errorf("JSONRPCCode(%s) = %d, want %d", tc.code, got, tc.expected)
		}
	}
	if cerr.JSONRPCCode(nil) != -32603 {
		t.Error("JSONRPCCode(nil) should be -32603")
	}
	if cerr.JSONRPCCode(fmt.Errorf("plain")) != -32603 {
		t.Error("JSONRPCCode(plain error) should be -32603")
	}
}

func TestErrorCodeMapping(t *testing.T) {
	cases := []string{cerr.CodeUsage, cerr.CodeRuntime, cerr.CodePermissionDenied, cerr.CodeTimeout, cerr.CodeAgentUnavailable}
	for _, code := range cases {
		err := cerr.Wrap(code, "msg", nil)
		if got := cerr.ErrorCode(err); got != code {
			t.Errorf("ErrorCode(%s) = %q, want %q", code, got, code)
		}
	}
	if cerr.ErrorCode(nil) != cerr.CodeRuntime {
		t.Error("ErrorCode(nil) should be RUNTIME")
	}
	if cerr.ErrorCode(fmt.Errorf("plain")) != cerr.CodeRuntime {
		t.Error("ErrorCode(plain error) should be RUNTIME")
	}
}

func TestAsReturnsFalseForNonOutputError(t *testing.T) {
	var oe *cerr.OutputError
	if cerr.As(fmt.Errorf("plain"), &oe) {
		t.Fatal("As should return false for plain error")
	}
}

func TestAsReturnsFalseForNil(t *testing.T) {
	var oe *cerr.OutputError
	if cerr.As(nil, &oe) {
		t.Fatal("As should return false for nil")
	}
}

func TestAsUnwrapsChain(t *testing.T) {
	inner := cerr.Wrap(cerr.CodeTimeout, "inner", nil)
	outer := fmt.Errorf("wrapped: %w", inner)
	var oe *cerr.OutputError
	if !cerr.As(outer, &oe) {
		t.Fatal("As should unwrap the chain and find OutputError")
	}
	if oe.Code != cerr.CodeTimeout {
		t.Fatalf("expected TIMEOUT, got %s", oe.Code)
	}
}
