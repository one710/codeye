package session

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/one710/codeye/internal/client"
	"github.com/one710/codeye/internal/queue"
	"github.com/one710/codeye/internal/session/persistence"
)

type Runtime struct {
	ClientFactory func() *client.Client
	Repo          *persistence.Repo
	SocketPath    string
	TTL           time.Duration
	MaxDepth      int
}

func NewRuntime(repo *persistence.Repo, socketPath string, ttl time.Duration, maxDepth int, cf func() *client.Client) *Runtime {
	return &Runtime{
		ClientFactory: cf,
		Repo:          repo,
		SocketPath:    socketPath,
		TTL:           ttl,
		MaxDepth:      maxDepth,
	}
}

// CreateSession always creates a new session and returns the record (RecordID = session ID for user).
func (r *Runtime) CreateSession(ctx context.Context, agent, cwd, name string) (persistence.Record, error) {
	c := r.ClientFactory()
	if err := c.Start(ctx); err != nil {
		return persistence.Record{}, err
	}
	defer c.Close()
	sid, err := c.CreateSession(ctx, cwd)
	if err != nil {
		return persistence.Record{}, err
	}
	rec := persistence.Record{
		RecordID:   randomID(),
		ACPSession: sid,
		Agent:      agent,
		Cwd:        cwd,
		Name:       name,
		Closed:     false,
	}
	if err := r.Repo.Save(rec); err != nil {
		return persistence.Record{}, err
	}
	return rec, nil
}

func (r *Runtime) Prompt(ctx context.Context, rec persistence.Record, prompt string) (string, error) {
	stopReason, _, err := r.PromptWithOutput(ctx, rec, prompt)
	return stopReason, err
}

func (r *Runtime) PromptWithOutput(ctx context.Context, rec persistence.Record, prompt string) (string, string, error) {
	return r.PromptInProcess(ctx, rec, prompt)
}

// PromptInProcess runs the prompt in the current process with the same client as exec,
// so streaming and permission prompts behave identically to exec.
func (r *Runtime) PromptInProcess(ctx context.Context, rec persistence.Record, prompt string) (string, string, error) {
	c := r.ClientFactory()
	if err := c.Start(ctx); err != nil {
		return "", "", err
	}
	defer c.Close()
	sid := rec.ACPSession
	if err := c.LoadSession(ctx, sid, rec.Cwd); err != nil {
		newSid, createErr := c.CreateSession(ctx, rec.Cwd)
		if createErr != nil {
			return "", "", createErr
		}
		sid = newSid
		rec.ACPSession = newSid
		_ = r.Repo.Save(rec)
	}
	result, err := c.Prompt(ctx, sid, prompt)
	if err != nil {
		return "", "", err
	}
	stopReason := result.StopReason
	if stopReason == "" {
		stopReason = "end_turn"
	}
	return stopReason, result.Text, nil
}

func (r *Runtime) Cancel(ctx context.Context, rec persistence.Record) error {
	return r.withSession(ctx, rec, func(c *client.Client, sid string) error {
		return c.Cancel(ctx, sid)
	})
}

func (r *Runtime) SetMode(ctx context.Context, rec persistence.Record, mode string) error {
	return r.withSession(ctx, rec, func(c *client.Client, sid string) error {
		return c.SetMode(ctx, sid, mode)
	})
}

func (r *Runtime) SetConfigOption(ctx context.Context, rec persistence.Record, key, value string) error {
	return r.withSession(ctx, rec, func(c *client.Client, sid string) error {
		return c.SetConfigOption(ctx, sid, key, value)
	})
}

// withSession starts the client, loads or creates the session, runs fn, then closes. Used by Cancel, SetMode, SetConfigOption.
func (r *Runtime) withSession(ctx context.Context, rec persistence.Record, fn func(*client.Client, string) error) error {
	c := r.ClientFactory()
	if err := c.Start(ctx); err != nil {
		return err
	}
	defer c.Close()
	sid := rec.ACPSession
	if err := c.LoadSession(ctx, sid, rec.Cwd); err != nil {
		newSid, createErr := c.CreateSession(ctx, rec.Cwd)
		if createErr != nil {
			return createErr
		}
		sid = newSid
		rec.ACPSession = newSid
		_ = r.Repo.Save(rec)
	}
	return fn(c, sid)
}

