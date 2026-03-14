package cli

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/one710/codeye/internal/agentregistry"
	"github.com/one710/codeye/internal/client"
	"github.com/one710/codeye/internal/config"
	cerr "github.com/one710/codeye/internal/errors"
	"github.com/one710/codeye/internal/output"
	"github.com/one710/codeye/internal/permissions"
	"github.com/one710/codeye/internal/session"
	"github.com/one710/codeye/internal/session/persistence"
	"github.com/one710/codeye/internal/version"
)

type globalFlags struct {
	Cwd          string
	AgentCommand string
	Format       output.Format
	PermMode     permissions.Mode
}

func Run(argv []string) int {
	if len(argv) > 1 && (argv[1] == "--version" || argv[1] == "-V") {
		fmt.Fprintln(os.Stdout, version.String())
		return 0
	}
	flags, rest, err := parseGlobals(argv[1:])
	if err != nil {
		format := detectRequestedFormat(argv[1:])
		output.Emitter{Format: format, Out: os.Stdout, Err: os.Stderr}.PrintRPCError(
			-32602,
			err.Error(),
			map[string]interface{}{"errorCode": cerr.CodeUsage},
		)
		return 2
	}
	cfg, err := config.Load(flags.Cwd)
	if err != nil {
		output.Emitter{Format: flags.Format, Out: os.Stdout, Err: os.Stderr}.PrintRPCError(
			-32603,
			err.Error(),
			map[string]interface{}{"errorCode": cerr.CodeRuntime},
		)
		return 1
	}
	if flags.Format == "" {
		flags.Format = output.Format(cfg.Format)
	}
	if flags.PermMode == "" {
		flags.PermMode = permissions.Mode(cfg.PermissionMode)
	}

	out := output.Emitter{Format: flags.Format, Out: os.Stdout, Err: os.Stderr}
	agent, cmd, cmdArgs := routeAgentAndCommand(rest, cfg.DefaultAgent)
	if cmd == "" {
		if flags.Format == output.JSONStrict {
			out.PrintRPCError(-32602, "command is required", map[string]interface{}{"errorCode": cerr.CodeUsage})
			return 2
		}
		printHelp()
		return 0
	}

	adapter, err := agentregistry.Resolve(agent, flags.AgentCommand, cfg.Agents)
	if err != nil {
		out.PrintRPCError(-32602, err.Error(), map[string]interface{}{"errorCode": cerr.CodeUsage})
		return 2
	}
	if err := agentregistry.ValidateInstalled(adapter); err != nil {
		out.PrintErrorWithCause(cerr.Wrap(cerr.CodeAgentUnavailable, "agent unavailable", err), err.Error())
		return cerr.ExitCode(cerr.Wrap(cerr.CodeAgentUnavailable, "agent unavailable", err))
	}

	root, err := session.DefaultRoot()
	if err != nil {
		out.PrintError(err.Error())
		return 1
	}
	repo := persistence.New(root)
	rt := session.NewRuntime(
		repo,
		socketPath(root, agent, flags.Cwd),
		time.Duration(cfg.QueueTTLSeconds)*time.Second,
		cfg.QueueMaxDepth,
		func() *client.Client {
			opts := client.Options{
				Command:           adapter.Command,
				Args:              adapter.Args,
				Cwd:               flags.Cwd,
				Mode:              flags.PermMode,
				AuthPolicy:        cfg.AuthPolicy,
				AuthCredentials:   cfg.Auth,
				InitializeTimeout: time.Duration(adapter.InitializeTO) * time.Second,
				NewSessionTimeout: time.Duration(adapter.NewSessionTO) * time.Second,
			}
			applySessionCallbacks(&opts, flags.Format, flags.PermMode)
			return client.New(opts)
		},
	)

	ctx := context.Background()
	code := dispatch(ctx, out, repo, rt, adapter, agent, flags.Cwd, cmd, cmdArgs, flags.PermMode, flags.Format)
	return code
}

