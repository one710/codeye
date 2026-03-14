package codeye_test

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	cerr "github.com/one710/codeye/internal/errors"
	"github.com/one710/codeye/internal/output"
)

func TestPrintSuccessText(t *testing.T) {
	var out bytes.Buffer
	e := output.Emitter{Format: output.Text, Out: &out, Err: &bytes.Buffer{}}
	e.PrintSuccess("session_ready", map[string]interface{}{"sessionId": "s1"})
	if got := strings.TrimSpace(out.String()); got != "session_ready" {
		t.Fatalf("expected 'session_ready' (no textMsg), got %q", got)
	}
}

func TestPrintSuccessJSON(t *testing.T) {
	var out bytes.Buffer
	e := output.Emitter{Format: output.JSON, Out: &out, Err: &bytes.Buffer{}}
	e.PrintSuccess("done", map[string]interface{}{"key": "val"})
	var parsed map[string]interface{}
	if err := json.Unmarshal(out.Bytes(), &parsed); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if parsed["action"] != "done" {
		t.Fatalf("expected action=done, got %v", parsed["action"])
	}
	if parsed["key"] != "val" {
		t.Fatalf("expected key=val, got %v", parsed["key"])
	}
}

func TestPrintSuccessJSONStrict(t *testing.T) {
	var out bytes.Buffer
	e := output.Emitter{Format: output.JSONStrict, Out: &out, Err: &bytes.Buffer{}}
	e.PrintSuccess("ok", nil)
	var parsed map[string]interface{}
	if err := json.Unmarshal(out.Bytes(), &parsed); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if parsed["action"] != "ok" {
		t.Fatalf("expected action=ok, got %v", parsed)
	}
}

func TestPrintSuccessQuietShowsSessionID(t *testing.T) {
	var out bytes.Buffer
	e := output.Emitter{Format: output.Quiet, Out: &out, Err: &bytes.Buffer{}}
	e.PrintSuccess("ready", map[string]interface{}{"sessionId": "abc"})
	if got := strings.TrimSpace(out.String()); got != "abc" {
		t.Fatalf("expected 'abc', got %q", got)
	}
}

func TestPrintSuccessQuietNoSessionID(t *testing.T) {
	var out bytes.Buffer
	e := output.Emitter{Format: output.Quiet, Out: &out, Err: &bytes.Buffer{}}
	e.PrintSuccess("ready", map[string]interface{}{"other": "val"})
	if out.Len() != 0 {
		t.Fatalf("quiet mode without sessionId should produce no output, got %q", out.String())
	}
}

func TestPrintError(t *testing.T) {
	var out, errBuf bytes.Buffer
	e := output.Emitter{Format: output.Text, Out: &out, Err: &errBuf}
	e.PrintError("something broke")
	if !strings.Contains(errBuf.String(), "something broke") {
		t.Fatalf("expected error on stderr, got %q", errBuf.String())
	}
}

func TestPrintErrorJSON(t *testing.T) {
	var out bytes.Buffer
	e := output.Emitter{Format: output.JSON, Out: &out, Err: &bytes.Buffer{}}
	e.PrintError("fail")
	var parsed map[string]interface{}
	if err := json.Unmarshal(out.Bytes(), &parsed); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	errObj, _ := parsed["error"].(map[string]interface{})
	if errObj["message"] != "fail" {
		t.Fatalf("expected message=fail, got %v", errObj)
	}
}

func TestPrintErrorWithCauseNil(t *testing.T) {
	var out, errBuf bytes.Buffer
	e := output.Emitter{Format: output.Text, Out: &out, Err: &errBuf}
	e.PrintErrorWithCause(nil, "fallback msg")
	if !strings.Contains(errBuf.String(), "fallback msg") {
		t.Fatalf("expected fallback on stderr, got %q", errBuf.String())
	}
}

func TestPrintErrorWithCauseTyped(t *testing.T) {
	var out bytes.Buffer
	e := output.Emitter{Format: output.JSON, Out: &out, Err: &bytes.Buffer{}}
	err := cerr.Wrap(cerr.CodeUsage, "bad input", nil)
	e.PrintErrorWithCause(err, "bad input")
	var parsed map[string]interface{}
	json.Unmarshal(out.Bytes(), &parsed)
	errObj, _ := parsed["error"].(map[string]interface{})
	data, _ := errObj["data"].(map[string]interface{})
	if data["errorCode"] != "USAGE" {
		t.Fatalf("expected errorCode=USAGE, got %v", data)
	}
	if code, ok := errObj["code"].(float64); !ok || int(code) != -32602 {
		t.Fatalf("expected JSON-RPC code -32602, got %v", errObj["code"])
	}
}

func TestPrintRPCErrorText(t *testing.T) {
	var out, errBuf bytes.Buffer
	e := output.Emitter{Format: output.Text, Out: &out, Err: &errBuf}
	e.PrintRPCError(-32602, "invalid params", nil)
	if !strings.Contains(errBuf.String(), "invalid params") {
		t.Fatalf("expected error on stderr, got %q", errBuf.String())
	}
	if out.Len() != 0 {
		t.Fatal("text mode should not write to stdout for errors")
	}
}

func TestPrintRPCErrorJSONWithData(t *testing.T) {
	var out bytes.Buffer
	e := output.Emitter{Format: output.JSONStrict, Out: &out, Err: &bytes.Buffer{}}
	e.PrintRPCError(-32601, "method not found", map[string]interface{}{"errorCode": "USAGE"})
	var parsed map[string]interface{}
	json.Unmarshal(out.Bytes(), &parsed)
	if parsed["jsonrpc"] != "2.0" {
		t.Fatal("expected jsonrpc 2.0")
	}
	errObj, _ := parsed["error"].(map[string]interface{})
	data, _ := errObj["data"].(map[string]interface{})
	if data["errorCode"] != "USAGE" {
		t.Fatalf("expected data.errorCode=USAGE, got %v", data)
	}
}

func TestPrintRPCErrorJSONNoData(t *testing.T) {
	var out bytes.Buffer
	e := output.Emitter{Format: output.JSON, Out: &out, Err: &bytes.Buffer{}}
	e.PrintRPCError(-32603, "internal error", nil)
	var parsed map[string]interface{}
	json.Unmarshal(out.Bytes(), &parsed)
	errObj, _ := parsed["error"].(map[string]interface{})
	if _, hasData := errObj["data"]; hasData {
		t.Fatal("expected no data field when nil")
	}
}
