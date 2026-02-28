---
name: tmux
description: Remote-control tmux sessions for interactive CLIs by sending keystrokes and scraping pane output. Use when the user needs an interactive TTY.
---

# tmux Skill

Use tmux only when you need an interactive TTY. Prefer `exec` tool for non-interactive tasks.

## Quickstart (exec tool)

```bash
SOCKET="${TMPDIR:-/tmp}/abot-tmux.sock"
SESSION=abot-shell

tmux -S "$SOCKET" new -d -s "$SESSION" -n shell
tmux -S "$SOCKET" send-keys -t "$SESSION":0.0 -- 'echo hello' Enter
tmux -S "$SOCKET" capture-pane -p -J -t "$SESSION":0.0 -S -200
```

## Sending Input

- Literal sends: `tmux -S "$SOCKET" send-keys -t target -l -- "$cmd"`
- Control keys: `tmux -S "$SOCKET" send-keys -t target C-c`

## Watching Output

- Capture recent: `tmux -S "$SOCKET" capture-pane -p -J -t target -S -200`

## Cleanup

- Kill session: `tmux -S "$SOCKET" kill-session -t "$SESSION"`
- Kill all: `tmux -S "$SOCKET" kill-server`

## Notes

- Supported on macOS/Linux only, requires `tmux` on PATH
- Use separate sessions for parallel tasks
- Always print monitor commands after starting a session
