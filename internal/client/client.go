package client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/one710/codeye/internal/acp"
	"github.com/one710/codeye/internal/permissions"
	fstool "github.com/one710/codeye/internal/tools/fs"
	"github.com/one710/codeye/internal/tools/terminal"
)

type Options struct {
	Command              string
	Args                 []string
	Cwd                  string
	Mode                 permissions.Mode
	NonInteractivePolicy permissions.NonInteractive
	AuthPolicy           string
	AuthCredentials      map[string]string
	InitializeTimeout    time.Duration
	NewSessionTimeout    time.Duration
	OnSessionUpdate      func(json.RawMessage)
	OnPermissionRequest  func(PermissionRequest) (selectedOptionID string)
	OnPermissionDenied   func(method string)
}

// PermissionRequest is the payload for an interactive permission prompt.
type PermissionRequest struct {
	Method  string
	Options []PermissionOption
}

// PermissionOption is one choice (e.g. allow_once, reject_once) with its agent-provided optionId.
type PermissionOption struct {
	OptionID string
	Kind     string
}

type Client struct {
	opts      Options
	cmd       *exec.Cmd
	transport *acp.Transport
	fs        *fstool.Handler
	term      *terminal.Manager

	capabilities agentCapabilities
	authMethods  []acp.AuthMethod

	idSeq    uint64
	pending  map[string]chan acp.Response
	pendingM sync.Mutex
	writeMu  sync.Mutex

	updateMu                sync.Mutex
	observedSessionUpdates  uint64
	processedSessionUpdates uint64
	suppressSessionUpdates  bool
	activePromptSessionID   string
	activePromptChunks      []string

	stop    chan struct{}
	stopped chan struct{}
}

type agentCapabilities struct {
	ListSession     bool
	LoadSession     bool
	SetMode         bool
	SetConfigOption bool
}

type PromptResult struct {
	StopReason string
	Text       string
}

func New(opts Options) *Client {
	return &Client{
		opts:    opts,
		pending: map[string]chan acp.Response{},
		stop:    make(chan struct{}),
		stopped: make(chan struct{}),
	}
}

func (c *Client) Start(ctx context.Context) error {
	if c.opts.Command == "" {
		return fmt.Errorf("agent command is required")
	}
	c.fs = fstool.New(filepath.Clean(c.opts.Cwd))
	c.term = terminal.New()
	c.cmd = exec.CommandContext(ctx, c.opts.Command, c.opts.Args...)
	c.cmd.Dir = c.opts.Cwd
	c.cmd.Env = buildAgentEnv(c.opts.AuthCredentials)
	stdin, err := c.cmd.StdinPipe()
	if err != nil {
		return err
	}
	stdout, err := c.cmd.StdoutPipe()
	if err != nil {
		return err
	}
	if err := c.cmd.Start(); err != nil {
		return err
	}
	c.transport = acp.NewTransport(stdout, stdin)
	go c.readLoop()

	timeout := c.opts.InitializeTimeout
	if timeout <= 0 {
		timeout = 15 * time.Second
	}
	initCtx, cancelInit := context.WithTimeout(ctx, timeout)
	defer cancelInit()
	initResultRaw, err := c.call(initCtx, acp.MethodInitialize, acp.InitializeRequest{
		ProtocolVersion: acp.ProtocolVersion,
		ClientInfo: map[string]interface{}{
			"name":    "codeye",
			"version": "0.1.0",
		},
		ClientCaps: map[string]interface{}{
			"fs": map[string]bool{
				"readTextFile":  true,
				"writeTextFile": true,
			},
			"terminal": true,
		},
	})
	if err != nil {
		return err
	}
	var initResult acp.InitializeResult
	if err := decodeResult(initResultRaw, &initResult); err != nil {
		return err
	}
	if initResult.ProtocolVersion != acp.ProtocolVersion {
		return fmt.Errorf("unsupported protocol version: agent=%d client=%d", initResult.ProtocolVersion, acp.ProtocolVersion)
	}
	c.capabilities = parseCapabilitiesMap(toMap(initResult.AgentCapabilities))
	c.authMethods = initResult.AuthMethods
	if err := c.authenticateIfRequired(initCtx); err != nil {
		return err
	}
	return nil
}

