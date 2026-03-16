# codeye Skill

Use `codeye` to run ACP-compatible coding agents (Cursor or custom adapters) from CLI with persistent sessions.

## Install

```bash
go install github.com/one710/codeye/cmd/codeye@latest
```

Or from local source:

```bash
go build -o /usr/local/bin/codeye ./cmd/codeye
```

## Quick start

```bash
codeye cursor sessions new                    # Creates session, prints session ID
codeye cursor prompt <session-id> "review this repository and suggest fixes"
codeye cursor status <session-id>
```

## CLI shape

`codeye` supports:

```bash
codeye [global-options] [agent] <command> [command-args]
```

- `agent` is optional and defaults to configured `defaultAgent` (usually `cursor`).
- Built-in agent: `cursor`.
- Any custom ACP command can be used with `--agent`.

### Commands

- `prompt <session-id> <text...>`: send prompt to an existing session.
- `exec <text...>`: one-shot prompt (creates ephemeral session, runs, exits; no persistence).
- `cancel <session-id>`: cooperative cancel for in-flight prompt.
- `set-mode <session-id> <mode>`: set session mode (capability-gated by agent).
- `set <session-id> <key> <value>`: set session config option (capability-gated by agent).
- `status <session-id>`: session status (`session`, `closed`, or `not-found`).
- `config`: show resolved config.
- `sessions`:
  - `sessions` or `sessions list`
  - `sessions new`
  - `sessions show <session-id>`
  - `sessions close <session-id>`

### Global options

- `--cwd <path>`: run in specific working directory.
- `--agent "<command ...>"`: custom ACP-compatible adapter command.
- `--format <text|json|json-strict|quiet>`: output format.
- `--json-strict`: alias for strict JSON output mode.
- `--audio <path>`: add audio file to prompt/exec (repeatable; .wav, .mp3, .ogg, .flac, .m4a).
- `--image <path>`: add image file to prompt/exec (repeatable; .png, .jpg, .gif, .webp).
- `--approve-all`: allow all ACP tool requests.
- `--approve-reads`: allow read-only tool requests, deny writes.
- `--deny-all`: deny all ACP tool requests.
- `--version`, `-V`: print codeye version.

## Agent selection

- `codeye cursor ...` uses Cursor ACP adapter command from config.
- `codeye --agent "<custom-acp-command>" ...` uses any ACP-compatible command.

## Permissions and output modes

- `--approve-all` allow all ACP tool requests.
- `--approve-reads` allow read-only tool requests and deny writes.
- `--deny-all` deny all tool requests.
- `--format json` machine-readable output events.
- `--format json-strict` JSON-only output (safe for orchestrators).

## Session workflow

```bash
SID=$(codeye cursor sessions new | jq -r .sessionId)   # or parse from output
codeye cursor prompt $SID "implement a failing test fix"
codeye cursor set-mode $SID plan
codeye cursor set $SID reasoning_effort high
codeye cursor cancel $SID
codeye cursor sessions close $SID
```

## Session management examples

```bash
# list local session ids and remote sessions (if agent supports session/list)
codeye cursor sessions list --format json

# show session metadata
codeye cursor sessions show <session-id>

# close session
codeye cursor sessions close <session-id>
```

## One-shot workflow (`exec`)

```bash
codeye cursor exec "summarize this repository"
codeye --agent "my-acp-adapter --stdio" exec "run a quick code review"
```

## Image and audio in prompt/exec

Place `--image` and `--audio` before the command. You can pass one or many files; they are sent as content blocks with the text prompt.

**One image with text (exec):**

```bash
codeye --image screenshot.png exec "what is shown in this screenshot? list the main UI elements"
```

**One image with text (prompt):**

```bash
SID=$(codeye cursor sessions new | jq -r .sessionId)
codeye --image diagram.png cursor prompt $SID "explain this diagram and suggest improvements"
```

**Multiple images with text:**

```bash
codeye --image before.png --image after.png exec "compare these two screenshots and describe what changed"
codeye --image fig1.png --image fig2.png cursor prompt $SID "summarize the flow in these two figures"
```

**Audio with text (e.g. transcription or analysis):**

```bash
codeye --audio meeting.wav exec "transcribe this and list action items"
codeye --audio intro.mp3 --audio outro.mp3 cursor prompt $SID "do these two clips sound consistent in tone?"
```

**Image and audio together:**

```bash
codeye --image slide.png --audio narration.wav exec "align this slide with the narration and suggest edits"
```

Supported image extensions: `.png`, `.jpg`, `.jpeg`, `.gif`, `.webp`. Audio: `.wav`, `.mp3`, `.ogg`, `.flac`, `.m4a`. The agent must advertise the corresponding prompt capabilities (image/audio) in initialization.

## Config file

Global config path: `~/.codeye/config.json`

```json
{
  "defaultAgent": "cursor",
  "format": "text",
  "permissionMode": "approve-reads",
  "nonInteractivePermissions": "deny",
  "queueTTLSeconds": 60,
  "queueMaxDepth": 32,
  "authPolicy": "skip",
  "auth": {},
  "agents": {
    "cursor": { "command": "agent", "args": ["acp"] }
  }
}
```

Each agent can optionally set `initializeTimeout` and `newSessionTimeout` (seconds). Project-local override file: `.codeye.json` in repo root or current working directory.

## Non-interactive automation pattern

```bash
SID=$(codeye --format json cursor sessions new | jq -r .sessionId)
codeye --format json-strict --approve-all cursor prompt $SID "apply the patch and run tests"
codeye cursor sessions close $SID
```

## Output and error behavior

- `--format json` and `--format json-strict` emit machine-readable JSON events.
- Errors in JSON modes are emitted as JSON-RPC style error objects.
- `json-strict` suppresses non-JSON human help output and is best for orchestration.

## ACP session flow

- **`sessions new`** creates a new session and prints its session ID. All session-scoped commands (`prompt`, `cancel`, `set-mode`, `set`, `status`, `sessions show`, `sessions close`) require that session ID.
- **`exec`** is one-shot: creates ephemeral session, runs prompt, exits. No persistence, no session ID needed.
- Session-based flow is **exec broken down**: `sessions new` → `prompt` → … → `sessions close`.

## Notes for coding agents

- Use `sessions new` to create a session, then pass the session ID to `prompt`, `cancel`, `set-mode`, `set`, `status`, `sessions show`, `sessions close`.
- Prefer `exec` for stateless single-turn tasks (no session management).
