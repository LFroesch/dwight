package main

import (
	"context"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"dwight/internal/ollama"
	"dwight/internal/storage"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

func (m model) Init() tea.Cmd {
	return tickCmd()
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case statusMsg:
		m.statusMsg = msg.message
		m.statusExp = time.Now().Add(3 * time.Second)
		return m, nil

	case tickMsg:
		return m, tickCmd()

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.adjustLayout()
		if m.viewMode == ViewChat {
			m.updateChatLines()
		}
		return m, nil

	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
		if msg.String() == "?" {
			m.showHelp = !m.showHelp
			return m, nil
		}
		if m.showHelp {
			if msg.String() == "q" || msg.String() == "esc" {
				m.showHelp = false
			}
			return m, nil
		}
		switch m.viewMode {
		case ViewMenu:
			return m.updateMenu(msg)
		case ViewChat:
			return m.updateChat(msg)
		case ViewConversationList:
			return m.updateConversationList(msg)
		case ViewConversationExport:
			return m.updateConversationExport(msg)
		case ViewSettings:
			return m.updateSettings(msg)
		case ViewModelManager:
			return m.updateModelManager(msg)
		case ViewModelCreate:
			return m.updateModelCreate(msg)
		case ViewModelPull:
			return m.updateModelPull(msg)
		case ViewModelLibrary:
			return m.updateModelLibrary(msg)
		case ViewConfirmDialog:
			return m.updateConfirmDialog(msg)
		}

	case spinner.TickMsg:
		if m.chatState == ChatStateLoading || m.chatState == ChatStateCheckingModel {
			var cmd tea.Cmd
			m.chatSpinner, cmd = m.chatSpinner.Update(msg)
			m.updateChatLines()
			return m, cmd
		}
		return m, nil

	case CheckModelMsg:
		if msg.Err != nil {
			m.chatErr = msg.Err
			m.chatState = ChatStateError
		} else if !msg.Available {
			m.chatState = ChatStateModelNotAvailable
			m.modelPullName = msg.ModelName
		} else {
			m.chatState = ChatStateReady
			m.chatTextArea.Focus()
		}
		return m, nil

	case ResponseMsg:
		if msg.Err != nil {
			m.chatErr = msg.Err
			m.chatState = ChatStateError
		} else {
			m.chatMessages = append(m.chatMessages, ChatMessage{
				Role: "assistant", Content: msg.Content, Timestamp: time.Now(),
				Duration: msg.Duration, PromptTokens: msg.PromptTokens, TotalTokens: msg.TotalTokens,
			})
			// Check for code blocks — enter review mode if found
			blocks := extractCodeBlocks(msg.Content)
			if len(blocks) > 0 && hasActionableBlocks(blocks) {
				m.codeBlocks = blocks
				m.reviewIndex = 0
				m.chatState = ChatStateReview
			} else {
				m.chatState = ChatStateReady
				m.chatTextArea.Focus()
			}
			m.updateChatLines()
		}
		return m, nil

	case streamStartedMsg:
		m.chatStreamCh = msg.ch
		m.chatStreaming = true
		m.chatState = ChatStateLoading
		return m, listenForChunk(msg.ch)

	case StreamChunkMsg:
		if !m.chatStreaming {
			return m, nil
		}
		if msg.Err != nil {
			m.chatErr = msg.Err
			m.chatState = ChatStateError
			m.chatStreaming = false
			m.chatStreamBuffer = ""
			m.updateChatLines()
			return m, nil
		}
		if msg.Done {
			if m.chatStreamBuffer != "" {
				m.chatMessages = append(m.chatMessages, ChatMessage{
					Role: "assistant", Content: m.chatStreamBuffer,
					Duration: msg.Duration, PromptTokens: msg.PromptTokens, TotalTokens: msg.TotalTokens,
				})
				m.chatStreamBuffer = ""
			}
			// Check for code blocks — enter review mode if found
			content := m.chatMessages[len(m.chatMessages)-1].Content
			blocks := extractCodeBlocks(content)
			if len(blocks) > 0 && hasActionableBlocks(blocks) {
				m.codeBlocks = blocks
				m.reviewIndex = 0
				m.chatState = ChatStateReview
			} else {
				m.chatState = ChatStateReady
				m.chatTextArea.Focus()
			}
			m.chatStreaming = false
			m.updateChatLines()
			return m, nil
		}
		m.chatStreamBuffer += msg.Content
		m.updateChatLines()
		return m, listenForChunk(m.chatStreamCh)

	case ClearChatMsg:
		m.chatMessages = []ChatMessage{}
		m.chatStreamBuffer = ""
		m.chatStreaming = false
		m.updateChatLines()
		return m, showStatus("Chat cleared")

	case InterruptMsg:
		m.chatState = ChatStateReady
		m.chatStreaming = false
		m.chatStreamBuffer = ""
		m.chatTextArea.Focus()
		m.updateChatLines()
		return m, showStatus("Interrupted")

	case ModelPullMsg:
		if msg.Err != nil {
			m.modelPullError = msg.Err
			m.modelPullStatus = ""
		} else {
			m.modelPullStatus = fmt.Sprintf("Successfully pulled: %s", m.modelPullName)
		}
		return m, nil

	case installedModelsMsg:
		m.installedModels = msg.models
		return m, showStatus(fmt.Sprintf("Refreshed: %d models installed", len(msg.models)))
	}

	return m, nil
}