func (c *Client) Close() error {
	close(c.stop)
	select {
	case <-c.stopped:
	case <-time.After(2 * time.Second):
	}
	if c.cmd != nil && c.cmd.Process != nil {
		_ = c.cmd.Process.Kill()
	}
	return nil
}

func (c *Client) SetOnSessionUpdate(fn func(json.RawMessage)) {
	c.updateMu.Lock()
	c.opts.OnSessionUpdate = fn
	c.updateMu.Unlock()
}

func (c *Client) CreateSession(ctx context.Context, cwd string) (string, error) {
	timeout := c.opts.NewSessionTimeout
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	createCtx, cancelCreate := context.WithTimeout(ctx, timeout)
	defer cancelCreate()
	resRaw, err := c.call(createCtx, acp.MethodSessionNew, acp.SessionNewRequest{
		Cwd:        filepath.Clean(cwd),
		MCPServers: []interface{}{},
	})
	if err != nil {
		return "", err
	}
	var res acp.SessionNewResponse
	if err := decodeResult(resRaw, &res); err != nil {
		return "", err
	}
	if res.SessionID == "" {
		return "", fmt.Errorf("session/new returned empty sessionId")
	}
	return res.SessionID, nil
}

func (c *Client) LoadSession(ctx context.Context, sessionID, cwd string) error {
	if !c.capabilities.LoadSession {
		return errors.New("agent does not advertise session/load capability")
	}
	c.updateMu.Lock()
	c.suppressSessionUpdates = true
	c.updateMu.Unlock()
	defer func() {
		c.updateMu.Lock()
		c.suppressSessionUpdates = false
		c.updateMu.Unlock()
	}()

	_, err := c.call(ctx, acp.MethodSessionLoad, acp.SessionLoadRequest{
		SessionID: sessionID,
		Cwd:       filepath.Clean(cwd),
	})
	if err != nil {
		return err
	}
	return c.waitForReplayDrain(80*time.Millisecond, 5*time.Second)
}

func (c *Client) ListSessions(ctx context.Context, cwd string) ([]string, error) {
	if !c.capabilities.ListSession {
		return nil, errors.New("agent does not advertise session/list capability")
	}
	resRaw, err := c.call(ctx, acp.MethodSessionList, acp.SessionListRequest{
		Cwd: filepath.Clean(cwd),
	})
	if err != nil {
		return nil, err
	}
	var res acp.SessionListResponse
	if err := decodeResult(resRaw, &res); err != nil {
		return nil, err
	}
	out := make([]string, 0, len(res.Sessions))
	for _, entry := range res.Sessions {
		if strings.TrimSpace(entry.SessionID) != "" {
			out = append(out, entry.SessionID)
		}
	}
	return out, nil
}

func (c *Client) Prompt(ctx context.Context, sessionID string, parts []acp.PromptPart) (PromptResult, error) {
	if len(parts) == 0 {
		parts = []acp.PromptPart{{Type: "text", Text: ""}}
	}
	c.updateMu.Lock()
	c.activePromptSessionID = sessionID
	c.activePromptChunks = nil
	c.updateMu.Unlock()
	defer func() {
		c.updateMu.Lock()
		c.activePromptSessionID = ""
		c.activePromptChunks = nil
		c.updateMu.Unlock()
	}()

	res, err := c.call(ctx, acp.MethodSessionPrompt, acp.SessionPromptRequest{
		SessionID: sessionID,
		Prompt:    parts,
	})
	if err != nil {
		return PromptResult{}, err
	}
	var promptResp acp.SessionPromptResponse
	if err := decodeResult(res, &promptResp); err != nil {
		return PromptResult{}, err
	}
	if promptResp.StopReason == "" {
		promptResp.StopReason = "end_turn"
	}
	textOut := extractPromptText(res)
	if textOut == "" {
		c.updateMu.Lock()
		textOut = strings.TrimSpace(strings.Join(c.activePromptChunks, ""))
		c.updateMu.Unlock()
	}
	return PromptResult{
		StopReason: promptResp.StopReason,
		Text:       textOut,
	}, nil
}

func (c *Client) Cancel(ctx context.Context, sessionID string) error {
	req := acp.NewNotification(acp.MethodSessionCancel, map[string]string{"sessionId": sessionID})
	return c.transport.WriteMessage(req)
}

