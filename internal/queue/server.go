package queue

import (
	"bufio"
	"context"
	"encoding/json"
	"net"
	"os"
	"sync"
	"time"

	"github.com/one710/codeye/internal/acp"
)

type Handler interface {
	Prompt(ctx context.Context, sessionID string, parts []acp.PromptPart) (PromptResult, error)
	Cancel(ctx context.Context, sessionID string) error
	SetMode(ctx context.Context, sessionID, mode string) error
	SetConfigOption(ctx context.Context, sessionID, key, value string) error
}

type Server struct {
	SocketPath string
	TTL        time.Duration
	Handler    Handler
	MaxDepth   int
}

func (s *Server) Run(ctx context.Context) error {
	_ = os.Remove(s.SocketPath)
	ln, err := net.Listen("unix", s.SocketPath)
	if err != nil {
		return err
	}
	defer ln.Close()
	defer os.Remove(s.SocketPath)

	var mu sync.Mutex
	depth := 0
	last := time.Now()

	go func() {
		t := time.NewTicker(500 * time.Millisecond)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				_ = ln.Close()
				return
			case <-t.C:
				if s.TTL > 0 && time.Since(last) > s.TTL {
					_ = ln.Close()
					return
				}
			}
		}
	}()

	for {
		conn, err := ln.Accept()
		if err != nil {
			return nil
		}
		last = time.Now()
		go func() {
			defer conn.Close()
			req, err := readReq(conn)
			if err != nil {
				writeResp(conn, Response{
					RequestID:  req.RequestID,
					OK:         false,
					Message:    err.Error(),
					Code:       "QUEUE_PROTOCOL",
					DetailCode: "INVALID_REQUEST",
					Origin:     "working_session",
				})
				return
			}

			mu.Lock()
			if s.MaxDepth > 0 && depth >= s.MaxDepth {
				mu.Unlock()
				writeResp(conn, Response{
					RequestID:  req.RequestID,
					OK:         false,
					Message:    "queue overload",
					Code:       "QUEUE_ERROR",
					DetailCode: "WORKING_SESSION_OVERLOADED",
					Origin:     "working_session",
					Retryable:  true,
				})
				return
			}
			depth++
			mu.Unlock()
			defer func() {
				mu.Lock()
				depth--
				mu.Unlock()
			}()

			cmdCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
			defer cancel()
			resp, err := s.dispatch(cmdCtx, req)
			if err != nil {
				writeResp(conn, Response{
					RequestID:  req.RequestID,
					OK:         false,
					Message:    err.Error(),
					Code:       "QUEUE_RUNTIME",
					DetailCode: "REQUEST_FAILED",
					Origin:     "working_session",
				})
				return
			}
			resp.OK = true
			resp.PID = os.Getpid()
			resp.RequestID = req.RequestID
			writeResp(conn, resp)
		}()
	}
}

func (s *Server) dispatch(ctx context.Context, req Request) (Response, error) {
	switch req.Command {
	case CmdPrompt:
		parts := req.PromptParts
		if len(parts) == 0 {
			parts = []acp.PromptPart{{Type: "text", Text: req.Prompt}}
		}
		result, err := s.Handler.Prompt(ctx, req.SessionID, parts)
		if err != nil {
			return Response{}, err
		}
		return Response{StopReason: result.StopReason, Text: result.Text}, nil
	case CmdCancel:
		return Response{}, s.Handler.Cancel(ctx, req.SessionID)
	case CmdSetMode:
		return Response{}, s.Handler.SetMode(ctx, req.SessionID, req.Mode)
	case CmdSetConfigOption:
		return Response{}, s.Handler.SetConfigOption(ctx, req.SessionID, req.Key, req.Value)
	case CmdHealth:
		return Response{}, nil
	default:
		return Response{}, os.ErrInvalid
	}
}

func readReq(conn net.Conn) (Request, error) {
	line, err := bufio.NewReader(conn).ReadBytes('\n')
	if err != nil {
		return Request{}, err
	}
	var req Request
	if err := json.Unmarshal(line, &req); err != nil {
		return Request{}, err
	}
	return req, nil
}

func writeResp(conn net.Conn, resp Response) {
	b, _ := json.Marshal(resp)
	_, _ = conn.Write(append(b, '\n'))
}
