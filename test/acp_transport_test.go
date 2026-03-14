package codeye_test

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/one710/codeye/internal/acp"
)

type errWriter struct{ err error }

func (e errWriter) Write([]byte) (int, error) { return 0, e.err }

type blockingReader struct{}

func (blockingReader) Read([]byte) (int, error) { select {} }

func TestTransportWriteMessageSuccess(t *testing.T) {
	buf := &bytes.Buffer{}
	tr := acp.NewTransport(strings.NewReader(""), buf)
	err := tr.WriteMessage(map[string]string{"hello": "world"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "hello") {
		t.Fatalf("expected hello in output, got %q", buf.String())
	}
}

func TestTransportWriteMessageWriteError(t *testing.T) {
	tr := acp.NewTransport(strings.NewReader(""), errWriter{err: errors.New("disk full")})
	err := tr.WriteMessage(map[string]string{"hello": "world"})
	if err == nil || !strings.Contains(err.Error(), "disk full") {
		t.Fatalf("expected disk full error, got %v", err)
	}
}

func TestTransportWriteMessageMarshalError(t *testing.T) {
	tr := acp.NewTransport(strings.NewReader(""), &bytes.Buffer{})
	err := tr.WriteMessage(make(chan int))
	if err == nil {
		t.Fatal("expected marshal error for channel type")
	}
}

func TestTransportReadLineValid(t *testing.T) {
	tr := acp.NewTransport(strings.NewReader("{\"method\":\"test\"}\n"), nil)
	line, err := tr.ReadLine(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if string(line) != "{\"method\":\"test\"}" {
		t.Fatalf("unexpected line: %q", string(line))
	}
}

func TestTransportReadLineEmptyLine(t *testing.T) {
	tr := acp.NewTransport(strings.NewReader("\n"), nil)
	_, err := tr.ReadLine(context.Background())
	if err == nil || !strings.Contains(err.Error(), "empty ACP line") {
		t.Fatalf("expected empty line error, got %v", err)
	}
}

func TestTransportReadLineContextCancelled(t *testing.T) {
	tr := acp.NewTransport(&blockingReader{}, nil)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := tr.ReadLine(ctx)
	if err == nil {
		t.Fatal("expected context error")
	}
}

func TestTransportReadLineEOF(t *testing.T) {
	tr := acp.NewTransport(strings.NewReader(""), nil)
	_, err := tr.ReadLine(context.Background())
	if err == nil {
		t.Fatal("expected EOF error")
	}
}

func TestTransportReadWrite(t *testing.T) {
	in := bytes.NewBufferString("{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"initialize\"}\n")
	out := &bytes.Buffer{}
	tr := acp.NewTransport(in, out)
	line, err := tr.ReadLine(context.Background())
	if err != nil {
		t.Fatalf("ReadLine error: %v", err)
	}
	if len(line) == 0 {
		t.Fatal("expected non-empty line")
	}
	if err := tr.WriteMessage(acp.NewNotification("session/cancel", map[string]string{"sessionId": "s1"})); err != nil {
		t.Fatalf("WriteMessage error: %v", err)
	}
	if out.Len() == 0 {
		t.Fatal("expected output to be written")
	}
}

func TestTransportTimeout(t *testing.T) {
	in := bytes.NewBuffer(nil)
	out := &bytes.Buffer{}
	tr := acp.NewTransport(in, out)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	_, err := tr.ReadLine(ctx)
	if err == nil {
		t.Fatal("expected timeout")
	}
}
