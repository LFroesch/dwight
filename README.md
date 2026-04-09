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
```

## Features

- **Chat** — Multi-line input, markdown rendering, token tracking
- **Model Profiles** — Switch between saved model configurations (Alt+,/.)
- **Conversations** — Save, load, resume, export to Markdown/JSON
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
| `ctrl+l` | Clear chat |
| `ctrl+s` | Save conversation |
| `ctrl+n` | New conversation |
| `ctrl+r` | Attach file (RAG) |
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

## Requirements

- Go 1.23+
- Ollama running (local or remote via `OLLAMA_HOST`)

## License

[AGPL-3.0](LICENSE)
