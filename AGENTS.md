# codeg — TUI Coding Agent Manager

A terminal-based multi-agent task manager. Uses opencode as the coding agent backend, with LLM-assisted task decomposition and two-phase (coding + verification) execution.

## Project Structure

```
cmd/codeg/          - Main entry point (TUI)
internal/
  acp/              - ACP protocol types and SSE event parsing
  agent/
    core/           - AgentController: sub-task lifecycle management
    opencode/       - opencode CLI subprocess runner
    acp_manager.go  - ACP server process management (reserved)
    acp_session.go  - ACP session wrapper (reserved)
    runner.go       - AgentRunner interface
  config/           - Configuration loading + env var overrides
  llm/              - OpenAI-compatible API client (DeepSeek, etc.)
  log/              - Debug logging
  session/          - SQLite session store
  task/             - Task + SubTask models, storage, planning, judging
  tui/              - Bubble Tea terminal UI
    components/     - UI components (panels, forms, lists)
```

## Quick Start

```bash
export OPENAI_API_KEY="sk-your-key"
export OPENAI_BASE_URL="https://api.deepseek.com"  # or OpenAI
export CODEG_LLM_MODEL="deepseek-chat"

go run ./cmd/codeg/
```

With debug logging:

```bash
go run ./cmd/codeg/ --debug
# Logs: ~/.codeg/codeg.log
```

## Configuration

| Env Var | Default | Description |
|---------|---------|-------------|
| `OPENAI_API_KEY` | - | LLM API key for planning + judging |
| `OPENAI_BASE_URL` | `https://api.openai.com` | API base URL (DeepSeek, etc.) |
| `CODEG_LLM_MODEL` | `gpt-4o-mini` | Model for planning and judging |
| `CODEG_DB_PATH` | `~/.codeg/codeg.db` | SQLite database path |
| `CODEG_ACP_COMMAND` | `opencode acp` | ACP server command |
| `CODEG_ACP_PORT_FLAG` | `--port` | Port flag for ACP server |

Config file: `~/.codeg/config.json`

## TUI Keybindings

| Key | Action |
|-----|--------|
| `n` | Create new task |
| `b` | Bind working directory to task |
| `p` | Plan mode — LLM decomposes task into sub-tasks |
| `r` | Run coding agent on selected sub-task |
| `f` | Force verification on selected sub-task |
| `c` | Cancel running agent |
| `d` | Delete task |
| `↑/↓` | Navigate tasks / sub-tasks |
| `Tab` | Switch focus panel |
| `1/2/3` | Focus panel directly |
| `q` | Quit |

## Architecture

### Two-Phase Execution

```
Task
  └─ SubTask 1: planned → coding → verifying → done
  └─ SubTask 2: planned → coding → verifying → done
  └─ SubTask 3: planned → stuck (retry with 'r')
```

### Data Flow

```
User creates task → SQLite
User presses 'p' → opencode plans → sub-tasks stored in SQLite
User presses 'r' → opencode runs as subprocess → events stream to TUI
Coding completes → OpenAI judges completion → auto-start verification
Verification completes → OpenAI judges result → mark done/failed/stuck
All sub-tasks done → OpenAI summarizes → task completed
```

### OpenAI API Calls (2)

1. **Plan mode fallback**: `POST /v1/chat/completions` — when ACP planning fails
2. **Completion judge**: `POST /v1/chat/completions` — judges coding completion and verification results

### ACP Calls

Uses `opencode run --dir <cwd> --format json <prompt>` subprocess. Parses JSON lines from stdout. Event types: `step_start`, `text`, `tool_use`, `step_finish`, `error`.

## Development

```bash
go build ./cmd/codeg/          # Build
go test -short ./...           # Unit tests (175)
go test -run Integration ./...  # Integration tests (require opencode)
go vet ./...                   # Lint
```

## Status

v2 complete with:
- Sub-task architecture with two-phase execution
- LLM-assisted task decomposition (ACP primary, OpenAI fallback)
- OpenAI-based completion and verification judging
- Streaming agent output in TUI
- Per-subtask output persistence
- Debug logging (`--debug` flag)
