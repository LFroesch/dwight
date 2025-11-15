# Dwight Assistant Manager

Terminal-based AI resource manager and assistant powered by Docker + Ollama with support for custom models. Organizes prompts, templates, and project files while providing AI assistance.

## ✨ Features

### 🤖 AI Chat
- **Streaming Responses**: Real-time AI responses as they generate
- **Multi-line Input**: Full textarea support for longer messages (4000 char limit)
- **Markdown Rendering**: Code blocks, headers, and lists automatically formatted
- **Model Switching**: Quick model switching with Tab/Shift+Tab
- **Chat Management**: Save, clear, and export conversations
- **Custom Profiles**: Multiple AI model profiles with custom system prompts

### 📁 Resource Management
- **Local Resources**: Browse and manage files in current directory
- **Global Resources**: Access system-wide templates and resources
- **Smart Tagging**: Tag, categorize, and search AI resources
- **Push/Pull**: Transfer resources between local and global locations
- **Fuzzy Search**: Quick filtering by name, tags, or content
- **Sorting**: Sort by name, type, size, or modification date

### ⌨️ Chat Shortcuts
- `Enter` - Send message
- `Ctrl+L` - Clear chat without exiting
- `Ctrl+S` - Save chat log manually
- `Tab` - Switch to next model
- `Shift+Tab` - Switch to previous model
- `↑/↓` - Scroll through chat history
- `PgUp/PgDn` - Page through chat
- `Home/End` - Jump to top/bottom
- `Esc` - Exit to menu

### 📋 Resource Shortcuts
- `↑/↓` - Navigate resources
- `Enter/Space/v` - View resource details
- `i` - View full details
- `e` - Edit resource metadata
- `n/a` - Add new resource
- `f` - Search/filter resources
- `s` - Cycle sort options
- `S` - Reverse sort direction
- `p` - Push local to global
- `d` - Delete resource
- `r` - Refresh resources
- `Esc` - Back to menu
- `q` - Quit application

## 🚀 Quick Start

```bash
# Build
make build

# Run
./dwight

# Or with Go
go run .
```

## 📦 File Types

Supports: `.md`, `.txt`, `.json`, `.yaml`, `.yml`, `.toml`, `.go`, `.py`, `.js`, `.ts`

## 🎨 Model Profiles

Create custom AI profiles with different models and system prompts:
- **Coder Assistant** (qwen2.5-coder:7b) - Code generation and review
- **General Assistant** (llama3.2:3b) - General purpose conversations
- **Creative Writer** (llama3.2:3b) - Creative and descriptive writing

## 💾 Chat Logs

Chat logs are automatically saved to `./chats/` with format:
```
MM_DD_YY_H_MM_AM/PM_modelname.txt
```

## ⚙️ Configuration

Configuration stored in `~/.local/share/dwight/`:
- `config.json` - Main configuration
- `.dwight-models.json` - Model profiles
- `settings.json` - App settings
- `templates/` - Global templates directory