// =============================================================================
// Menu
// =============================================================================

func (m model) updateMenu(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit
	case "up", "k":
		if m.menuCursor > 0 {
			m.menuCursor--
		}
	case "down", "j":
		if m.menuCursor < len(m.menuItems)-1 {
			m.menuCursor++
		}
	case "enter", " ":
		switch m.menuCursor {
		case 0: // Chat
			m.viewMode = ViewChat
			m.chatState = ChatStateCheckingModel
			m.chatTextArea.Focus()
			m.chatMessages = []ChatMessage{}
			m.chatStreamBuffer = ""
			m.chatStreaming = false
			m.updateChatLines()
			return m, tea.Batch(m.checkModel(), m.chatSpinner.Tick)
		case 1: // Conversations
			m.viewMode = ViewConversationList
			convs, err := storage.ListConversations()
			if err == nil {
				m.conversations = convs
				m.selectedConv = 0
			}
		case 2: // Model Manager
			m.viewMode = ViewModelManager
			m.modelSelection = m.modelConfig.CurrentProfile
		case 3: // Settings
			m.viewMode = ViewSettings
			m.initSettingsInputs()
		case 4: // Quit
			return m, tea.Quit
		}
	}
	return m, nil
}

// =============================================================================
// Chat
// =============================================================================