func (c *Client) SetMode(ctx context.Context, sessionID, mode string) error {
	if !c.capabilities.SetMode {
		return errors.New("agent does not advertise session/set_mode capability")
	}
	_, err := c.call(ctx, acp.MethodSessionSetMode, acp.SessionSetModeRequest{
		SessionID: sessionID,
		ModeID:    mode,
	})
	return err
}

func (c *Client) SetConfigOption(ctx context.Context, sessionID, key, value string) error {
	if !c.capabilities.SetConfigOption {
		return errors.New("agent does not advertise session/set_config_option capability")
	}
	_, err := c.call(ctx, acp.MethodSessionSetConfig, acp.SessionSetConfigOptionRequest{
		SessionID: sessionID,
		ConfigID:  key,
		Value:     value,
	})
	return err
}

func (c *Client) call(ctx context.Context, method string, params interface{}) (map[string]interface{}, error) {
	id := fmt.Sprintf("%d", atomic.AddUint64(&c.idSeq, 1))
	ch := make(chan acp.Response, 1)
	c.pendingM.Lock()
	c.pending[id] = ch
	c.pendingM.Unlock()

	if err := c.transport.WriteMessage(acp.NewRequest(id, method, params)); err != nil {
		return nil, err
	}
	select {
	case <-ctx.Done():
		c.pendingM.Lock()
		delete(c.pending, id)
		c.pendingM.Unlock()
		return nil, ctx.Err()
	case resp := <-ch:
		if resp.Error != nil {
			return nil, fmt.Errorf("acp error (%d): %s", resp.Error.Code, resp.Error.Message)
		}
		out := map[string]interface{}{}
		if resp.Result == nil {
			return out, nil
		}
		b, _ := json.Marshal(resp.Result)
		_ = json.Unmarshal(b, &out)
		return out, nil
	}
}

func (c *Client) readLoop() {
	defer close(c.stopped)
	for {
		select {
		case <-c.stop:
			return
		default:
		}
		line, err := c.transport.ReadLine(context.Background())
		if err != nil {
			return
		}
		req, resp, isReq, err := acp.DecodeMessage(line)
		if err != nil {
			continue
		}
		if isReq {
			c.handleInboundRequest(req)
			continue
		}
		id := fmt.Sprintf("%v", resp.ID)
		c.pendingM.Lock()
		ch := c.pending[id]
		delete(c.pending, id)
		c.pendingM.Unlock()
		if ch != nil {
			ch <- resp
		}
	}
}

