package main

import (
	"fmt"
	"log"
	"os"
	"strings"

	"dwight/internal/storage"

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
	ta.Placeholder = "Type your message... (Enter to send)"
	ta.CharLimit = 8000
	ta.SetWidth(120)
	ta.SetHeight(4)
	ta.FocusedStyle.CursorLine = lipgloss.NewStyle()
	ta.ShowLineNumbers = false
	ta.KeyMap.InsertNewline.SetEnabled(false)

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("#205"))

	m := model{
		width:      80,
		height:     24,
		viewMode:   ViewMenu,
		config:     config,
		settings:   settings,
		modelConfig: modelConfig,
		currentDir:    currentDir,
		workContext:   workContext,

		menuCursor: 0,
		menuItems: []MenuItem{
			{Name: "Chat with Ollama", Desc: "Interactive AI chat"},
			{Name: "Conversation History", Desc: "Browse saved conversations"},
			{Name: "Model Manager", Desc: "Manage AI model profiles"},
			{Name: "Settings", Desc: "Configure preferences"},
			{Name: "Quit", Desc: "Exit Dwight"},
		},

		chatState:        ChatStateInit,
		chatMessages:     []ChatMessage{},
		chatTextArea:     ta,
		chatSpinner:      sp,
		chatMaxLines:     14,
		chatStreamBuffer: &strings.Builder{},

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
    • Chat with local Ollama models (streaming)
    • Manage model profiles and switch between them
    • Save, load, and export conversation history (global library under your data dir)
    • Attach local files as context (RAG)

ENVIRONMENT:
    OLLAMA_HOST    Ollama API host (default: localhost:11434)`)
}
