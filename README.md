# Dwight

Terminal AI chat client for local Ollama models. Chat, manage conversations, attach files as context, switch between model profiles. Built with Go and [Bubble Tea](https://github.com/charmbracelet/bubbletea).

## Quick Install

Supported platforms: Linux and macOS. On Windows, use WSL.

Recommended (installs to `~/.local/bin`):

```bash
curl -fsSL https://raw.githubusercontent.com/LFroesch/dwight/main/install.sh | bash
```

Or download a binary from [GitHub Releases](https://github.com/LFroesch/dwight/releases).

Or build from source:

```bash
make install
```

## Usage

```bash
dwight                                       # connect to localhost:11434
OLLAMA_HOST=xxx.xx.xx.x:11434 dwight        # connect to remote host
DWIGHT_MODEL=llama3.2:3b dwight             # override default model
```

## Features

- **Chat** — Multi-line input, markdown rendering, token tracking, and multi-message copy mode
- **Draft Controls** — `ctrl+c` clears the current draft, then closes chat when the input is already empty
- **Model Profiles** — Switch between saved model configurations (Alt+,/.)
- **Conversations** — Save, load, resume, and export to Markdown/JSON with timestamps, project/day export folders, and inline status feedback
- **RAG** — Attach local files as context for the current chat (Ctrl+R)
- **Model Library** — Browse available Ollama models, pull new ones

## Keybindings

### Menu
| Key | Action |
|-----|--------|
| `j/k` | Navigate |
| `enter` | Select |
| `q` | Quit |

### Chat
| Key | Action |
|-----|--------|
| `enter` | Send message |
| `ctrl+c` | Clear draft, or close chat if draft is empty |
| `ctrl+o` | Export current chat to Markdown |
| `ctrl+l` | Clear chat |
| `ctrl+s` | Save conversation |
| `ctrl+n` | New conversation |
| `ctrl+r` | Attach file (RAG) |
| `ctrl+y` | Copy mode (`space` mark, `y` copy selected/current) |
| `alt+,` / `alt+.` | Switch model profile |
| `esc` | Back to menu |

## Configuration

Stored in `~/.local/share/dwight/`:

| File | Purpose |
|------|---------|
| `config.json` | App config (file types, templates dir) |
| `.dwight-models.json` | Model profiles (name, model, temperature, etc.) |
| `settings.json` | System prompt, username, timeout |
| `conversations/` | Saved conversation history (JSON) |
| `exports/` | Exports grouped by project and day, e.g. `exports/<project>/YYYY-MM-DD/04-18-26_3-12-pm.md` |

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `OLLAMA_HOST` | `localhost:11434` | Ollama API endpoint |
| `DWIGHT_MODEL` | `qwen2.5:7b` | Default model for new profiles |

## Requirements

- Go 1.23+
- Ollama running (local or remote via `OLLAMA_HOST`)

## License

[AGPL-3.0](LICENSE)
