package main

import (
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textarea"
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
	ViewChat
	ViewGlobalResources
	ViewSettings
	ViewCleanup
	ViewCleanupChats
	ViewModelManager
	ViewModelCreate
	ViewModelPull
	ViewConfirmDialog
	ViewConversationList
	ViewConversationExport
	ViewModelLibrary
)

type statusMsg struct {
	message string
}

type ConfirmAction int

const (
	ConfirmDelete ConfirmAction = iota
	ConfirmPush
	ConfirmPull
	ConfirmDeleteModel
)

type ConfirmDialog struct {
	Action      ConfirmAction
	Message     string
	Resource    *AIResource
	PreviousView ViewMode
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
	chatState        ChatState
	chatMessages     []ChatMessage
	chatTextArea     textarea.Model
	chatSpinner      spinner.Model
	chatErr          error
	chatLines        []string
	chatScrollPos    int
	chatMaxLines     int
	chatStreaming    bool
	chatStreamBuffer strings.Builder
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
	// Model library
	libraryModels   []OllamaLibraryModel
	installedModels []OllamaModel
	librarySelection int
	libraryFilter    string
	appSettings     AppSettings
	settingsInputs  []textinput.Model
	sortBy          string
	sortDesc        bool
	confirmDialog   *ConfirmDialog
	// Conversation management
	currentConversation *Conversation
	conversations       []ConversationMetadata
	conversationSearch  string
	selectedConv        int
	// RAG (Retrieval Augmented Generation)
	attachedResources   []string
	showResourcePicker  bool
	// Message selection for copy/edit
	selectedMessage     int
	showMessageMenu     bool
	// Export
	exportFormat        string
}

type ChatState int

const (
	ChatStateInit ChatState = iota
	ChatStateCheckingModel
	ChatStateModelNotAvailable
	ChatStateReady
	ChatStateLoading
	ChatStateStreaming
	ChatStateError
)

type ChatMessage struct {
	Role         string
	Content      string
	Timestamp    time.Time
	Duration     time.Duration
	PromptTokens int
	TotalTokens  int
	// Cache formatted lines to avoid re-rendering
	formattedLines []string
	lastWidth      int
}

type CheckModelMsg struct {
	Available bool
	ModelName string
	Err       error
}

type ResponseMsg struct {
	Content      string
	Duration     time.Duration
	Err          error
	PromptTokens int
	TotalTokens  int
}

type StreamChunkMsg struct {
	Content      string
	Done         bool
	Err          error
	Duration     time.Duration
	PromptTokens int
	TotalTokens  int
}

type ClearChatMsg struct{}

type CopyToClipboardMsg struct {
	Content string
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