func (c *Client) handleInboundRequest(req acp.Request) {
	switch req.Method {
	case acp.MethodSessionUpdate:
		c.updateMu.Lock()
		c.observedSessionUpdates++
		updateText := extractSessionUpdateText(req.Params)
		if c.activePromptSessionID != "" && updateText != "" {
			c.activePromptChunks = append(c.activePromptChunks, updateText)
		}
		suppress := c.suppressSessionUpdates
		c.updateMu.Unlock()
		if c.opts.OnSessionUpdate != nil {
			if !suppress {
				b, _ := json.Marshal(req.Params)
				c.opts.OnSessionUpdate(json.RawMessage(b))
			}
		}
		c.updateMu.Lock()
		c.processedSessionUpdates = c.observedSessionUpdates
		c.updateMu.Unlock()
	case acp.MethodSessionRequestPerm:
		if c.opts.Mode == permissions.Ask && c.opts.OnPermissionRequest != nil {
			// Defer prompt so any agent chunks already in the stream are processed and shown first.
			reqID := req.ID
			params := req.Params
			go func() {
				time.Sleep(1200 * time.Millisecond)
				preq := parsePermissionRequest(params)
				selected := c.opts.OnPermissionRequest(preq)
				var result map[string]interface{}
				if selected != "" {
					result = map[string]interface{}{
						"outcome": map[string]interface{}{"outcome": "selected", "optionId": selected},
					}
				} else {
					result = map[string]interface{}{
						"outcome": map[string]interface{}{"outcome": string(permissions.DecisionCancelled)},
					}
				}
				if c.opts.OnPermissionDenied != nil && isPermissionDenied(params, result) {
					c.opts.OnPermissionDenied(nestedMethod(params))
				}
				c.writeMu.Lock()
				_ = c.transport.WriteMessage(acp.Response{JSONRPC: "2.0", ID: reqID, Result: result})
				c.writeMu.Unlock()
			}()
			return
		}
		var result map[string]interface{}
		result = permissionDecision(req.Params, c.opts.Mode)
		if c.opts.OnPermissionDenied != nil && isPermissionDenied(req.Params, result) {
			c.opts.OnPermissionDenied(nestedMethod(req.Params))
		}
		c.writeMu.Lock()
		_ = c.transport.WriteMessage(acp.Response{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  result,
		})
		c.writeMu.Unlock()
	case acp.MethodFSReadTextFile:
		var params acp.FSReadTextFileRequest
		_ = decodeAny(req.Params, &params)
		content, err := c.fs.ReadTextFile(params.Path)
		if err != nil {
			c.replyErr(req.ID, -32602, err.Error())
			return
		}
		c.writeMu.Lock()
		_ = c.transport.WriteMessage(acp.Response{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  acp.FSReadTextFileResponse{Content: content},
		})
		c.writeMu.Unlock()
	case acp.MethodFSWriteTextFile:
		var params acp.FSWriteTextFileRequest
		_ = decodeAny(req.Params, &params)
		if err := c.fs.WriteTextFile(params.Path, params.Content); err != nil {
			c.replyErr(req.ID, -32602, err.Error())
			return
		}
		c.writeMu.Lock()
		_ = c.transport.WriteMessage(acp.Response{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  map[string]any{},
		})
		c.writeMu.Unlock()
	case acp.MethodTerminalCreate:
		var params acp.TerminalCreateRequest
		_ = decodeAny(req.Params, &params)
		id, err := c.term.Create(params.Command, params.Args...)
		if err != nil {
			c.replyErr(req.ID, -32603, err.Error())
			return
		}
		c.writeMu.Lock()
		_ = c.transport.WriteMessage(acp.Response{JSONRPC: "2.0", ID: req.ID, Result: acp.TerminalCreateResponse{ID: id}})
		c.writeMu.Unlock()
	case acp.MethodTerminalOutput:
		var params acp.TerminalOutputRequest
		_ = decodeAny(req.Params, &params)
		out, err := c.term.Output(params.ID)
		if err != nil {
			c.replyErr(req.ID, -32602, err.Error())
			return
		}
		c.writeMu.Lock()
		_ = c.transport.WriteMessage(acp.Response{JSONRPC: "2.0", ID: req.ID, Result: acp.TerminalOutputResponse{Output: out}})
		c.writeMu.Unlock()
	case acp.MethodTerminalWaitForExit:
		var params acp.TerminalOutputRequest
		_ = decodeAny(req.Params, &params)
		waitResult, err := c.term.Wait(params.ID)
		result := acp.TerminalWaitForExitResponse{ExitCode: waitResult.ExitCode, Signal: waitResult.Signal}
		if err != nil {
			result.Error = err.Error()
		}
		c.writeMu.Lock()
		_ = c.transport.WriteMessage(acp.Response{JSONRPC: "2.0", ID: req.ID, Result: result})
		c.writeMu.Unlock()
	case acp.MethodTerminalKill:
		params := toMap(req.Params)
		id := int(toFloat(params["id"]))
		err := c.term.Kill(id)
		if err != nil {
			c.replyErr(req.ID, -32602, err.Error())
			return
		}
		c.writeMu.Lock()
		_ = c.transport.WriteMessage(acp.Response{JSONRPC: "2.0", ID: req.ID, Result: map[string]bool{"killed": true}})
		c.writeMu.Unlock()
	case acp.MethodTerminalRelease:
		params := toMap(req.Params)
		id := int(toFloat(params["id"]))
		c.term.Release(id)
		c.writeMu.Lock()
		_ = c.transport.WriteMessage(acp.Response{JSONRPC: "2.0", ID: req.ID, Result: map[string]bool{"released": true}})
		c.writeMu.Unlock()
	default:
		// Unknown request methods are method-not-found.
		c.replyErr(req.ID, -32601, "method not found")
	}
}

