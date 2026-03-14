<div align="center">

# codeye

**Your AI coding agents, one CLI away.**

Drive Cursor and any ACP-compatible agent from your terminal — with persistent sessions and full automation support.

[![Go](https://img.shields.io/badge/Go-1.24+-00ADD8?style=flat&logo=go&logoColor=white)](https://go.dev)
[![License](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](LICENSE)
[![CI](https://github.com/one710/codeye/actions/workflows/ci.yml/badge.svg)](https://github.com/one710/codeye/actions/workflows/ci.yml)

</div>

---

## What is codeye?

**codeye** is a production-grade Go CLI that speaks the [Agent Client Protocol (ACP)](https://github.com/anthropics/agent-client-protocol) — the emerging open standard for communicating with AI coding agents.

Instead of juggling multiple GUIs, browser tabs, and agent-specific interfaces, you get **one unified command line** to talk to any ACP-compatible backend. Send prompts, manage sessions, control permissions, pipe output to your scripts — all without leaving the terminal.

```bash
codeye cursor exec "find the bug in auth.go and write a regression test"
```

That's it. One command. For persistent sessions, use `sessions new` then `prompt <session-id> <text>`. Your agent reads the repo, reasons about the code, and delivers results — all streamed back to your terminal.

## Why codeye?

- **Agent-agnostic.** Use Cursor (built-in) or any custom ACP adapter with a flag. No lock-in.
- **Persistent sessions.** Session identity survives across commands. Each run uses ACP `session/load` (agent restores history) then `session/prompt` in-process — no separate long-lived process required when the agent supports session restore.
- **Automation-first.** JSON and strict-JSON output modes, non-interactive permission policies, and structured errors make codeye a first-class citizen in pipelines and orchestration.
- **Permission-gated tools.** Agents request filesystem reads, writes, and terminal commands — codeye enforces your permission policy before anything touches disk.
- **Zero dependencies.** Pure Go, single binary, no runtime requirements. Build it, drop it anywhere.

## Quick Start

### Install

```bash
go install github.com/one710/codeye/cmd/codeye@latest
```

Or build from source:

```bash
git clone https://github.com/one710/codeye.git
cd codeye
go build -o ./bin/codeye ./cmd/codeye
```

### Your first session

```bash
# Create a session (prints session ID)
codeye cursor sessions new

# Send a prompt (use the session ID from above)
codeye cursor prompt <session-id> "review the last commit and suggest improvements"

# Check session status
codeye cursor status <session-id>
```

### One-shot mode

Don't need persistence? Use `exec` for fire-and-forget tasks:

```bash
codeye cursor exec "summarize this repository in three bullet points"
```

### Pick your agent

```bash
codeye cursor prompt <session-id> "fix the failing tests"

# One-shot (no session): use exec
codeye cursor exec "explain the main function"

# Or bring your own ACP-compatible adapter
codeye --agent "my-custom-agent --stdio" prompt <session-id> "review security posture"
```

## Commands

| Command                          | Description                                         |
| -------------------------------- | --------------------------------------------------- |
| `prompt <session-id> <text>`     | Send a prompt to an existing session                |
| `exec <text>`                    | One-shot prompt (ephemeral session, no persistence) |
| `cancel <session-id>`            | Cooperatively cancel an in-flight prompt            |
| `set-mode <session-id> <mode>`   | Set the session mode (e.g., `plan`, `code`)         |
| `set <session-id> <key> <value>` | Set a config option on the session                  |
| `status <session-id>`            | Show session status                                 |
| `config`                         | Display resolved configuration                      |
| `sessions list`                  | List local and remote sessions                      |
| `sessions new`                   | Create a new session (prints session ID)            |
| `sessions show <session-id>`     | Show session metadata                               |
| `sessions close <session-id>`    | Mark the session closed                             |

### Global Options

| Flag              | Description                                                 |
| ----------------- | ----------------------------------------------------------- |
| `--cwd <path>`    | Run in a specific working directory                         |
| `--agent "<cmd>"` | Use a custom ACP-compatible agent command                   |
| `--format <mode>` | Output format: `text`, `json`, `json-strict`, `quiet`       |
| `--json-strict`   | Shorthand for `--format json-strict`                        |
| `--approve-all`   | Allow all agent tool requests                               |
| `--approve-reads` | Allow read-only tool requests, deny writes                  |
| `--deny-all`      | Deny all agent tool requests                                |
| `--ask`           | Prompt to approve or reject each tool request (interactive) |
| `--version`, `-V` | Print version                                               |

## Configuration

codeye loads config from two sources, with project-local values taking precedence. Set `permissionMode` to `"ask"` to be prompted for each tool call (allow once/always, reject once/always, or cancel).

| Source  | Path                              |
| ------- | --------------------------------- |
| Global  | `~/.codeye/config.json`           |
| Project | `.codeye.json` (repo root or cwd) |

Example configuration:

```json
{
  "defaultAgent": "cursor",
  "format": "text",
  "permissionMode": "approve-reads",
  "agents": {
    "cursor": { "command": "agent", "args": ["acp"] }
  }
}
```

## Automation & CI

codeye is designed for headless, scriptable workflows:

```bash
# One-shot with strict JSON output
codeye --format json-strict --approve-all cursor exec "apply the patch and run tests"

# Use in a CI pipeline
codeye --deny-all cursor exec "check for security vulnerabilities" --format json
```

Errors in JSON modes are emitted as structured JSON-RPC error objects, making them safe to parse and act on programmatically.

## Agent Skill

codeye ships with a `SKILL.md` that any AI agent can read to learn how to install and use the tool. This means your agents can bootstrap other agents — give one agent the skill, and it can orchestrate coding work across multiple backends.

See [`skills/codeye/SKILL.md`](skills/codeye/SKILL.md) for the full agent-facing reference.

## Development

### Prerequisites

- Go 1.24+

### Build & Test

```bash
make build       # Compile all packages
make test        # Run the test suite
make race        # Run tests with race detector
make check       # Format, vet, and test in one shot
```

### Project Structure

```
cmd/codeye/          CLI entrypoint
internal/
  acp/               ACP protocol types, transport, payloads
  client/            Core ACP client and agent lifecycle
  cli/               Command routing and flag parsing
  config/            Layered config loading
  agentregistry/     Agent command resolution and adapters
  errors/            Structured error types and exit codes
  output/            Output formatting (text, JSON, quiet)
  permissions/       Permission policy engine
  queue/             Queue IPC and server (optional; session runtime)
  session/           Session persistence and runtime
  tools/             Filesystem and terminal tool handlers
  version/           Version metadata
skills/codeye/       Agent skill reference (SKILL.md)
```

### Contributing

Contributions are welcome! Please open an issue or pull request. When submitting changes:

1. Run `make check` to ensure formatting, vetting, and tests pass.
2. Add or update tests for any new functionality.
3. Keep commits focused — one logical change per commit.

## License

Licensed under the [Apache License, Version 2.0](LICENSE).

```
Copyright 2025-2026 one710

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
```