func (m model) updateChat(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// @ autocomplete overlay
	if m.showAtComplete {
		return m.updateAtComplete(msg)
	}

	// Resource picker sub-mode
	if m.showResourcePicker {
		return m.updateResourcePicker(msg)
	}

	// Handle review mode first (before main switch to avoid duplicate cases)
	if m.chatState == ChatStateReview {
		switch msg.String() {
		case "a":
			return m.acceptCodeBlock()
		case "r":
			m.chatState = ChatStateReady
			m.chatTextArea.Focus()
			m.chatTextArea.SetValue("refine this: ")
			return m, nil
		case "n", "esc":
			m.reviewIndex++
			if m.reviewIndex >= len(m.codeBlocks) || !hasMoreActionableBlocks(m.codeBlocks, m.reviewIndex) {
				m.chatState = ChatStateReady
				m.codeBlocks = nil
				m.reviewIndex = 0
				m.chatTextArea.Focus()
			}
			return m, nil
		}
		return m, nil
	}

	// Copy mode — navigate messages, yank to clipboard
	if m.chatCopyMode {
		return m.updateChatCopyMode(msg)
	}

	switch msg.String() {
	case "esc":
		// If generating, cancel and clean up immediately so late StreamChunkMsgs are ignored.
		if m.chatState == ChatStateLoading || m.chatStreaming {
			if m.cancelChat != nil {
				m.cancelChat()
				m.cancelChat = nil
			}
			m.chatState = ChatStateReady
			m.chatStreaming = false
			m.chatStreamBuffer = ""
			m.chatTextArea.Focus()
			m.updateChatLines()
			return m, showStatus("Interrupted")
		}
		// Save and exit to menu
		if len(m.chatMessages) > 0 {
			m.saveCurrentChat()
		}
		m.viewMode = ViewMenu
		m.chatTextArea.Blur()
		m.chatMessages = []ChatMessage{}
		m.chatState = ChatStateInit
		m.chatStreamBuffer = ""
		m.chatStreaming = false
		return m, nil

	case "ctrl+l":
		if m.chatState == ChatStateReady {
			return m, func() tea.Msg { return ClearChatMsg{} }
		}

	case "ctrl+s":
		if len(m.chatMessages) > 0 {
			if err := m.saveCurrentChat(); err == nil {
				return m, showStatus("Conversation saved")
			}
			return m, showStatus("Failed to save")
		}

	case "ctrl+r":
		if m.chatState == ChatStateReady {
			m.showResourcePicker = !m.showResourcePicker
			m.pickerCursor = 0
			return m, nil
		}

	case "ctrl+n":
		if m.chatState == ChatStateReady {
			if len(m.chatMessages) > 0 {
				m.saveCurrentChat()
			}
			m.chatMessages = []ChatMessage{}
			m.currentConversation = nil
			m.attachedResources = nil
			m.chatStreamBuffer = ""
			m.updateChatLines()
			return m, showStatus("New conversation")
		}

	case "ctrl+y":
		if len(m.chatMessages) > 0 {
			m.chatCopyMode = true
			m.chatCopyIdx = len(m.chatMessages) - 1
			m.updateChatLines()
			return m, nil
		}

	case "alt+.":
		if m.chatState == ChatStateReady && !m.chatStreaming {
			m.modelConfig.CurrentProfile = (m.modelConfig.CurrentProfile + 1) % len(m.modelConfig.Profiles)
			storage.SaveModelConfig(m.modelConfig)
			m.chatState = ChatStateCheckingModel
			m.updateChatLines()
			return m, tea.Batch(m.checkModel(), m.chatSpinner.Tick)
		}

	case "alt+,":
		if m.chatState == ChatStateReady && !m.chatStreaming {
			m.modelConfig.CurrentProfile = (m.modelConfig.CurrentProfile - 1 + len(m.modelConfig.Profiles)) % len(m.modelConfig.Profiles)
			storage.SaveModelConfig(m.modelConfig)
			m.chatState = ChatStateCheckingModel
			m.updateChatLines()
			return m, tea.Batch(m.checkModel(), m.chatSpinner.Tick)
		}

	case "y", "Y":
		if m.chatState == ChatStateModelNotAvailable {
			m.chatState = ChatStateCheckingModel
			m.updateChatLines()
			return m, tea.Batch(m.pullModel(), m.chatSpinner.Tick)
		}

	case "n", "N":
		if m.chatState == ChatStateModelNotAvailable {
			m.viewMode = ViewMenu
			m.chatTextArea.Blur()
			m.chatMessages = []ChatMessage{}
			m.chatState = ChatStateInit
			return m, nil
		}

	case "enter":
		if m.chatState == ChatStateReady && strings.TrimSpace(m.chatTextArea.Value()) != "" {
			userMsg := strings.TrimSpace(m.chatTextArea.Value())
			m.chatMessages = append(m.chatMessages, ChatMessage{
				Role: "user", Content: userMsg, Timestamp: time.Now(),
			})
			m.chatTextArea.Reset()
			m.chatState = ChatStateLoading
			m.updateChatLines()
			return m, tea.Batch(
				m.sendChat(userMsg),
				m.chatSpinner.Tick,
			)
		}

	case "up", "down", "pgup", "pgdown", "home", "end":
		m.handleChatScroll(msg.String())
		return m, nil
	}

	// Update textarea
	if m.chatState == ChatStateReady && !m.chatStreaming {
		var cmd tea.Cmd
		m.chatTextArea, cmd = m.chatTextArea.Update(msg)

		// Detect @ typed — trigger autocomplete
		val := m.chatTextArea.Value()
		if strings.HasSuffix(val, "@") {
			m.showAtComplete = true
			m.atCompleteFilter = ""
			m.atCompleteCursor = 0
			allFiles := m.scanProjectFiles()
			m.atCompleteFiles = fuzzyMatch(allFiles, "")
			if len(m.atCompleteFiles) > 20 {
				m.atCompleteFiles = m.atCompleteFiles[:20]
			}
		}

		return m, cmd
	}
	return m, nil
}