func (c *Client) replyErr(id any, code int, message string) {
	c.writeMu.Lock()
	_ = c.transport.WriteMessage(acp.Response{
		JSONRPC: "2.0",
		ID:      id,
		Error:   &acp.Error{Code: code, Message: message},
	})
	c.writeMu.Unlock()
}

func toMap(v interface{}) map[string]interface{} {
	if m, ok := v.(map[string]interface{}); ok {
		return m
	}
	b, _ := json.Marshal(v)
	out := map[string]interface{}{}
	_ = json.Unmarshal(b, &out)
	return out
}

func toFloat(v interface{}) float64 {
	switch t := v.(type) {
	case float64:
		return t
	case int:
		return float64(t)
	default:
		return 0
	}
}

func nestedMethod(params interface{}) string {
	m := toMap(params)
	if m == nil {
		return ""
	}
	tryTool := func(tool map[string]interface{}) string {
		if tool == nil {
			return ""
		}
		if s, _ := tool["method"].(string); strings.TrimSpace(s) != "" {
			return strings.TrimSpace(s)
		}
		if s, _ := tool["title"].(string); strings.TrimSpace(s) != "" {
			return strings.TrimSpace(s)
		}
		if raw, _ := tool["rawInput"].(map[string]interface{}); raw != nil {
			for _, key := range []string{"method", "tool", "toolName", "tool_name", "name"} {
				if s, _ := raw[key].(string); strings.TrimSpace(s) != "" {
					return strings.TrimSpace(s)
				}
			}
		}
		return ""
	}
	if tool, _ := m["toolCall"].(map[string]interface{}); tool != nil {
		if s := tryTool(tool); s != "" {
			return s
		}
	}
	if s, ok := m["method"].(string); ok && strings.TrimSpace(s) != "" {
		return strings.TrimSpace(s)
	}
	if update, _ := m["update"].(map[string]interface{}); update != nil {
		if tool, _ := update["toolCall"].(map[string]interface{}); tool != nil {
			if s := tryTool(tool); s != "" {
				return s
			}
		}
	}
	return ""
}

func parsePermissionRequest(params interface{}) PermissionRequest {
	out := PermissionRequest{Method: nestedMethod(params)}
	m := toMap(params)
	raw, _ := m["options"].([]interface{})
	for _, r := range raw {
		opt, _ := r.(map[string]interface{})
		if opt == nil {
			continue
		}
		id, _ := opt["optionId"].(string)
		kind, _ := opt["kind"].(string)
		out.Options = append(out.Options, PermissionOption{OptionID: strings.TrimSpace(id), Kind: strings.TrimSpace(kind)})
	}
	return out
}

func isPermissionDenied(params interface{}, result map[string]interface{}) bool {
	outcome := toMap(result["outcome"])
	if outcome == nil {
		return false
	}
	o, _ := outcome["outcome"].(string)
	if o == string(permissions.DecisionCancelled) {
		return true
	}
	if o != "selected" {
		return false
	}
	optionID, _ := outcome["optionId"].(string)
	if optionID == "" {
		return false
	}
	m := toMap(params)
	raw, _ := m["options"].([]interface{})
	for _, r := range raw {
		opt, _ := r.(map[string]interface{})
		if opt == nil {
			continue
		}
		id, _ := opt["optionId"].(string)
		kind, _ := opt["kind"].(string)
		if strings.TrimSpace(id) == optionID {
			k := strings.TrimSpace(kind)
			return k == "reject_once" || k == "reject_always"
		}
	}
	return false
}

func (c *Client) waitForReplayDrain(idleFor time.Duration, timeout time.Duration) error {
	if idleFor < 0 {
		idleFor = 0
	}
	if timeout < idleFor {
		timeout = idleFor
	}

	deadline := time.Now().Add(timeout)
	lastObserved := uint64(0)
	idleSince := time.Now()
	for time.Now().Before(deadline) {
		c.updateMu.Lock()
		observed := c.observedSessionUpdates
		processed := c.processedSessionUpdates
		c.updateMu.Unlock()

		if observed != lastObserved {
			lastObserved = observed
			idleSince = time.Now()
		}
		if processed == observed && time.Since(idleSince) >= idleFor {
			return nil
		}
		time.Sleep(20 * time.Millisecond)
	}
	return errors.New("timed out waiting for session replay drain")
}

