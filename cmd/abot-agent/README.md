# abot-agent

Interactive CLI for AI agents.

## Installation

```bash
# Build from source
cd /Users/tangredtea/Desktop/project/abot
go build -o bin/abot-agent ./cmd/abot-agent
```

## Quick Start

```bash
# 1. Create config
cp config.agent.example.yaml config.yaml
vim config.yaml  # Fill in your API key

# 2. Run
./bin/abot-agent

# 3. Chat
You: Hello
Bot: Hello! How can I help you?
```

## Commands

- `/help` - Show help message
- `/agents` - List available agents
- `/switch <id>` - Switch to a different agent
- `/session new` - Start a new session
- `/session info` - Show current session info
- `/history` - Show recent conversation history
- `/clear` - Clear the screen
- `/model` - Show the current agent's model
- `/status` - Show system status
- `/exit`, `/quit` - Exit the REPL

## Multi-line Input

You can enter multi-line input using:

- Triple quotes: `"""` ... `"""`
- Code blocks: ` ``` ` ... ` ``` `
- Backslash continuation: `\` at end of line

## Configuration

See `config.agent.example.yaml` for a minimal configuration template.

Required fields:
- `providers` - At least one LLM provider with API key

Optional fields:
- `agents` - Agent definitions (auto-created if empty)
- `session` - Session storage configuration
- `context_window` - Context window size
- `bus_buffer_size` - Message bus buffer size

## Features

- Interactive REPL with readline support
- Streaming responses
- Ctrl+C to interrupt current request
- Tab completion for commands
- Local conversation history
- No database required (lightweight)

## Command-line Options

```bash
abot-agent [options]

Options:
  -config string
        path to config file (default "config.yaml")
  -tenant string
        tenant ID (default "default-tenant")
  -user string
        user ID (default "default-user")
```

## Examples

```bash
# Use custom config file
./bin/abot-agent -config my-config.yaml

# Specify tenant and user
./bin/abot-agent -tenant my-tenant -user my-user

# Multi-line input
You: """
Write a function that:
1. Takes a list of numbers
2. Returns the sum
"""
```