func (m model) updateAtComplete(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.showAtComplete = false
		return m, nil
	case "up", "ctrl+k":
		if m.atCompleteCursor > 0 {
			m.atCompleteCursor--
		}
	case "down", "ctrl+j":
		if m.atCompleteCursor < len(m.atCompleteFiles)-1 {
			m.atCompleteCursor++
		}
	case "enter", "tab":
		if m.atCompleteCursor < len(m.atCompleteFiles) {
			selected := m.atCompleteFiles[m.atCompleteCursor]
			// Replace the @filter with @selected-path in textarea
			val := m.chatTextArea.Value()
			// Find the last @ and replace everything after it
			lastAt := strings.LastIndex(val, "@")
			if lastAt >= 0 {
				m.chatTextArea.SetValue(val[:lastAt] + "@" + selected + " ")
			}
			m.showAtComplete = false
		}
		return m, nil
	case "backspace":
		if m.atCompleteFilter == "" {
			// Remove the @ from textarea too
			val := m.chatTextArea.Value()
			if strings.HasSuffix(val, "@") {
				m.chatTextArea.SetValue(val[:len(val)-1])
			}
			m.showAtComplete = false
			return m, nil
		}
		m.atCompleteFilter = m.atCompleteFilter[:len(m.atCompleteFilter)-1]
		m.atCompleteCursor = 0
		allFiles := m.scanProjectFiles()
		m.atCompleteFiles = fuzzyMatch(allFiles, m.atCompleteFilter)
		if len(m.atCompleteFiles) > 20 {
			m.atCompleteFiles = m.atCompleteFiles[:20]
		}
		// Update textarea to reflect filter
		val := m.chatTextArea.Value()
		lastAt := strings.LastIndex(val, "@")
		if lastAt >= 0 {
			m.chatTextArea.SetValue(val[:lastAt] + "@" + m.atCompleteFilter)
		}
	default:
		// Typing characters — add to filter
		key := msg.String()
		if len(key) == 1 {
			m.atCompleteFilter += key
			m.atCompleteCursor = 0
			allFiles := m.scanProjectFiles()
			m.atCompleteFiles = fuzzyMatch(allFiles, m.atCompleteFilter)
			if len(m.atCompleteFiles) > 20 {
				m.atCompleteFiles = m.atCompleteFiles[:20]
			}
			// Update textarea to reflect filter
			val := m.chatTextArea.Value()
			lastAt := strings.LastIndex(val, "@")
			if lastAt >= 0 {
				m.chatTextArea.SetValue(val[:lastAt] + "@" + m.atCompleteFilter)
			}
		}
	}
	return m, nil
}

func (m model) updateChatCopyMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "q":
		m.chatCopyMode = false
		m.updateChatLines()
		return m, nil
	case "up", "k":
		if m.chatCopyIdx > 0 {
			m.chatCopyIdx--
			m.updateChatLines()
		}
	case "down", "j":
		if m.chatCopyIdx < len(m.chatMessages)-1 {
			m.chatCopyIdx++
			m.updateChatLines()
		}
	case "y", "enter":
		if m.chatCopyIdx < len(m.chatMessages) {
			text := m.chatMessages[m.chatCopyIdx].Content
			m.chatCopyMode = false
			return m, func() tea.Msg {
				if err := copyToClipboard(text); err != nil {
					return statusMsg{message: fmt.Sprintf("Copy failed: %v", err)}
				}
				return statusMsg{message: "Copied to clipboard"}
			}
		}
	}
	return m, nil
}

func (m model) updateResourcePicker(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Scan for .md/.txt/.json files in current dir for attachment
	files := m.scanAttachableFiles()

	switch msg.String() {
	case "esc", "enter":
		m.showResourcePicker = false
		m.chatTextArea.Focus()
		return m, showStatus(fmt.Sprintf("%d resources attached", len(m.attachedResources)))
	case "up", "k":
		if m.pickerCursor > 0 {
			m.pickerCursor--
		}
	case "down", "j":
		if m.pickerCursor < len(files)-1 {
			m.pickerCursor++
		}
	case " ":
		if m.pickerCursor < len(files) {
			path := files[m.pickerCursor]
			// Toggle
			idx := -1
			for i, p := range m.attachedResources {
				if p == path {
					idx = i
					break
				}
			}
			if idx >= 0 {
				m.attachedResources = append(m.attachedResources[:idx], m.attachedResources[idx+1:]...)
			} else {
				m.attachedResources = append(m.attachedResources, path)
			}
		}
	case "c":
		m.attachedResources = nil
		return m, showStatus("Cleared attachments")
	}
	return m, nil
}