func parseCapabilitiesMap(rawCaps map[string]interface{}) agentCapabilities {
	caps := agentCapabilities{}
	if rawCaps == nil {
		return caps
	}
	// Optional methods must be gated by explicit capability.
	caps.ListSession = boolFromMap(rawCaps, "listSessions")
	caps.LoadSession = boolFromMap(rawCaps, "loadSession")
	caps.SetMode = boolFromMap(rawCaps, "setMode")
	caps.SetConfigOption = boolFromMap(rawCaps, "setConfigOption")

	// Compatibility with sessionCapabilities.list shape.
	if !caps.ListSession {
		if sc, ok := rawCaps["sessionCapabilities"].(map[string]interface{}); ok {
			caps.ListSession = boolFromMap(sc, "list")
		}
	}
	return caps
}

func decodeResult(input map[string]interface{}, out interface{}) error {
	b, err := json.Marshal(input)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, out)
}

func decodeAny(input interface{}, out interface{}) error {
	b, err := json.Marshal(input)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, out)
}

func (c *Client) authenticateIfRequired(ctx context.Context) error {
	if len(c.authMethods) == 0 {
		return nil
	}
	methodID, ok := c.selectAuthMethod()
	if !ok {
		if strings.EqualFold(c.opts.AuthPolicy, "fail") {
			ids := make([]string, 0, len(c.authMethods))
			for _, m := range c.authMethods {
				ids = append(ids, m.ID)
			}
			return fmt.Errorf("agent advertised auth methods [%s] but no credentials found", strings.Join(ids, ", "))
		}
		return nil
	}
	_, err := c.call(ctx, acp.MethodAuthenticate, acp.AuthenticateRequest{MethodID: methodID})
	return err
}

func (c *Client) selectAuthMethod() (string, bool) {
	for _, method := range c.authMethods {
		if credentialForMethod(method.ID, c.opts.AuthCredentials) != "" {
			return method.ID, true
		}
	}
	return "", false
}

func credentialForMethod(methodID string, configured map[string]string) string {
	if v := strings.TrimSpace(envGet(methodID)); v != "" {
		return v
	}
	token := normalizeMethodToken(methodID)
	if token != "" {
		if v := strings.TrimSpace(envGet(token)); v != "" {
			return v
		}
		if v := strings.TrimSpace(envGet("CODEYE_AUTH_" + token)); v != "" {
			return v
		}
	}
	if configured != nil {
		if v := strings.TrimSpace(configured[methodID]); v != "" {
			return v
		}
	}
	if token == "" {
		return ""
	}
	if configured != nil {
		if v := strings.TrimSpace(configured[token]); v != "" {
			return v
		}
		if v := strings.TrimSpace(configured["CODEYE_AUTH_"+token]); v != "" {
			return v
		}
	}
	return ""
}

func normalizeMethodToken(methodID string) string {
	var b strings.Builder
	for _, r := range strings.ToUpper(strings.TrimSpace(methodID)) {
		if (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		} else {
			b.WriteRune('_')
		}
	}
	return strings.Trim(b.String(), "_")
}

var envGet = func(key string) string { return strings.TrimSpace(os.Getenv(key)) }

func buildAgentEnv(auth map[string]string) []string {
	env := os.Environ()
	if len(auth) == 0 {
		return env
	}
	for key, value := range auth {
		v := strings.TrimSpace(value)
		if v == "" {
			continue
		}
		if strings.TrimSpace(key) != "" && !strings.Contains(key, "=") {
			env = append(env, key+"="+v)
		}
		token := normalizeMethodToken(key)
		if token != "" {
			env = append(env, "CODEYE_AUTH_"+token+"="+v)
			env = append(env, token+"="+v)
		}
	}
	return env
}

func boolFromMap(m map[string]interface{}, key string) bool {
	v, ok := m[key]
	if !ok {
		return false
	}
	b, ok := v.(bool)
	return ok && b
}

