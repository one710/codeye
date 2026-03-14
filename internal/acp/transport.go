package acp

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"sync"
)

type Transport struct {
	r  *bufio.Reader
	w  io.Writer
	mu sync.Mutex
}

func NewTransport(stdin io.Reader, stdout io.Writer) *Transport {
	return &Transport{
		r: bufio.NewReader(stdin),
		w: stdout,
	}
}

func (t *Transport) ReadLine(ctx context.Context) ([]byte, error) {
	type result struct {
		line string
		err  error
	}
	ch := make(chan result, 1)
	go func() {
		line, err := t.r.ReadString('\n')
		ch <- result{line: line, err: err}
	}()
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case out := <-ch:
		if out.err != nil {
			return nil, out.err
		}
		line := strings.TrimSpace(out.line)
		if line == "" {
			return nil, errors.New("empty ACP line")
		}
		return []byte(line), nil
	}
}

func (t *Transport) WriteMessage(v interface{}) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	if _, err := t.w.Write(append(b, '\n')); err != nil {
		return err
	}
	return nil
}
