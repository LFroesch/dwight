package main

import (
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
)

type AIResource struct {
	Name        string    `json:"name"`
	Path        string    `json:"path"`
	Type        string    `json:"type"`
	Tags        []string  `json:"tags"`
	Description string    `json:"description"`
	Size        int64     `json:"size"`
	ModTime     time.Time `json:"mod_time"`
	IsTemplate  bool      `json:"is_template"`
}

type ProjectMetadata struct {
	ProjectName   string                      `json:"project_name"`
	Created       time.Time                   `json:"created"`
	Resources     map[string]ResourceMetadata `json:"resources"`
	TemplatesUsed []string                    `json:"templates_used"`
	Settings      ProjectSettings             `json:"settings"`
}

type ResourceMetadata struct {
	Tags         []string  `json:"tags"`
	Description  string    `json:"description"`
	Type         string    `json:"type"`
	LastModified time.Time `json:"last_modified"`
}

type ProjectSettings struct {
	DefaultModel string `json:"default_model"`
	AutoScan     bool   `json:"auto_scan"`
}

type Config struct {
	TemplatesDir string   `json:"templates_dir"`
	ResourceDirs []string `json:"resource_dirs"`
	FileTypes    []string `json:"file_types"`
}

type ConfigFile struct {
	App     string `json:"app"`
	Version string `json:"version"`
	Config  Config `json:"config"`
}

const ProjectMetaFile = ".dwight.json"

type ModelProfile struct {
	Name         string  `json:"name"`
	Model        string  `json:"model"`
	SystemPrompt string  `json:"system_prompt"`
	Temperature  float64 `json:"temperature"`
}

type ModelConfig struct {
	Profiles       []ModelProfile `json:"profiles"`
	CurrentProfile int            `json:"current_profile"`
}

type ViewMode int

const (
	ViewMenu ViewMode = iota
	ViewResourceManager
	ViewDetails
	ViewCreate
	ViewChatPlaceholder
	ViewGlobalResourcesPlaceholder
	ViewSettingsPlaceholder
	ViewCleanupPlaceholder
	ViewCleanupChats
	ViewModelManager
	ViewModelCreate
	ViewModelPull
)

type statusMsg struct {
	message string
}

type tickMsg time.Time

type model struct {
	config          Config
	resources       []AIResource
	filteredRes     []AIResource
	globalRes       []AIResource
	table           table.Model
	viewport        viewport.Model
	textInput       textinput.Model
	inputs          []textinput.Model
	menuTable       table.Model
	viewMode        ViewMode
	width           int
	height          int
	statusMsg       string
	statusExpiry    time.Time
	currentDir      string
	projectRoot     string
	projectMeta     *ProjectMetadata
	selectedRes     *AIResource
	editMode        bool
	editField       int
	filterTag       string
	configFile      string
	showHelp        bool
	lastUpdate      time.Time
	cursor          int
	fromGlobal      bool
	chatState       ChatState
	chatMessages    []ChatMessage
	chatInput       textinput.Model
	chatSpinner     spinner.Model
	chatErr         error
	// CUSTOM TEXT VIEWER - NO MORE VIEWPORT BULLSHIT
	chatLines       []string
	chatScrollPos   int
	chatMaxLines    int
	// File viewer
	fileLines       []string
	fileScrollPos   int
	fileMaxLines    int
	modelConfig     ModelConfig
	modelSelection  int
	currentModel    string
	modelInputs     []textinput.Model
	modelPullName   string
	modelPullStatus string
	modelPullError  error
}

type ChatState int

const (
	ChatStateInit ChatState = iota
	ChatStateCheckingModel
	ChatStateReady
	ChatStateLoading
	ChatStateError
)

type ChatMessage struct {
	Role         string
	Content      string
	Duration     time.Duration
	PromptTokens int
	TotalTokens  int
}

type CheckModelMsg struct {
	Available bool
	Err       error
}

type ResponseMsg struct {
	Content      string
	Duration     time.Duration
	Err          error
	PromptTokens int
	TotalTokens  int
}

func showStatus(msg string) tea.Cmd {
	return func() tea.Msg {
		return statusMsg{message: msg}
	}
}

func tickCmd() tea.Cmd {
	return tea.Tick(time.Second*2, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}
