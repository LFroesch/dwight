# Dwight

Terminal AI chat client for Ollama and Gemini models. Chat, manage conversations, attach files as context, and switch between provider-aware model profiles. Built with Go and [Bubble Tea](https://github.com/charmbracelet/bubbletea).

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
GEMINI_API_KEY=your_key dwight              # use Gemini profiles with an API key
```

## Gemini Demo Setup

For a demo deployment where you cannot run Ollama:

```bash
export GEMINI_API_KEY="your-google-ai-studio-key"
dwight
```

Then in Dwight:

1. Open `Model Manager`
2. Press `n`
3. Set `Provider` to `gemini`
4. Set `Model` to `gemini-2.5-flash`
5. Save the profile and make it the default

Dwight will use `GEMINI_API_KEY` automatically. `GOOGLE_API_KEY` also works.

## Features

- **Chat** — Auto-growing multi-line composer with internal scrolling, arrow-key cursor movement, markdown rendering, token tracking, and multi-message copy mode with speaker labels in clipboard output
- **Draft Controls** — `ctrl+c` clears the current draft, then closes chat when the input is already empty
- **Model Profiles** — Switch between saved Ollama/Gemini configurations (Alt+,/.)
- **Conversations** — Save, load, resume, and export to Markdown/JSON with timestamps, project/day export folders, and inline status feedback
- **RAG** — Attach local files as context for the current chat (Ctrl+R)
- **Model Library** — Browse available Ollama models and pull new ones
- **Help Overlay** — Press `?` anywhere for keybindings and provider setup hints

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
| `alt+enter` | Insert newline in the chat draft |
| `up` / `down` | Move within a multi-line draft |
| `pgup` / `pgdown` | Scroll chat history |
| `ctrl+c` | Clear draft, or close chat if draft is empty |
| `ctrl+o` | Export current chat to Markdown |
| `ctrl+l` | Clear chat |
| `ctrl+s` | Save conversation |
| `ctrl+n` | New conversation |
| `ctrl+r` | Attach file (RAG) |
| `ctrl+y` | Copy mode (`space` mark, `y` copy selected/current) |
| `alt+,` / `alt+.` | Switch model profile |
| `?` | Toggle help overlay |
| `esc` | Back to menu |

## Configuration

Stored in `~/.local/share/dwight/`:

| File | Purpose |
|------|---------|
| `config.json` | App config (file types, templates dir) |
| `.dwight-models.json` | Model profiles (`provider`, model, temperature, system prompt) |
| `settings.json` | System prompt, username, timeout |
| `conversations/` | Saved conversation history (JSON) |
| `exports/` | Exports grouped by project and day, e.g. `exports/<project>/YYYY-MM-DD/04-18-26_3-12-pm.md` |

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `OLLAMA_HOST` | `localhost:11434` | Ollama API endpoint |
| `DWIGHT_MODEL` | `qwen2.5:7b` | Default model for new profiles |
| `GEMINI_API_KEY` | unset | Gemini API key from Google AI Studio |
| `GOOGLE_API_KEY` | unset | Alternate Gemini API key env var |

## Requirements

- Go 1.23+
- Ollama running locally/remotely for Ollama profiles
- Gemini API key for Gemini profiles

## License

[AGPL-3.0](LICENSE)