func dispatch(
	ctx context.Context,
	out output.Emitter,
	repo *persistence.Repo,
	rt *session.Runtime,
	adapter agentregistry.Adapter,
	agent,
	cwd,
	cmd string,
	args []string,
	permMode permissions.Mode,
	format output.Format,
) int {
	switch cmd {
	case "prompt":
		if len(args) < 2 {
			out.PrintRPCError(-32602, "usage: prompt <session-id> <text...>", map[string]interface{}{"errorCode": cerr.CodeUsage})
			return 2
		}
		sessionID := args[0]
		promptText := strings.Join(args[1:], " ")
		rec, err := repo.Load(sessionID)
		if err != nil {
			out.PrintError("session not found: " + sessionID)
			return 1
		}
		if rec.Closed {
			out.PrintError("session closed: " + sessionID)
			return 1
		}
		// ACP: session/load (by agent) restores conversation history; then session/prompt. All in-process.
		stopReason, responseText, err := rt.PromptWithOutput(ctx, rec, promptText)
		if err != nil {
			out.PrintError(err.Error())
			return 1
		}
		// Do not print responseText again; OnSessionUpdate already streamed it.
		// Pass "" to suppress "prompt_completed" in text format (agent output already shown)
		out.PrintSuccess("prompt_completed", map[string]interface{}{
			"sessionId":  rec.RecordID,
			"stopReason": stopReason,
			"text":       strings.TrimSpace(responseText),
		}, "")
		return 0
	case "exec":
		if len(args) == 0 {
			out.PrintRPCError(-32602, "prompt is required", map[string]interface{}{"errorCode": cerr.CodeUsage})
			return 2
		}
		stopReason, responseText, err := rt.RunOnceWithOutput(ctx, cwd, strings.Join(args, " "))
		if err != nil {
			out.PrintError(err.Error())
			return 1
		}
		if out.Format == output.Text && strings.TrimSpace(responseText) != "" {
			fmt.Fprintln(out.Out, normalizeChunkedText(responseText))
		}
		out.PrintSuccess("exec_completed", map[string]interface{}{
			"agent":      agent,
			"stopReason": stopReason,
			"text":       strings.TrimSpace(responseText),
		}, "")
		return 0
	case "sessions":
		return dispatchSessions(ctx, out, rt, repo, agent, cwd, args, permMode, format)
	case "cancel":
		if len(args) < 1 {
			out.PrintRPCError(-32602, "usage: cancel <session-id>", map[string]interface{}{"errorCode": cerr.CodeUsage})
			return 2
		}
		rec, err := repo.Load(args[0])
		if err != nil {
			out.PrintError("session not found: " + args[0])
			return 1
		}
		if rec.Closed {
			out.PrintError("session closed: " + args[0])
			return 1
		}
		if err := rt.Cancel(ctx, rec); err != nil {
			out.PrintError(err.Error())
			return 1
		}
		out.PrintSuccess("cancel_result", map[string]interface{}{"cancelled": true}, "Session cancelled.")
		return 0
	case "set-mode":
		if len(args) != 2 {
			out.PrintRPCError(-32602, "usage: set-mode <session-id> <mode>", map[string]interface{}{"errorCode": cerr.CodeUsage})
			return 2
		}
		rec, err := repo.Load(args[0])
		if err != nil {
			out.PrintError("session not found: " + args[0])
			return 1
		}
		if rec.Closed {
			out.PrintError("session closed: " + args[0])
			return 1
		}
		if err := rt.SetMode(ctx, rec, args[1]); err != nil {
			out.PrintError(err.Error())
			return 1
		}
		out.PrintSuccess("mode_set", map[string]interface{}{"modeId": args[1]}, "Mode set to "+args[1]+".")
		return 0
	case "set":
		if len(args) != 3 {
			out.PrintRPCError(-32602, "usage: set <session-id> <key> <value>", map[string]interface{}{"errorCode": cerr.CodeUsage})
			return 2
		}
		rec, err := repo.Load(args[0])
		if err != nil {
			out.PrintError("session not found: " + args[0])
			return 1
		}
		if rec.Closed {
			out.PrintError("session closed: " + args[0])
			return 1
		}
		configKey := args[1]
		if compatKey, ok := adapter.ConfigKeyCompat[configKey]; ok {
			configKey = compatKey
		}
		if err := rt.SetConfigOption(ctx, rec, configKey, args[2]); err != nil {
			out.PrintError(err.Error())
			return 1
		}
		out.PrintSuccess("config_set", map[string]interface{}{"key": configKey, "value": args[2]}, "Set "+configKey+"="+args[2]+".")
		return 0
	case "status":
		if len(args) < 1 {
			out.PrintRPCError(-32602, "usage: status <session-id>", map[string]interface{}{"errorCode": cerr.CodeUsage})
			return 2
		}
		rec, err := repo.Load(args[0])
		if err != nil {
			out.PrintSuccess("status_snapshot", map[string]interface{}{"status": "not-found", "sessionId": args[0]}, "Session not found: "+args[0])
			return 0
		}
		if rec.Closed {
			out.PrintSuccess("status_snapshot", map[string]interface{}{"status": "closed", "sessionId": rec.RecordID}, "Session closed: "+rec.RecordID)
			return 0
		}
		out.PrintSuccess("status_snapshot", map[string]interface{}{
			"status":    "session",
			"sessionId": rec.RecordID,
		}, "Session: "+rec.RecordID)
		return 0
	case "config":
		cfg, err := config.Load(cwd)
		if err != nil {
			out.PrintError(err.Error())
			return 1
		}
		if out.Format == output.JSON || out.Format == output.JSONStrict {
			b, _ := json.Marshal(cfg)
			fmt.Fprintln(out.Out, string(b))
		} else {
			fmt.Fprintf(out.Out, "defaultAgent: %s\n", cfg.DefaultAgent)
		}
		return 0
	default:
		out.PrintRPCError(-32601, "unknown command: "+cmd, map[string]interface{}{"errorCode": cerr.CodeUsage})
		return 2
	}
}

