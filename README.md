[![justforfunnoreally.dev badge](https://img.shields.io/badge/justforfunnoreally-dev-9ff)](https://justforfunnoreally.dev)

# Tinker

A background coding agent. Give it a prompt, walk away, get results.

## Dependencies

[ripgrep](https://github.com/BurntSushi/ripgrep)

## Installation

1. Add API keys as environment variables:

```bash
export ANTHROPIC_API_KEY="your-api-key-here"
export GOOGLE_API_KEY="your-api-key-here"
export BRAVE_SEARCH_API_KEY="your-api-key-here"  # Optional, enables web search
```

2. Run the installation script (Linux only):

```bash
curl -fsSL https://raw.githubusercontent.com/honganh1206/tinker/main/scripts/install.sh | sudo -E bash
```

## Usage

### Run a task (headless)

```bash
tinker "fix the linting errors in cmd/"
```

### Run with verification

```bash
tinker --verify-cmd "go test ./..." "add unit tests for the user service"
```

### Run with a specific model

```bash
tinker --provider anthropic --model claude-4-sonnet "refactor the auth module"
```

Output is structured JSON on stdout. Logs go to stderr.

### Other commands

```bash
tinker model                    # List available models
tinker sessions                 # List sessions
tinker version                  # Show version
```

## MCP

To add MCP servers:

```bash
tinker mcp --server-cmd "my-server:npx @modelcontextprotocol/server-everything"
```

## Development

```bash
make serve   # Run the persistence server
make run PROMPT='"your prompt here"'  # Run the agent
make test    # Run tests
```

[References](./docs/References.md)