// =============================================================================
// Conversations
// =============================================================================

func (m model) updateConversationList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "q":
		m.viewMode = ViewMenu
		return m, nil
	case "up", "k":
		if m.selectedConv > 0 {
			m.selectedConv--
		}
	case "down", "j":
		if m.selectedConv < len(m.conversations)-1 {
			m.selectedConv++
		}
	case "enter":
		if m.selectedConv < len(m.conversations) {
			conv := m.conversations[m.selectedConv]
			loaded, err := storage.LoadConversation(conv.ID)
			if err == nil {
				m.currentConversation = loaded
				m.chatMessages = convMessagesToChat(loaded.Messages)
				m.viewMode = ViewChat
				m.chatState = ChatStateReady
				m.chatTextArea.Focus()
				m.updateChatLines()
				return m, showStatus(fmt.Sprintf("Loaded: %s", conv.Title))
			}
			return m, showStatus(fmt.Sprintf("Failed: %v", err))
		}
	case "d":
		if m.selectedConv < len(m.conversations) {
			conv := m.conversations[m.selectedConv]
			if err := storage.DeleteConversation(conv.ID); err == nil {
				m.conversations, _ = storage.ListConversations()
				if m.selectedConv >= len(m.conversations) && m.selectedConv > 0 {
					m.selectedConv--
				}
				return m, showStatus(fmt.Sprintf("Deleted: %s", conv.Title))
			}
		}
	case "e":
		if m.selectedConv < len(m.conversations) {
			m.viewMode = ViewConversationExport
		}
	}
	return m, nil
}

func (m model) updateConversationExport(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.viewMode = ViewConversationList
		return m, nil
	case "1":
		return m, m.exportConversation("markdown")
	case "2":
		return m, m.exportConversation("json")
	}
	return m, nil
}

// =============================================================================
// Settings
// =============================================================================

func (m model) updateSettings(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.viewMode = ViewMenu
		m.settingsInputs = nil
		return m, nil
	case "enter":
		if len(m.settingsInputs) >= 3 {
			m.settings.MainPrompt = m.settingsInputs[0].Value()
			m.settings.UserName = m.settingsInputs[1].Value()
			if t, err := strconv.Atoi(m.settingsInputs[2].Value()); err == nil && t > 0 {
				m.settings.ChatTimeout = t
			}
			storage.SaveSettings(m.settings)
			m.viewMode = ViewMenu
			m.settingsInputs = nil
			return m, showStatus("Settings saved")
		}
	case "tab":
		m.cycleInputFocus(m.settingsInputs, 1)
	case "shift+tab":
		m.cycleInputFocus(m.settingsInputs, -1)
	default:
		return m.updateFocusedInput(m.settingsInputs, msg)
	}
	return m, nil
}

func (m *model) initSettingsInputs() {
	m.settingsInputs = make([]textinput.Model, 3)

	m.settingsInputs[0] = textinput.New()
	m.settingsInputs[0].SetValue(m.settings.MainPrompt)
	m.settingsInputs[0].CharLimit = 500
	m.settingsInputs[0].Focus()

	m.settingsInputs[1] = textinput.New()
	m.settingsInputs[1].SetValue(m.settings.UserName)
	m.settingsInputs[1].CharLimit = 50

	m.settingsInputs[2] = textinput.New()
	m.settingsInputs[2].SetValue(fmt.Sprintf("%d", m.settings.ChatTimeout))
	m.settingsInputs[2].CharLimit = 10
}

// =============================================================================
// Model Manager
// =============================================================================

