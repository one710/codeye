package codeye_test

import (
	"testing"

	"github.com/one710/codeye/internal/acp"
)

func TestNewRequest(t *testing.T) {
	req := acp.NewRequest("1", "initialize", map[string]int{"protocolVersion": 1})
	if req.JSONRPC != "2.0" {
		t.Fatal("expected jsonrpc 2.0")
	}
	if req.ID != "1" || req.Method != "initialize" {
		t.Fatalf("unexpected request: %+v", req)
	}
}

func TestNewNotification(t *testing.T) {
	n := acp.NewNotification("session/cancel", map[string]string{"sessionId": "s1"})
	if n.JSONRPC != "2.0" {
		t.Fatal("expected jsonrpc 2.0")
	}
	if n.ID != nil {
		t.Fatal("notifications should not have an ID")
	}
	if n.Method != "session/cancel" {
		t.Fatalf("expected method session/cancel, got %s", n.Method)
	}
}

func TestDecodeMessageRequest(t *testing.T) {
	line := []byte(`{"jsonrpc":"2.0","id":"1","method":"initialize","params":{}}`)
	req, _, isReq, err := acp.DecodeMessage(line)
	if err != nil {
		t.Fatal(err)
	}
	if !isReq {
		t.Fatal("expected request")
	}
	if req.Method != "initialize" {
		t.Fatalf("expected initialize, got %s", req.Method)
	}
}

func TestDecodeMessageResponse(t *testing.T) {
	line := []byte(`{"jsonrpc":"2.0","id":"1","result":{"sessionId":"s1"}}`)
	_, resp, isReq, err := acp.DecodeMessage(line)
	if err != nil {
		t.Fatal(err)
	}
	if isReq {
		t.Fatal("expected response")
	}
	if resp.ID != "1" {
		t.Fatalf("unexpected id: %v", resp.ID)
	}
}

func TestDecodeMessageErrorResponse(t *testing.T) {
	line := []byte(`{"jsonrpc":"2.0","id":"2","error":{"code":-32601,"message":"not found"}}`)
	_, resp, isReq, err := acp.DecodeMessage(line)
	if err != nil {
		t.Fatal(err)
	}
	if isReq {
		t.Fatal("expected response")
	}
	if resp.Error == nil || resp.Error.Code != -32601 {
		t.Fatalf("unexpected error: %+v", resp.Error)
	}
}

func TestDecodeMessageNotification(t *testing.T) {
	line := []byte(`{"jsonrpc":"2.0","method":"session/update","params":{"event":"chunk"}}`)
	req, _, isReq, err := acp.DecodeMessage(line)
	if err != nil {
		t.Fatal(err)
	}
	if !isReq {
		t.Fatal("notifications should be decoded as requests")
	}
	if req.Method != "session/update" {
		t.Fatalf("expected session/update, got %s", req.Method)
	}
}

func TestDecodeMessageInvalid(t *testing.T) {
	_, _, _, err := acp.DecodeMessage([]byte("not json"))
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}