func (r *Runtime) RunWorkingSession(ctx context.Context, initial persistence.Record) error {
	lease := queue.NewLeaseStore(r.SocketPath + ".lease")
	if err := lease.Acquire(os.Getpid()); err != nil {
		return err
	}
	defer func() { _ = lease.Release() }()

	heartbeatCtx, cancelHeartbeat := context.WithCancel(ctx)
	defer cancelHeartbeat()
	go func() {
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-heartbeatCtx.Done():
				return
			case <-ticker.C:
				_ = lease.Refresh()
			}
		}
	}()

	c := r.ClientFactory()
	if err := c.Start(ctx); err != nil {
		return err
	}
	defer c.Close()
	liveSessionID := initial.ACPSession
	if err := c.LoadSession(ctx, initial.ACPSession, initial.Cwd); err != nil {
		// Some agents do not support session/load. Fallback to creating a
		// live working session so prompt/cancel/mode/config commands still work.
		sid, createErr := c.CreateSession(ctx, initial.Cwd)
		if createErr != nil {
			return createErr
		}
		liveSessionID = sid
	}
	if liveSessionID != initial.ACPSession {
		initial.ACPSession = liveSessionID
		_ = r.Repo.Save(initial)
	}
	handler := &workingSessionHandler{
		client:        c,
		liveSessionID: liveSessionID,
		cwd:           initial.Cwd,
		repo:          r.Repo,
		record:        initial,
	}
	c.SetOnSessionUpdate(handler.captureSessionUpdate)
	server := &queue.Server{
		SocketPath: r.SocketPath,
		TTL:        r.TTL,
		MaxDepth:   r.MaxDepth,
		Handler:    handler,
	}
	return server.Run(ctx)
}

func (r *Runtime) RunOnce(ctx context.Context, cwd, prompt string) (string, error) {
	stopReason, _, err := r.RunOnceWithOutput(ctx, cwd, prompt)
	return stopReason, err
}

func (r *Runtime) RunOnceWithOutput(ctx context.Context, cwd, prompt string) (string, string, error) {
	c := r.ClientFactory()
	if err := c.Start(ctx); err != nil {
		return "", "", err
	}
	defer c.Close()
	sid, err := c.CreateSession(ctx, cwd)
	if err != nil {
		return "", "", err
	}
	result, err := c.Prompt(ctx, sid, prompt)
	if err != nil {
		return "", "", err
	}
	return result.StopReason, result.Text, nil
}

func (r *Runtime) ListRemoteSessions(ctx context.Context, cwd string) ([]string, error) {
	c := r.ClientFactory()
	if err := c.Start(ctx); err != nil {
		return nil, err
	}
	defer c.Close()
	return c.ListSessions(ctx, cwd)
}

func DefaultRoot() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	root := filepath.Join(home, ".codeye")
	return root, os.MkdirAll(root, 0o755)
}

func randomID() string {
	buf := make([]byte, 8)
	_, _ = rand.Read(buf)
	return hex.EncodeToString(buf)
}

type workingSessionHandler struct {
	client        *client.Client
	liveSessionID string
	cwd           string
	repo          *persistence.Repo
	record        persistence.Record
	mu            sync.Mutex
	chunks        []string
	fullMessage   string // set when we get a non-chunk final message to avoid duplicating chunk content
}

func (w *workingSessionHandler) Prompt(ctx context.Context, sessionID, prompt string) (queue.PromptResult, error) {
	w.mu.Lock()
	activeSessionID := w.liveSessionID
	w.chunks = nil
	w.fullMessage = ""
	w.mu.Unlock()

	result, err := w.client.Prompt(ctx, activeSessionID, prompt)
	if err != nil && shouldRecreateSessionOnPromptError(err) {
		// Agent rejected stale/unknown session; recreate once and retry.
		if sid, createErr := w.client.CreateSession(ctx, w.cwd); createErr == nil {
			w.mu.Lock()
			w.liveSessionID = sid
			w.record.ACPSession = sid
			if w.repo != nil {
				_ = w.repo.Save(w.record)
			}
			w.mu.Unlock()
			result, err = w.client.Prompt(ctx, sid, prompt)
		}
	}
	if err != nil {
		return queue.PromptResult{}, err
	}
	text := strings.TrimSpace(result.Text)
	if text == "" {
		w.mu.Lock()
		if w.fullMessage != "" {
			text = strings.TrimSpace(w.fullMessage)
		} else {
			text = strings.TrimSpace(joinChunksWithNewlines(w.chunks))
		}
		w.mu.Unlock()
	}
	return queue.PromptResult{StopReason: result.StopReason, Text: text}, nil
}

