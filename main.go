package main

import (
	"fmt"
	"log"
	"os"

	"dwight/internal/storage"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func main() {
	if len(os.Args) > 1 && (os.Args[1] == "-h" || os.Args[1] == "--help") {
		showUsage()
		return
	}

	currentDir, _ := os.Getwd()
	workContext := storage.DetectWorkContext(currentDir)
	config := storage.LoadConfig()
	settings := storage.LoadSettings()
	modelConfig := storage.LoadModelConfig()

	ta := textarea.New()
	ta.Placeholder = "Type your message... (Enter sends, Alt+Enter inserts a new line)"
	ta.CharLimit = 8000
	ta.MaxHeight = 10
	ta.SetWidth(120)
	ta.SetHeight(3)
	ta.FocusedStyle.CursorLine = lipgloss.NewStyle()
	ta.ShowLineNumbers = false
	ta.KeyMap.InsertNewline = key.NewBinding(
		key.WithKeys("shift+enter", "alt+enter"),
		key.WithHelp("alt+enter", "insert newline"),
	)

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("#205"))

	m := model{
		width:       80,
		height:      24,
		viewMode:    ViewMenu,
		config:      config,
		settings:    settings,
		modelConfig: modelConfig,
		currentDir:  currentDir,
		workContext: workContext,

		menuCursor: 0,
		menuItems: []MenuItem{
			{Name: "Chat", Desc: "Interactive AI chat"},
			{Name: "Conversation History", Desc: "Browse saved conversations"},
			{Name: "Model Manager", Desc: "Manage AI model profiles"},
			{Name: "Settings", Desc: "Configure preferences"},
			{Name: "Quit", Desc: "Exit Dwight"},
		},

		chatState:    ChatStateInit,
		chatMessages: []ChatMessage{},
		chatTextArea: ta,
		chatSpinner:  sp,
		chatMaxLines: 14,

		editingProfile: -1,
	}

	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		log.Fatal(err)
	}
}

func showUsage() {
	fmt.Println(`dwight - Terminal AI Chat & Doc Manager

USAGE:
    dwight [FLAGS]

FLAGS:
    -h, --help    Show this help message

FEATURES:
    • Chat with Ollama or Gemini models (streaming)
    • Manage model profiles and switch between them
    • Save, load, and export conversation history (global library under your data dir)
    • Attach local files as context (RAG)

ENVIRONMENT:
    OLLAMA_HOST     Ollama API host (default: localhost:11434)
    GEMINI_API_KEY  Gemini API key for Google AI Studio
    GOOGLE_API_KEY  Alternate Gemini API key env var`)
}