func dispatchSessions(ctx context.Context, out output.Emitter, rt *session.Runtime, repo *persistence.Repo, agent, cwd string, args []string, permMode permissions.Mode, format output.Format) int {
	if len(args) == 0 || args[0] == "list" {
		records, err := repo.List()
		var local []persistence.Record
		var localIDs []string
		if err == nil {
			for _, r := range records {
				if !r.Closed {
					local = append(local, r)
					localIDs = append(localIDs, r.RecordID)
				}
			}
		}
		remote, remoteErr := rt.ListRemoteSessions(ctx, cwd)
		payload := map[string]interface{}{
			"sessions": localIDs,
		}
		if remoteErr == nil {
			payload["remoteSessions"] = remote
		}
		var listMsg string
		if len(local) == 0 && (remoteErr != nil || len(remote) == 0) {
			listMsg = "No sessions."
		} else {
			var buf bytes.Buffer
			tw := tabwriter.NewWriter(&buf, 0, 0, 2, ' ', 0)
			fmt.Fprintln(tw, "ID\tAGENT\tCWD")
			for _, r := range local {
				fmt.Fprintf(tw, "%s\t%s\t%s\n", r.RecordID, r.Agent, r.Cwd)
			}
			if remoteErr == nil {
				for _, id := range remote {
					fmt.Fprintf(tw, "%s\t-\t-\n", id)
				}
			}
			tw.Flush()
			listMsg = "Sessions:\n" + buf.String()
		}
		out.PrintSuccess("sessions_list", payload, listMsg)
		return 0
	}
	switch args[0] {
	case "new":
		rec, err := rt.CreateSession(ctx, agent, cwd, "")
		if err != nil {
			out.PrintError(err.Error())
			return 1
		}
		textOut := fmt.Sprintf("id: %s\nagent: %s\ncwd: %s", rec.RecordID, rec.Agent, rec.Cwd)
		out.PrintSuccess("session_ready", map[string]interface{}{
			"sessionId": rec.RecordID,
			"created":   true,
		}, textOut)
		return 0
	case "show":
		if len(args) < 2 {
			out.PrintRPCError(-32602, "usage: sessions show <session-id>", map[string]interface{}{"errorCode": cerr.CodeUsage})
			return 2
		}
		rec, err := repo.Load(args[1])
		if err != nil {
			out.PrintError("session not found: " + args[1])
			return 1
		}
		if out.Format == output.JSON || out.Format == output.JSONStrict {
			b, _ := json.Marshal(rec)
			fmt.Fprintln(out.Out, string(b))
		} else {
			fmt.Fprintf(out.Out, "id: %s\nagent: %s\ncwd: %s\n", rec.RecordID, rec.Agent, rec.Cwd)
		}
		return 0
	case "close":
		if len(args) < 2 {
			out.PrintRPCError(-32602, "usage: sessions close <session-id>", map[string]interface{}{"errorCode": cerr.CodeUsage})
			return 2
		}
		rec, err := repo.Load(args[1])
		if err != nil {
			out.PrintError("session not found: " + args[1])
			return 1
		}
		rec.Closed = true
		if err := repo.Save(rec); err != nil {
			out.PrintError(err.Error())
			return 1
		}
		out.PrintSuccess("session_closed", map[string]interface{}{"sessionId": rec.RecordID}, "Session closed.")
		return 0
	default:
		out.PrintError("unknown sessions command")
		return 2
	}
}

