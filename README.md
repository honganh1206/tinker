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

## Dependencies

[ripgrep](https://github.com/BurntSushi/ripgrep)

## Discord

To interact with the agent through Discord, set up a Discord bot and run the Discord channel:

1. Create a Discord application and bot at the [Discord Developer Portal](https://discord.com/developers/applications).
2. Under **Bot**, copy the bot token and enable the **Message Content Intent** (the channel also requires the `Guild Messages` and `Direct Messages` intents).
3. Invite the bot to your server with the `bot` scope and at least the following permissions: `View Channels`, `Send Messages`, `Create Public Threads`, `Send Messages in Threads`, and `Read Message History`.
4. Export the bot token (and optionally an instance name) as environment variables:

   ```bash
   export DISCORD_BOT_TOKEN="your-discord-bot-token"
   ```

5. Start NATS and the Discord channel alongside the runner:

   ```bash
   make local-nats     # Start NATS JetStream (in a separate terminal)
   make run-channel    # Build and start the Discord channel
   make run-runner     # Start the agent runner (in a separate terminal)
   ```

   Or start everything (NATS, channel, runner, and web UI) at once:

   ```bash
   make dev-all
   ```

6. In Discord, mention the bot (`@your-bot your prompt`) in any channel it can read. The channel will create a thread for the conversation; reply inside the thread (mentioning the bot) to continue.

## MCP

To add MCP servers:

```bash
tinker mcp --server-cmd "my-server:npx @modelcontextprotocol/server-everything"
```