func (w *workingSessionHandler) Cancel(ctx context.Context, sessionID string) error {
	w.mu.Lock()
	activeSessionID := w.liveSessionID
	w.mu.Unlock()
	return w.client.Cancel(ctx, activeSessionID)
}

func (w *workingSessionHandler) SetMode(ctx context.Context, sessionID, mode string) error {
	w.mu.Lock()
	activeSessionID := w.liveSessionID
	w.mu.Unlock()
	return w.client.SetMode(ctx, activeSessionID, mode)
}

func (w *workingSessionHandler) SetConfigOption(ctx context.Context, sessionID, key, value string) error {
	w.mu.Lock()
	activeSessionID := w.liveSessionID
	w.mu.Unlock()
	return w.client.SetConfigOption(ctx, activeSessionID, key, value)
}

func shouldRecreateSessionOnPromptError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "Resource not found") || strings.Contains(msg, "(-32002)")
}

func (w *workingSessionHandler) captureSessionUpdate(raw json.RawMessage) {
	var payload map[string]interface{}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return
	}
	update := asMap(payload["update"])
	if update == nil {
		update = payload
	}
	event, _ := update["event"].(string)
	if strings.TrimSpace(event) == "" {
		event, _ = update["sessionUpdate"].(string)
	}
	switch event {
	case "assistant_message", "assistant_message_chunk", "agent_message", "agent_message_chunk":
	default:
		return
	}
	text := firstString(update, "text", "delta", "content")
	if text == "" {
		if content := asMap(update["content"]); content != nil {
			text = firstString(content, "text", "content")
		}
	}
	if text == "" {
		if message := asMap(update["message"]); message != nil {
			text = firstString(message, "text", "content")
		}
	}
	if strings.TrimSpace(text) == "" {
		return
	}
	w.mu.Lock()
	if strings.Contains(event, "_chunk") {
		w.chunks = append(w.chunks, text)
	} else {
		// Full message (not a chunk): use it alone to avoid duplicating chunk content
		w.fullMessage = text
	}
	w.mu.Unlock()
}

// joinChunksWithNewlines joins chunks, inserting newlines where the next chunk starts a new line (e.g. shebang, "echo").
func joinChunksWithNewlines(chunks []string) string {
	if len(chunks) == 0 {
		return ""
	}
	if len(chunks) == 1 {
		return chunks[0]
	}
	var b strings.Builder
	b.WriteString(chunks[0])
	for i := 1; i < len(chunks); i++ {
		s := chunks[i]
		prev := chunks[i-1]
		// Insert newline if next chunk looks like start of new line (not continuation of sentence)
		needNewline := len(s) > 0 && !startsWithContinuation(s) && !endsWithContinuation(prev)
		if needNewline {
			b.WriteString("\n")
		}
		b.WriteString(s)
	}
	return b.String()
}

func startsWithContinuation(s string) bool {
	if s == "" {
		return true
	}
	switch s[0] {
	case ' ', '\t', ',', '.', '!', '?', ';', ':', ')', ']', '}', '"', '\'':
		return true
	}
	return false
}

func endsWithContinuation(s string) bool {
	if s == "" {
		return true
	}
	switch s[len(s)-1] {
	case ' ', '\t', '(', '[', '{', '"', '\'':
		return true
	}
	return false
}

func asMap(v interface{}) map[string]interface{} {
	m, _ := v.(map[string]interface{})
	return m
}

func firstString(m map[string]interface{}, keys ...string) string {
	for _, key := range keys {
		if s, ok := m[key].(string); ok && strings.TrimSpace(s) != "" {
			return s
		}
	}
	return ""
}