func routeAgentAndCommand(tokens []string, defaultAgent string) (string, string, []string) {
	if len(tokens) == 0 {
		return defaultAgent, "", nil
	}
	verbs := map[string]bool{
		"prompt": true, "exec": true, "cancel": true, "set-mode": true,
		"set": true, "sessions": true, "status": true, "config": true,
	}
	agent := defaultAgent
	if !verbs[tokens[0]] {
		agent = tokens[0]
		tokens = tokens[1:]
	}
	if len(tokens) == 0 {
		return agent, "prompt", nil
	}
	return agent, tokens[0], tokens[1:]
}

func parseGlobals(args []string) (globalFlags, []string, error) {
	cwd, _ := os.Getwd()
	flags := globalFlags{Cwd: cwd}
	rest := []string{}
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if !strings.HasPrefix(arg, "-") {
			rest = append(rest, args[i:]...)
			break
		}
		switch arg {
		case "--cwd":
			i++
			flags.Cwd = args[i]
		case "--agent":
			i++
			flags.AgentCommand = args[i]
		case "--format":
			i++
			flags.Format = output.Format(args[i])
		case "--json-strict":
			flags.Format = output.JSONStrict
		case "--approve-all":
			flags.PermMode = permissions.ApproveAll
		case "--approve-reads":
			flags.PermMode = permissions.ApproveReads
		case "--deny-all":
			flags.PermMode = permissions.DenyAll
		case "--ask":
			flags.PermMode = permissions.Ask
		default:
			if strings.HasPrefix(arg, "--") {
				return flags, nil, fmt.Errorf("unknown flag: %s", arg)
			}
		}
	}
	flags.Cwd = filepath.Clean(flags.Cwd)
	return flags, rest, nil
}

func socketPath(root, agent, cwd string) string {
	h := sha1.Sum([]byte(agent + "|" + cwd))
	return filepath.Join(root, "queue-"+hex.EncodeToString(h[:8])+".sock")
}

// applySessionCallbacks sets OnSessionUpdate, OnPermissionDenied, and optionally OnPermissionRequest
// so that exec and the working session (prompt mode) share the same behavior.
func applySessionCallbacks(opts *client.Options, format output.Format, permMode permissions.Mode) {
	opts.OnSessionUpdate = func(raw json.RawMessage) {
		if format == output.JSON || format == output.JSONStrict {
			fmt.Fprintln(os.Stdout, string(raw))
			return
		}
		if format == output.Text {
			renderSessionUpdateText(raw, os.Stdout)
		}
	}
	opts.OnPermissionDenied = func(method string) {
		fmt.Fprintf(os.Stderr, "codeye: permission denied for %s (use --approve-all or --ask to allow)\n", method)
	}
	if permMode == permissions.Ask {
		opts.OnPermissionRequest = interactivePermissionPrompter(os.Stderr, os.Stdin, os.Stdout)
	}
}

func printHelp() {
	fmt.Fprintln(os.Stdout, "codeye <agent?> <command> [args]")
	fmt.Fprintln(os.Stdout, "Commands: prompt, exec, sessions, cancel, set-mode, set, status, config")
	fmt.Fprintln(os.Stdout, "Examples:")
	fmt.Fprintln(os.Stdout, "  codeye cursor sessions new")
	fmt.Fprintln(os.Stdout, "  codeye cursor prompt <session-id> \"fix tests\"")
}