func permissionDecision(params interface{}, mode permissions.Mode) map[string]interface{} {
	m := toMap(params)
	options, _ := m["options"].([]interface{})
	selectByKind := func(kinds ...string) string {
		for _, kind := range kinds {
			for _, raw := range options {
				opt, ok := raw.(map[string]interface{})
				if !ok {
					continue
				}
				k, _ := opt["kind"].(string)
				if k != kind {
					continue
				}
				if id, ok := opt["optionId"].(string); ok && strings.TrimSpace(id) != "" {
					return id
				}
			}
		}
		return ""
	}
	selectFirst := func() string {
		for _, raw := range options {
			opt, ok := raw.(map[string]interface{})
			if !ok {
				continue
			}
			if id, ok := opt["optionId"].(string); ok && strings.TrimSpace(id) != "" {
				return id
			}
		}
		return ""
	}
	cancelled := map[string]interface{}{
		"outcome": map[string]interface{}{"outcome": string(permissions.DecisionCancelled)},
	}
	selected := func(optionID string) map[string]interface{} {
		return map[string]interface{}{
			"outcome": map[string]interface{}{"outcome": "selected", "optionId": optionID},
		}
	}

	allowID := selectByKind("allow_once", "allow_always")
	rejectID := selectByKind("reject_once", "reject_always")

	switch mode {
	case permissions.Ask:
		return cancelled
	case permissions.ApproveAll:
		if allowID != "" {
			return selected(allowID)
		}
		if first := selectFirst(); first != "" {
			return selected(first)
		}
		return cancelled
	case permissions.DenyAll:
		if rejectID != "" {
			return selected(rejectID)
		}
		return cancelled
	default: // approve-reads
		method := nestedMethod(params)
		if permissions.IsReadOnly(method) && allowID != "" {
			return selected(allowID)
		}
		if rejectID != "" {
			return selected(rejectID)
		}
		return cancelled
	}
}

func extractPromptText(res map[string]interface{}) string {
	if res == nil {
		return ""
	}
	// Common direct shapes.
	if s, ok := res["text"].(string); ok {
		return strings.TrimSpace(s)
	}
	if s, ok := res["content"].(string); ok {
		return strings.TrimSpace(s)
	}
	// Common nested shape: { message: { text/content/... } }
	if msg, ok := res["message"].(map[string]interface{}); ok {
		if s, ok := msg["text"].(string); ok && strings.TrimSpace(s) != "" {
			return strings.TrimSpace(s)
		}
		if s, ok := msg["content"].(string); ok && strings.TrimSpace(s) != "" {
			return strings.TrimSpace(s)
		}
		if parts := extractTextParts(msg["content"]); parts != "" {
			return parts
		}
	}
	// ACP-like content arrays.
	if parts := extractTextParts(res["content"]); parts != "" {
		return parts
	}
	if parts := extractTextParts(res["output"]); parts != "" {
		return parts
	}
	return ""
}

func extractTextParts(v interface{}) string {
	items, ok := v.([]interface{})
	if !ok {
		return ""
	}
	var parts []string
	for _, item := range items {
		entry, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		if text, ok := entry["text"].(string); ok && strings.TrimSpace(text) != "" {
			parts = append(parts, strings.TrimSpace(text))
			continue
		}
		if content, ok := entry["content"].(string); ok && strings.TrimSpace(content) != "" {
			parts = append(parts, strings.TrimSpace(content))
			continue
		}
	}
	return strings.TrimSpace(strings.Join(parts, "\n"))
}

func extractSessionUpdateText(params interface{}) string {
	m := toMap(params)
	if len(m) == 0 {
		return ""
	}
	update := toMap(m["update"])
	if len(update) == 0 {
		update = m
	}
	if s, ok := update["text"].(string); ok && strings.TrimSpace(s) != "" {
		return s
	}
	if s, ok := update["delta"].(string); ok && strings.TrimSpace(s) != "" {
		return s
	}
	if s, ok := update["content"].(string); ok && strings.TrimSpace(s) != "" {
		return s
	}
	if contentMap, ok := update["content"].(map[string]interface{}); ok {
		if s, ok := contentMap["text"].(string); ok && strings.TrimSpace(s) != "" {
			return s
		}
	}
	if msg, ok := update["message"].(map[string]interface{}); ok {
		if s, ok := msg["text"].(string); ok && strings.TrimSpace(s) != "" {
			return s
		}
		if parts := extractTextParts(msg["content"]); parts != "" {
			return parts
		}
	}
	if parts := extractTextParts(update["content"]); parts != "" {
		return parts
	}
	return ""
}
