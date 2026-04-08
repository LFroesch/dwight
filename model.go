package main

import (
	"context"
	"time"

	"dwight/internal/ollama"
	"dwight/internal/storage"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

// =============================================================================
// View modes
// =============================================================================

type ViewMode int

const (
	ViewMenu ViewMode = iota
	ViewChat
	ViewConversationList
	ViewConversationExport
	ViewSettings
	ViewModelManager
	ViewModelCreate
	ViewModelPull
	ViewModelLibrary
	ViewConfirmDialog
)

type ChatState int

const (
	ChatStateInit ChatState = iota
	ChatStateCheckingModel
	ChatStateModelNotAvailable
	ChatStateReady
	ChatStateLoading
	ChatStateError
	ChatStateReview
)

type ConfirmAction int

const (
	ConfirmDeleteModel ConfirmAction = iota
)

// =============================================================================
// Message types
// =============================================================================

type statusMsg struct{ message string }
type tickMsg time.Time

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

type streamStartedMsg struct{ ch <-chan ollama.StreamChunk }

type ClearChatMsg struct{}
type InterruptMsg struct{}

type ModelPullMsg struct {
	Success bool
	Status  string
	Err     error
}

type installedModelsMsg struct {
	models []ollama.Model
}

// =============================================================================
// Chat message (display-level, with cache)
// =============================================================================

type ChatMessage struct {
	Role         string
	Content      string
	Timestamp    time.Time
	Duration     time.Duration
	PromptTokens int
	TotalTokens  int
	// Render cache
	formattedLines []string
	lastWidth      int
}

// =============================================================================
// Confirm dialog
// =============================================================================

type ConfirmDialog struct {
	Action       ConfirmAction
	Message      string
	PreviousView ViewMode
}

// =============================================================================
// Model struct — the single BubbleTea model
// =============================================================================

type model struct {
	// Dimensions
	width  int
	height int

	// Navigation
	viewMode  ViewMode
	showHelp  bool
	statusMsg string
	statusExp time.Time

	// Config & storage (loaded once)
	config       storage.Config
	settings     storage.Settings
	modelConfig  storage.ModelConfig
	currentDir   string
	workContext  storage.WorkContext // cwd + git tagging for new conversations

	// Menu
	menuCursor int
	menuItems  []MenuItem

	// Chat
	chatState        ChatState
	chatMessages     []ChatMessage
	chatTextArea     textarea.Model
	chatSpinner      spinner.Model
	chatErr          error
	chatLines        []string
	chatScrollPos    int
	chatMaxLines     int
	chatStreaming     bool
	chatStreamBuffer string
	chatStreamCh     <-chan ollama.StreamChunk
	cancelChat       context.CancelFunc // cancels in-flight generation

	// Copy mode — navigate messages, yank to clipboard
	chatCopyMode bool
	chatCopyIdx  int // index into chatMessages (-1 = last)

	// Conversation management
	currentConversation *storage.Conversation
	conversations       []storage.ConversationMeta
	selectedConv        int

	// RAG — attached resource paths
	attachedResources  []string
	showResourcePicker bool
	pickerCursor       int

	// Model management
	modelSelection  int
	modelInputs     []textinput.Model
	editingProfile  int // -1 = creating new, >=0 = editing index
	modelPullName   string
	modelPullStatus string
	modelPullError  error

	// Model library
	libraryModels    []ollama.LibraryModel
	installedModels  []ollama.Model
	librarySelection int
	libraryFilter    string

	// Settings editor
	settingsInputs []textinput.Model

	// Conversation export
	exportFormat string

	// @ autocomplete
	showAtComplete    bool
	atCompleteFiles   []string // filtered results
	atCompleteCursor  int
	atCompleteFilter  string
	fileCache         []string // cached project files
	fileCacheDir      string   // dir the cache was built for

	// Code block review (accept/refine/reject)
	codeBlocks  []CodeBlock
	reviewIndex int

	// Dialogs
	confirmDialog *ConfirmDialog
}

// MenuItem represents a menu option.
type MenuItem struct {
	Name string
	Desc string
}

// CodeBlock represents an extracted code block from an AI response.
type CodeBlock struct {
	Language string
	Path     string // detected file path (may be empty)
	Content  string
}

// =============================================================================
// Helpers
// =============================================================================

const (
	minWidth  = 60
	minHeight = 20
)

func (m *model) safeWidth() int {
	if m.width < minWidth {
		return minWidth
	}
	return m.width
}

func (m *model) safeHeight() int {
	if m.height < minHeight {
		return minHeight
	}
	return m.height
}

func (m *model) currentProfile() storage.ModelProfile {
	return m.modelConfig.Current()
}

func showStatus(msg string) tea.Cmd {
	return func() tea.Msg { return statusMsg{message: msg} }
}

func tickCmd() tea.Cmd {
	return tea.Tick(2*time.Second, func(t time.Time) tea.Msg { return tickMsg(t) })
}
