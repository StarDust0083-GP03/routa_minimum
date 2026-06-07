# AGENTS.md

This project is a Go-based coding agent that combines the coding capabilities from pi with context management from beads.

## Project Structure

```
cmd/codeg/          - Main entry point
internal/
  agent/            - Agent core logic
  llm/              - LLM providers (OpenAI, Anthropic, Google)
  tools/            - Code tools (read, write, edit, bash, grep, find, ls)
  beads/            - Task/dependency management
  session/          - Session management
  compaction/       - Context compression
  cli/              - Interactive CLI
  config/           - Configuration
```

## Development

```bash
go run cmd/codeg/main.go           # Run the agent
go build cmd/codeg                  # Build binary
go test ./...                       # Run tests
go vet ./...                        # Run linter
```

## Configuration

- API keys via environment variables: `OPENAI_API_KEY`, `ANTHROPIC_API_KEY`, etc.
- Config file: `~/.codeg/config.json`

## Key Concepts

- **Tasks**: Managed via beads-style Issue/Dependency system
- **Sessions**: Persistent conversation history with session management
- **Compaction**: Context compression to manage token limits
- **Tools**: Read, write, edit, bash, grep, find, ls

## Status

This project is in early planning phase. See TODO.md for detailed implementation tasks.