func detectRequestedFormat(args []string) output.Format {
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--json-strict":
			return output.JSONStrict
		case "--format":
			if i+1 < len(args) {
				switch output.Format(args[i+1]) {
				case output.JSON, output.JSONStrict, output.Quiet, output.Text:
					return output.Format(args[i+1])
				}
				i++
			}
		}
	}
	return output.Text
}

func renderSessionUpdateText(raw json.RawMessage, out io.Writer) {
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
	case "assistant_message", "assistant_message_chunk", "agent_message", "agent_message_chunk", "":
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
	rawText := text
	if strings.TrimSpace(rawText) == "" {
		return
	}
	if strings.Contains(event, "_chunk") {
		fmt.Fprint(out, rawText)
		return
	}
	fmt.Fprintln(out, normalizeChunkedText(rawText))
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

func normalizeChunkedText(text string) string {
	lines := strings.Split(text, "\n")
	tokens := make([]string, 0, len(lines))
	for _, line := range lines {
		t := strings.TrimSpace(line)
		if t == "" {
			continue
		}
		tokens = append(tokens, t)
	}
	if len(tokens) == 0 {
		return strings.TrimSpace(text)
	}
	var b strings.Builder
	for i, tok := range tokens {
		if i == 0 {
			b.WriteString(tok)
			continue
		}
		prev := b.String()
		if shouldAttachWithoutSpace(tok) || endsWithOpenPunctuation(prev) {
			b.WriteString(tok)
		} else if len(lines) > 1 {
			// Preserve line breaks when input had multiple lines (e.g. code blocks).
			b.WriteString("\n")
			b.WriteString(tok)
		} else {
			b.WriteString(" ")
			b.WriteString(tok)
		}
	}
	return strings.TrimSpace(b.String())
}

func shouldAttachWithoutSpace(token string) bool {
	switch token {
	case ".", ",", "!", "?", ";", ":", ")", "]", "}":
		return true
	}
	if strings.HasPrefix(token, "'") || strings.HasPrefix(token, "’") {
		return true
	}
	return false
}

func endsWithOpenPunctuation(s string) bool {
	if s == "" {
		return false
	}
	last := s[len(s)-1]
	return last == '(' || last == '[' || last == '{'
}

// interactivePermissionPrompter returns a callback that prompts on stderr and reads from stdin.
// If stdout is provided, it is flushed before prompting so streaming output is visible first.
func interactivePermissionPrompter(stderr io.Writer, stdin io.Reader, stdout io.Writer) func(client.PermissionRequest) string {
	order := []struct {
		kind  string
		label string
	}{
		{"allow_once", "Allow once"},
		{"allow_always", "Allow always"},
		{"reject_once", "Reject once"},
		{"reject_always", "Reject always"},
	}
	return func(preq client.PermissionRequest) string {
		if stdout != nil {
			if f, ok := stdout.(*os.File); ok {
				_ = f.Sync()
			}
		}
		if f, ok := stderr.(*os.File); ok {
			_ = f.Sync()
		}
		fmt.Fprintln(stderr)
		fmt.Fprintf(stderr, "Tool: %s\n", preq.Method)
		var byKind map[string]string
		if len(preq.Options) > 0 {
			byKind = make(map[string]string)
			for _, o := range preq.Options {
				if o.Kind != "" && o.OptionID != "" {
					byKind[o.Kind] = o.OptionID
				}
			}
		}
		num := 0
		for _, x := range order {
			if _, ok := byKind[x.kind]; ok {
				num++
				fmt.Fprintf(stderr, " [%d] %s\n", num, x.label)
			}
		}
		fmt.Fprintf(stderr, " [c] Cancel\nChoice: ")
		scanner := bufio.NewScanner(stdin)
		if !scanner.Scan() {
			return ""
		}
		line := strings.TrimSpace(strings.ToLower(scanner.Text()))
		if line == "" || line == "c" {
			return ""
		}
		n, err := strconv.Atoi(line)
		if err != nil || n < 1 {
			return ""
		}
		idx := 0
		for _, x := range order {
			if byKind[x.kind] != "" {
				idx++
				if idx == n {
					return byKind[x.kind]
				}
			}
		}
		return ""
	}
}
