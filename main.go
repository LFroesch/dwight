package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func main() {
	if len(os.Args) > 1 && (os.Args[1] == "-h" || os.Args[1] == "--help") {
		showUsage()
		return
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		log.Fatal(err)
	}

	configFile := filepath.Join(homeDir, ".local/share/dwight/config.json")
	currentDir, _ := os.Getwd()

	os.MkdirAll(filepath.Dir(configFile), 0755)

	config := loadConfig(configFile)
	configChanged := false

	if config.TemplatesDir == "" {
		config.TemplatesDir = filepath.Join(homeDir, ".local/share/dwight/templates")
		os.MkdirAll(config.TemplatesDir, 0755)
		configChanged = true
	}

	if len(config.FileTypes) == 0 {
		config.FileTypes = []string{".md", ".txt", ".json", ".yaml", ".yml", ".py", ".js", ".ts"}
		configChanged = true
	}

	if configChanged || !fileExists(configFile) {
		saveConfig(configFile, config)
	}

	projectRoot := findProjectRoot(currentDir)

	m := model{
		config:      config,
		configFile:  configFile,
		currentDir:  currentDir,
		projectRoot: projectRoot,
		width:       100,
		height:      24,
		viewMode:    ViewMenu,
		editMode:    false,
		editField:   0,
		showHelp:    false,
		lastUpdate:  time.Now(),
	}

	m.loadProjectMetadata()
	m.loadModelConfig()

	m.chatInput = textinput.New()
	m.chatInput.Placeholder = "Type your message..."
	m.chatInput.CharLimit = 500
	m.chatInput.Width = 100

	m.chatSpinner = spinner.New()
	m.chatSpinner.Spinner = spinner.Dot
	m.chatSpinner.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	m.chatState = ChatStateInit
	m.chatMessages = []ChatMessage{}
	m.chatViewport = viewport.New(m.width-6, m.height-8)

	menuColumns := []table.Column{
		{Title: "Option", Width: 30},
		{Title: "Description", Width: 50},
	}

	menuTable := table.New(
		table.WithColumns(menuColumns),
		table.WithFocused(true),
		table.WithHeight(8),
	)

	s := table.DefaultStyles()
	s.Header = s.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("#374151")).
		BorderBottom(true).
		Bold(true).
		Foreground(lipgloss.Color("#F3F4F6")).
		Background(lipgloss.Color("#1F2937"))
	s.Selected = s.Selected.
		Foreground(lipgloss.Color("#F3F4F6")).
		Background(lipgloss.Color("#7C3AED")).
		Bold(true)
	s.Cell = s.Cell.
		Foreground(lipgloss.Color("#E5E7EB"))
	menuTable.SetStyles(s)

	menuRows := []table.Row{
		{"Resource Manager", "Manage AI resources, templates, and prompts"},
		{"Chat with Ollama", "Interactive AI chat interface"},
		{"View Global Resources", "Browse system-wide AI resources"},
		{"Settings", "Configure Dwight preferences (Coming Soon)"},
		{"Stop Ollama", "Stop Ollama container to free memory"},
		{"Clean Up Old Resources", "Remove unused or outdated resources (Coming Soon)"},
		{"Clean Up Project Chat Logs", "Remove old chat logs from this project"},
		{"Model Manager", "Manage AI models and profiles"},
		{"Quit", "Exit Dwight"},
	}
	menuTable.SetRows(menuRows)
	m.menuTable = menuTable

	columns := []table.Column{
		{Title: "Name", Width: 25},
		{Title: "Type", Width: 12},
		{Title: "Size", Width: 10},
		{Title: "Tags", Width: 20},
		{Title: "Modified", Width: 15},
		{Title: "Path", Width: 30},
	}

	t := table.New(
		table.WithColumns(columns),
		table.WithFocused(true),
		table.WithHeight(10),
	)

	t.SetStyles(s)
	m.table = t

	m.viewport = viewport.New(80, 20)

	m.scanResources()
	m.updateTableData()

	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		log.Fatal(err)
	}
	stopOllamaContainer()
}

func showUsage() {
	fmt.Println(`Dwight - AI Resource File Manager

USAGE:
    dwight [FLAGS]

FLAGS:
    -h, --help    Show this help message

FEATURES:
    • Scan and manage AI resource files (templates, prompts, contexts)
    • Create new templates from files or directories
    • Tag and categorize resources
    • Quick file content preview
    • Template instantiation
    • Project-based metadata (.dwight.json)
    • Ollama chat integration (Coming Soon)

MENU OPTIONS:
    Resource Manager     - Manage local AI resources
    Chat with Ollama     - Interactive AI chat
    View Global Resources - Browse system resources
    Settings            - Configure preferences
    Clean Up Old Resources - Remove unused files
    Quit                - Exit application`)
}
