# Dwight Assistant Manager

Terminal-based AI resource manager and assistant powered by Docker + Ollama with support for custom models. Organizes prompts, templates, and project files while providing AI assistance.

## ✨ Features

### 🤖 AI Chat
- **Streaming Responses**: Real-time AI responses as they generate
- **Multi-line Input**: Full textarea support for longer messages (8000 char limit)
- **Markdown Rendering**: Code blocks, headers, and lists automatically formatted
- **Model Switching**: Quick model switching with Tab/Shift+Tab
- **Conversation Management**: Save, load, resume, and delete conversations
- **Export Conversations**: Export to Markdown, JSON, or plain text formats
- **Custom Profiles**: Multiple AI model profiles with custom system prompts
- **Token Tracking**: Real-time token count and context window usage display
- **Context Window Management**: Auto-trim conversations to fit model limits
- **RAG Support**: Attach local resources as context for AI responses
- **Smart Chat Stats**: View response time, tokens/second, and conversation metrics

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
- `Ctrl+S` - Save current conversation
- `Ctrl+O` - Open conversation history
- `Ctrl+N` - Start new conversation
- `Ctrl+R` - Attach/detach resources for RAG
- `Ctrl+T` - Trim conversation to fit context window
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

## 💬 Conversation Management

Conversations are automatically saved as structured JSON files in `./conversations/` and can be:
- **Loaded and resumed** at any time
- **Searched and filtered** by title, model, or content
- **Exported** to Markdown, JSON, or plain text
- **Tagged and organized** for easy retrieval

Each conversation includes:
- Complete message history with timestamps
- Token usage statistics
- Model and profile information
- Attached resources for context

## 💾 Chat Logs

Legacy chat logs are saved to `./chats/` with format:
```
MM_DD_YY_H_MM_AM/PM_modelname.txt
```

## 📎 RAG (Retrieval Augmented Generation)

Attach local files as context for your AI conversations:
1. Press `Ctrl+R` in chat to open the resource picker
2. Use `↑↓` to navigate, `Space` to toggle selection
3. Press `Enter` to attach selected resources
4. The AI will use the file contents to provide contextual answers

Supported file types: `.md`, `.txt`, `.json`, `.yaml`, `.yml`, `.toml`, `.go`, `.py`, `.js`, `.ts`

## ⚙️ Configuration

Configuration stored in `~/.local/share/dwight/`:
- `config.json` - Main configuration
- `.dwight-models.json` - Model profiles
- `settings.json` - App settings
- `templates/` - Global templates directory