func (m model) updateModelManager(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "q":
		m.viewMode = ViewMenu
	case "up", "k":
		if m.modelSelection > 0 {
			m.modelSelection--
		}
	case "down", "j":
		if m.modelSelection < len(m.modelConfig.Profiles)-1 {
			m.modelSelection++
		}
	case "enter":
		m.modelConfig.CurrentProfile = m.modelSelection
		storage.SaveModelConfig(m.modelConfig)
		return m, showStatus(fmt.Sprintf("Default: %s", m.modelConfig.Profiles[m.modelSelection].Name))
	case "b":
		m.viewMode = ViewModelLibrary
		m.libraryModels = ollama.PopularModels()
		m.librarySelection = 0
		m.libraryFilter = ""
		return m, refreshInstalledModels()
	case "n":
		m.viewMode = ViewModelCreate
		m.editingProfile = -1
		m.modelInputs = m.newProfileInputs("", "", "", "0.7")
	case "e":
		if m.modelSelection < len(m.modelConfig.Profiles) {
			p := m.modelConfig.Profiles[m.modelSelection]
			m.viewMode = ViewModelCreate
			m.editingProfile = m.modelSelection
			m.modelInputs = m.newProfileInputs(p.Name, p.Model, p.SystemPrompt, fmt.Sprintf("%.1f", p.Temperature))
		}
	case "p":
		if m.modelSelection < len(m.modelConfig.Profiles) {
			name := m.modelConfig.Profiles[m.modelSelection].Model
			m.viewMode = ViewModelPull
			m.modelPullName = name
			m.modelPullStatus = fmt.Sprintf("Pulling: %s...", name)
			m.modelPullError = nil
			return m, pullOllamaModel(name)
		}
	case "d":
		if len(m.modelConfig.Profiles) > 1 && m.modelSelection < len(m.modelConfig.Profiles) {
			p := m.modelConfig.Profiles[m.modelSelection]
			m.confirmDialog = &ConfirmDialog{
				Action:       ConfirmDeleteModel,
				Message:      fmt.Sprintf("Delete profile '%s' (%s)?", p.Name, p.Model),
				PreviousView: ViewModelManager,
			}
			m.viewMode = ViewConfirmDialog
		} else {
			return m, showStatus("Cannot delete last profile")
		}
	}
	return m, nil
}

func (m model) updateModelCreate(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.viewMode = ViewModelManager
		m.modelInputs = nil
		m.editingProfile = -1
		return m, nil
	case "enter":
		if len(m.modelInputs) >= 4 && m.modelInputs[0].Value() != "" && m.modelInputs[1].Value() != "" {
			temp := 0.7
			if t := m.modelInputs[3].Value(); t != "" {
				if parsed, err := strconv.ParseFloat(t, 64); err == nil && parsed >= 0 && parsed <= 1 {
					temp = parsed
				}
			}
			profile := storage.ModelProfile{
				Name: m.modelInputs[0].Value(), Model: m.modelInputs[1].Value(),
				SystemPrompt: m.modelInputs[2].Value(), Temperature: temp,
			}
			if m.editingProfile >= 0 && m.editingProfile < len(m.modelConfig.Profiles) {
				m.modelConfig.Profiles[m.editingProfile] = profile
			} else {
				m.modelConfig.Profiles = append(m.modelConfig.Profiles, profile)
			}
			storage.SaveModelConfig(m.modelConfig)
			m.viewMode = ViewModelManager
			m.modelInputs = nil
			m.editingProfile = -1
			return m, showStatus(fmt.Sprintf("Profile '%s' saved", profile.Name))
		}
		return m, showStatus("Name and model required")
	case "tab":
		m.cycleInputFocus(m.modelInputs, 1)
	case "shift+tab":
		m.cycleInputFocus(m.modelInputs, -1)
	default:
		return m.updateFocusedInput(m.modelInputs, msg)
	}
	return m, nil
}

func (m model) updateModelPull(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.String() == "esc" {
		m.viewMode = ViewModelManager
		m.modelPullStatus = ""
		m.modelPullError = nil
	}
	return m, nil
}

func (m model) updateModelLibrary(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "q":
		m.viewMode = ViewModelManager
		m.libraryFilter = ""
		return m, nil
	case "up", "k":
		if m.librarySelection > 0 {
			m.librarySelection--
		}
	case "down", "j":
		maxSel := len(m.getFilteredLibrary()) - 1
		if m.librarySelection < maxSel {
			m.librarySelection++
		}
	case "backspace":
		if len(m.libraryFilter) > 0 {
			m.libraryFilter = m.libraryFilter[:len(m.libraryFilter)-1]
			m.librarySelection = 0
		}
	case "r":
		return m, refreshInstalledModels()
	case "enter":
		filtered := m.getFilteredLibrary()
		if m.librarySelection < len(filtered) {
			name := filtered[m.librarySelection].Name
			if ollama.IsModelInstalled(name, m.installedModels) {
				return m, showStatus(fmt.Sprintf("%s already installed", name))
			}
			m.viewMode = ViewModelPull
			m.modelPullName = name
			m.modelPullStatus = fmt.Sprintf("Installing %s...", name)
			m.modelPullError = nil
			return m, pullOllamaModel(name)
		}
	default:
		key := msg.String()
		if len(key) == 1 {
			m.libraryFilter += key
			m.librarySelection = 0
		}
	}
	return m, nil
}

