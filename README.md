[![justforfunnoreally.dev badge](https://img.shields.io/badge/justforfunnoreally-dev-9ff)](https://justforfunnoreally.dev)

# Tinker

A background coding agent. Give it a prompt, walk away, get results.

## Development

Add API keys as environment variables:

```bash
export ANTHROPIC_API_KEY="your-api-key-here"
export GOOGLE_API_KEY="your-api-key-here"
export BRAVE_SEARCH_API_KEY="your-api-key-here"  # Optional, enables web search
```


```bash
make serve   # Run the persistence server
make run PROMPT='"your prompt here"'  # Run the agent
make test    # Run tests
```


## Dependencies

[ripgrep](https://github.com/BurntSushi/ripgrep)

## MCP

To add MCP servers:

```bash
tinker mcp --server-cmd "my-server:npx @modelcontextprotocol/server-everything"
```