func (m model) updateConfirmDialog(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.confirmDialog == nil {
		return m, nil
	}
	switch msg.String() {
	case "y", "Y":
		switch m.confirmDialog.Action {
		case ConfirmDeleteModel:
			return m.executeDeleteModel()
		}
	case "n", "N", "esc":
		prev := m.confirmDialog.PreviousView
		m.confirmDialog = nil
		m.viewMode = prev
	}
	return m, nil
}

func (m model) executeDeleteModel() (tea.Model, tea.Cmd) {
	if len(m.modelConfig.Profiles) <= 1 || m.modelSelection >= len(m.modelConfig.Profiles) {
		m.confirmDialog = nil
		m.viewMode = ViewModelManager
		return m, showStatus("Cannot delete last profile")
	}
	name := m.modelConfig.Profiles[m.modelSelection].Name
	m.modelConfig.Profiles = append(m.modelConfig.Profiles[:m.modelSelection], m.modelConfig.Profiles[m.modelSelection+1:]...)
	if m.modelConfig.CurrentProfile >= len(m.modelConfig.Profiles) {
		m.modelConfig.CurrentProfile = len(m.modelConfig.Profiles) - 1
	}
	if m.modelSelection >= len(m.modelConfig.Profiles) {
		m.modelSelection = len(m.modelConfig.Profiles) - 1
	}
	storage.SaveModelConfig(m.modelConfig)
	m.confirmDialog = nil
	m.viewMode = ViewModelManager
	return m, showStatus(fmt.Sprintf("Deleted: %s", name))
}

// =============================================================================
// Shared input helpers
// =============================================================================

func (m *model) cycleInputFocus(inputs []textinput.Model, dir int) {
	if len(inputs) == 0 {
		return
	}
	current := -1
	for i, inp := range inputs {
		if inp.Focused() {
			current = i
			break
		}
	}
	if current >= 0 {
		inputs[current].Blur()
		next := (current + dir + len(inputs)) % len(inputs)
		inputs[next].Focus()
	}
}

func (m model) updateFocusedInput(inputs []textinput.Model, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	for i := range inputs {
		if inputs[i].Focused() {
			var cmd tea.Cmd
			inputs[i], cmd = inputs[i].Update(msg)
			return m, cmd
		}
	}
	return m, nil
}

func (m *model) newProfileInputs(name, modelName, prompt, temp string) []textinput.Model {
	inputs := make([]textinput.Model, 4)
	inputs[0] = textinput.New()
	inputs[0].SetValue(name)
	inputs[0].Placeholder = "My Assistant"
	inputs[0].Focus()
	inputs[1] = textinput.New()
	inputs[1].SetValue(modelName)
	inputs[1].Placeholder = "llama3.2:3b"
	inputs[2] = textinput.New()
	inputs[2].SetValue(prompt)
	inputs[2].Placeholder = "You are a helpful assistant..."
	inputs[2].CharLimit = 500
	inputs[3] = textinput.New()
	inputs[3].SetValue(temp)
	inputs[3].Placeholder = "0.7"
	inputs[3].CharLimit = 3
	return inputs
}

// =============================================================================
// Layout
// =============================================================================

func (m *model) adjustLayout() {
	w := m.safeWidth()
	h := m.safeHeight()

	// Chat area sizing
	var chatOverhead int
	if h < 20 {
		chatOverhead = 8
	} else if h < 30 {
		chatOverhead = 12
	} else {
		chatOverhead = 15
	}
	m.chatMaxLines = h - chatOverhead
	if m.chatMaxLines < 5 {
		m.chatMaxLines = 5
	}

	// Textarea sizing
	taWidth := w - 4
	if taWidth < 20 {
		taWidth = 20
	}
	m.chatTextArea.SetWidth(taWidth)

	var taHeight int
	switch {
	case h < 15:
		taHeight = 2
	case h < 40:
		taHeight = 3
	default:
		taHeight = 4
	}
	m.chatTextArea.SetHeight(taHeight)
}

// =============================================================================
// Ollama commands (BubbleTea wrappers)
// =============================================================================

func (m *model) checkModel() tea.Cmd {
	return func() tea.Msg {
		name := m.currentProfile().Model
		avail, err := ollama.CheckModel(name)
		if err != nil {
			return CheckModelMsg{Available: false, ModelName: name, Err: err}
		}
		return CheckModelMsg{Available: avail, ModelName: name}
	}
}

func (m *model) pullModel() tea.Cmd {
	return func() tea.Msg {
		name := m.currentProfile().Model
		if err := ollama.PullModel(name); err != nil {
			return CheckModelMsg{Available: false, ModelName: name, Err: err}
		}
		return CheckModelMsg{Available: true, ModelName: name}
	}
}

func (m *model) sendChat(userMsg string) tea.Cmd {
	ctx, cancel := context.WithCancel(context.Background())
	m.cancelChat = cancel

	profile := m.currentProfile()

	// Build messages
	var msgs []ollama.ChatMessage
	systemPrompt := profile.SystemPrompt
	if m.settings.MainPrompt != "" {
		systemPrompt = m.settings.MainPrompt + "\n\n" + systemPrompt
	}

	// Attach RAG resources to system prompt
	if len(m.attachedResources) > 0 {
		var rag strings.Builder
		rag.WriteString("\n\n=== ATTACHED RESOURCES ===\n\n")
		for _, path := range m.attachedResources {
			if data, err := readFileContent(path); err == nil {
				fmt.Fprintf(&rag, "--- %s ---\n%s\n\n", filepath.Base(path), data)
			}
		}
		rag.WriteString("=== END RESOURCES ===\nUse these for context.")
		systemPrompt += rag.String()
	}

	if systemPrompt != "" {
		msgs = append(msgs, ollama.ChatMessage{Role: "system", Content: systemPrompt})
	}
	baseDir := m.currentDir
	for _, msg := range m.chatMessages[:len(m.chatMessages)-1] {
		if msg.Role == "user" || msg.Role == "assistant" {
			content := msg.Content
			if msg.Role == "user" {
				content = resolveAtReferences(content, baseDir)
			}
			msgs = append(msgs, ollama.ChatMessage{Role: msg.Role, Content: content})
		}
	}
	msgs = append(msgs, ollama.ChatMessage{Role: "user", Content: resolveAtReferences(userMsg, baseDir)})

	req := ollama.ChatRequest{
		Model: profile.Model, Messages: msgs,
		Temperature: profile.Temperature,
		Timeout:     time.Duration(m.settings.ChatTimeout) * time.Second,
	}

	return func() tea.Msg {
		ch, err := ollama.ChatStream(ctx, req)
		if err != nil {
			if ctx.Err() != nil {
				return InterruptMsg{}
			}
			return ResponseMsg{Err: err}
		}
		return streamStartedMsg{ch: ch}
	}
}

func pullOllamaModel(name string) tea.Cmd {
	return func() tea.Msg {
		if err := ollama.PullModel(name); err != nil {
			return ModelPullMsg{Err: err}
		}
		return ModelPullMsg{Success: true}
	}
}

func refreshInstalledModels() tea.Cmd {
	return func() tea.Msg {
		models, err := ollama.ListModels()
		if err != nil {
			return statusMsg{message: fmt.Sprintf("Failed to refresh: %v", err)}
		}
		return installedModelsMsg{models: models}
	}
}

func listenForChunk(ch <-chan ollama.StreamChunk) tea.Cmd {
	return func() tea.Msg {
		chunk, ok := <-ch
		if !ok {
			return StreamChunkMsg{Done: true}
		}
		return StreamChunkMsg{
			Content:      chunk.Content,
			Done:         chunk.Done,
			Err:          chunk.Err,
			Duration:     chunk.Duration,
			PromptTokens: chunk.PromptTokens,
			TotalTokens:  chunk.TotalTokens,
		}
	}
}
