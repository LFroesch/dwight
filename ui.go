package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func (m model) Init() tea.Cmd {
	return tickCmd()
}

func (m model) updateModelManager(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "q":
		m.viewMode = ViewMenu
		return m, nil
	case "up", "k":
		if m.modelSelection > 0 {
			m.modelSelection--
		}
	case "down", "j":
		if m.modelSelection < len(m.modelConfig.Profiles)-1 {
			m.modelSelection++
		}
	case "e":
		if m.modelSelection < len(m.modelConfig.Profiles) {
			profile := m.modelConfig.Profiles[m.modelSelection]
			m.viewMode = ViewModelCreate // Reuse the same view
			m.modelInputs = make([]textinput.Model, 4)

			// Profile name
			m.modelInputs[0] = textinput.New()
			m.modelInputs[0].SetValue(profile.Name)
			m.modelInputs[0].CharLimit = 50

			// Model name
			m.modelInputs[1] = textinput.New()
			m.modelInputs[1].SetValue(profile.Model)
			m.modelInputs[1].CharLimit = 50

			// System prompt
			m.modelInputs[2] = textinput.New()
			m.modelInputs[2].SetValue(profile.SystemPrompt)
			m.modelInputs[2].CharLimit = 500

			// Temperature
			m.modelInputs[3] = textinput.New()
			m.modelInputs[3].SetValue(fmt.Sprintf("%.1f", profile.Temperature))
			m.modelInputs[3].CharLimit = 3

			// Focus the first input
			m.modelInputs[0].Focus()

			// Store that we're editing, not creating
			m.editField = m.modelSelection // Reuse this field to track which profile we're editing

			return m, nil
		}
	case "enter":
		m.modelConfig.CurrentProfile = m.modelSelection
		m.saveModelConfig()
		return m, showStatus(fmt.Sprintf("✅ Default profile set to: %s", m.modelConfig.Profiles[m.modelSelection].Name))
	case "b":
		// Browse Ollama model library
		m.viewMode = ViewModelLibrary
		m.libraryModels = getPopularModels()
		m.librarySelection = 0
		m.libraryFilter = ""
		// Load installed models
		return m, refreshInstalledModels()

	case "n":
		m.viewMode = ViewModelCreate
		m.modelInputs = make([]textinput.Model, 4)

		// Profile name
		m.modelInputs[0] = textinput.New()
		m.modelInputs[0].Placeholder = "My Custom Assistant"
		m.modelInputs[0].Focus()

		// Model name
		m.modelInputs[1] = textinput.New()
		m.modelInputs[1].Placeholder = "llama3.2:3b"

		// System prompt
		m.modelInputs[2] = textinput.New()
		m.modelInputs[2].Placeholder = "You are a helpful assistant..."
		m.modelInputs[2].CharLimit = 500

		// Temperature
		m.modelInputs[3] = textinput.New()
		m.modelInputs[3].Placeholder = "0.7"
		m.modelInputs[3].CharLimit = 3

		return m, nil
	case "p":
		// Pull the model from the currently selected profile
		if m.modelSelection < len(m.modelConfig.Profiles) {
			selectedModel := m.modelConfig.Profiles[m.modelSelection].Model
			m.viewMode = ViewModelPull
			m.modelPullName = selectedModel
			m.modelPullStatus = fmt.Sprintf("🔄 Pulling model: %s...", selectedModel)
			m.modelPullError = nil
			return m, pullOllamaModel(selectedModel)
		}
		return m, nil
	case "d":
		if len(m.modelConfig.Profiles) > 1 && m.modelSelection < len(m.modelConfig.Profiles) {
			profile := m.modelConfig.Profiles[m.modelSelection]
			m.confirmDialog = &ConfirmDialog{
				Action:       ConfirmDeleteModel,
				Message:      fmt.Sprintf("Are you sure you want to delete model profile '%s'?\n\nModel: %s\nThis action cannot be undone.", profile.Name, profile.Model),
				PreviousView: ViewModelManager,
			}
			m.viewMode = ViewConfirmDialog
		} else {
			return m, showStatus("❌ Cannot delete last profile")
		}
		return m, nil
	}
	return m, nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case statusMsg:
		m.statusMsg = msg.message
		m.statusExpiry = time.Now().Add(3 * time.Second)
		return m, nil

	case tickMsg:
		m.lastUpdate = time.Time(msg)
		return m, tickCmd()

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.adjustLayout()
		// If we're in chat mode, refresh the chat content with new dimensions
		if m.viewMode == ViewChat {
			m.updateChatLines()
		}
		return m, nil

	case tea.KeyMsg:
		if m.editMode {
			return m.updateEdit(msg)
		}

		switch m.viewMode {
		case ViewMenu:
			return m.updateMenu(msg)
		case ViewResourceManager:
			return m.updateResourceManager(msg)
		case ViewDetails:
			return m.updateDetails(msg)
		case ViewCreate:
			return m.updateCreate(msg)
		case ViewGlobalResources, ViewCleanup, ViewCleanupChats:
			return m.updatePlaceholder(msg)
		case ViewChat:
			return m.updateChatEnhanced(msg)
		case ViewModelManager:
			return m.updateModelManager(msg)
		case ViewModelCreate:
			return m.updateModelCreate(msg)
		case ViewModelPull:
			return m.updateModelPull(msg)
		case ViewSettings:
			return m.updateSettings(msg)
		case ViewConfirmDialog:
			return m.updateConfirmDialog(msg)
		case ViewConversationList:
			return m.updateConversationList(msg)
		case ViewConversationExport:
			return m.updateConversationExport(msg)
		case ViewModelLibrary:
			return m.updateModelLibrary(msg)
		}

	case installedModelsMsg:
		m.installedModels = msg.models
		return m, showStatus(fmt.Sprintf("✅ Refreshed: %d models installed", len(msg.models)))

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
			// Model is not available, ask user if they want to pull it
			m.chatState = ChatStateModelNotAvailable
			m.modelPullName = msg.ModelName
		} else {
			m.chatState = ChatStateReady
			m.chatTextArea.Focus()
		}
		return m, nil

	case StreamChunkMsg:
		if msg.Err != nil {
			m.chatErr = msg.Err
			m.chatState = ChatStateError
			m.chatStreaming = false
			m.chatStreamBuffer.Reset()
			m.updateChatLines()
			return m, nil
		}

		if msg.Done {
			// Finalize the streaming message with metadata
			if m.chatStreamBuffer.Len() > 0 {
				m.chatMessages = append(m.chatMessages, ChatMessage{
					Role:         "assistant",
					Content:      m.chatStreamBuffer.String(),
					Duration:     msg.Duration,
					PromptTokens: msg.PromptTokens,
					TotalTokens:  msg.TotalTokens,
				})
				m.chatStreamBuffer.Reset()
			}
			m.chatState = ChatStateReady
			m.chatStreaming = false
			m.chatTextArea.Focus()
			m.updateChatLines()
			return m, nil
		}

		// Append chunk to buffer and update display
		m.chatStreamBuffer.WriteString(msg.Content)
		m.updateChatLines()

		// Continue listening for next chunk - this is key!
		// The waitForStreamChunk function will return the next message from the channel
		return m, nil

	case ResponseMsg:
		if msg.Err != nil {
			m.chatErr = msg.Err
			m.chatState = ChatStateError
		} else {
			m.chatMessages = append(m.chatMessages, ChatMessage{
				Role:         "assistant",
				Content:      msg.Content,
				Timestamp:    time.Now(),
				Duration:     msg.Duration,
				PromptTokens: msg.PromptTokens,
				TotalTokens:  msg.TotalTokens,
			})
			m.chatState = ChatStateReady
			m.chatTextArea.Focus()
			m.updateChatLines()
		}
		return m, nil

	case ClearChatMsg:
		m.chatMessages = []ChatMessage{}
		m.chatStreamBuffer.Reset()
		m.chatStreaming = false
		m.updateChatLines()
		return m, showStatus("💬 Chat cleared")

	case ModelPullMsg:
		if msg.Err != nil {
			m.modelPullError = msg.Err
			m.modelPullStatus = ""
		} else {
			m.modelPullStatus = fmt.Sprintf("✅ Successfully pulled model: %s", m.modelPullName)
		}
		return m, nil
	}

	return m, nil
}

func (m model) updateSettings(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.viewMode = ViewMenu
		m.settingsInputs = nil
		return m, nil
	case "enter":
		// Save settings
		if len(m.settingsInputs) >= 4 {
			m.appSettings.MainPrompt = m.settingsInputs[0].Value()
			m.appSettings.MemoryAllotment = m.settingsInputs[1].Value()
			m.appSettings.UserName = m.settingsInputs[2].Value()

			if timeout := m.settingsInputs[3].Value(); timeout != "" {
				if t, err := strconv.Atoi(timeout); err == nil && t > 0 {
					m.appSettings.ChatTimeout = t
				}
			}

			m.saveSettings()
			m.viewMode = ViewMenu
			m.settingsInputs = nil
			return m, showStatus("✅ Settings saved")
		}
		return m, nil
	case "tab":
		if len(m.settingsInputs) > 0 {
			currentField := -1
			for i, input := range m.settingsInputs {
				if input.Focused() {
					currentField = i
					break
				}
			}
			if currentField >= 0 {
				m.settingsInputs[currentField].Blur()
				nextField := (currentField + 1) % len(m.settingsInputs)
				m.settingsInputs[nextField].Focus()
			}
		}
		return m, nil
	case "shift+tab":
		if len(m.settingsInputs) > 0 {
			currentField := -1
			for i, input := range m.settingsInputs {
				if input.Focused() {
					currentField = i
					break
				}
			}
			if currentField >= 0 {
				m.settingsInputs[currentField].Blur()
				nextField := (currentField - 1 + len(m.settingsInputs)) % len(m.settingsInputs)
				m.settingsInputs[nextField].Focus()
			}
		}
		return m, nil
	}

	// Update the focused input
	for i := range m.settingsInputs {
		if m.settingsInputs[i].Focused() {
			var cmd tea.Cmd
			m.settingsInputs[i], cmd = m.settingsInputs[i].Update(msg)
			return m, cmd
		}
	}
	return m, nil
}

func (m model) updateModelCreate(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.viewMode = ViewModelManager
		m.modelInputs = nil
		m.editField = -1
		return m, nil
	case "enter":
		// Validate and save profile
		if len(m.modelInputs) >= 4 && m.modelInputs[0].Value() != "" && m.modelInputs[1].Value() != "" {
			temp := 0.7
			if t := m.modelInputs[3].Value(); t != "" {
				if parsed, err := strconv.ParseFloat(t, 64); err == nil && parsed >= 0 && parsed <= 1 {
					temp = parsed
				}
			}

			newProfile := ModelProfile{
				Name:         m.modelInputs[0].Value(),
				Model:        m.modelInputs[1].Value(),
				SystemPrompt: m.modelInputs[2].Value(),
				Temperature:  temp,
			}

			// Check if we're editing or creating
			if m.editField >= 0 && m.editField < len(m.modelConfig.Profiles) {
				// Editing existing profile
				m.modelConfig.Profiles[m.editField] = newProfile
				statusMsg := fmt.Sprintf("✅ Profile '%s' updated", newProfile.Name)
				m.editField = -1 // Reset edit field
				m.saveModelConfig()
				m.viewMode = ViewModelManager
				m.modelInputs = nil
				return m, showStatus(statusMsg)
			} else {
				// Creating new profile
				m.modelConfig.Profiles = append(m.modelConfig.Profiles, newProfile)
				m.saveModelConfig()
				m.viewMode = ViewModelManager
				m.modelInputs = nil
				return m, showStatus(fmt.Sprintf("✅ Profile '%s' created", newProfile.Name))
			}
		}
		return m, showStatus("❌ Please fill in at least name and model")
	case "tab":
		if len(m.modelInputs) > 0 {
			currentField := -1
			for i, input := range m.modelInputs {
				if input.Focused() {
					currentField = i
					break
				}
			}
			if currentField >= 0 {
				m.modelInputs[currentField].Blur()
				nextField := (currentField + 1) % len(m.modelInputs)
				m.modelInputs[nextField].Focus()
			}
		}
		return m, nil
	case "shift+tab":
		if len(m.modelInputs) > 0 {
			currentField := -1
			for i, input := range m.modelInputs {
				if input.Focused() {
					currentField = i
					break
				}
			}
			if currentField >= 0 {
				m.modelInputs[currentField].Blur()
				nextField := (currentField - 1 + len(m.modelInputs)) % len(m.modelInputs)
				m.modelInputs[nextField].Focus()
			}
		}
	}

	// Update the focused input
	for i := range m.modelInputs {
		if m.modelInputs[i].Focused() {
			var cmd tea.Cmd
			m.modelInputs[i], cmd = m.modelInputs[i].Update(msg)
			return m, cmd
		}
	}
	return m, nil
}

func (m model) updateModelPull(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.viewMode = ViewModelManager
		m.modelPullStatus = ""
		m.modelPullError = nil
		return m, nil
	}
	return m, nil
}

func (m model) updateMenu(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		// Stop Ollama container to free memory when exiting app
		go stopOllamaContainer()
		return m, tea.Quit
	case "enter", " ":
		cursor := m.menuTable.Cursor()
		switch cursor {
		case 0:
			m.viewMode = ViewResourceManager
		case 1:
			// Chat with Ollama
			m.viewMode = ViewChat
			m.chatState = ChatStateCheckingModel
			m.chatTextArea.Focus()
			m.chatMessages = []ChatMessage{} // Clear previous messages
			m.chatStreamBuffer.Reset()
			m.chatStreaming = false
			m.updateChatLines()              // Initialize text viewer with empty content
			return m, tea.Batch(
				m.checkOllamaModel(),
				m.chatSpinner.Tick,
			)
		case 2:
			// Conversation History
			m.viewMode = ViewConversationList
			convs, err := m.listConversations()
			if err == nil {
				m.conversations = convs
				m.selectedConv = 0
			}
			return m, nil
		case 3:
			m.viewMode = ViewGlobalResources
			m.scanGlobalResources()
		case 4:
			m.viewMode = ViewSettings
			// Initialize settings inputs
			m.settingsInputs = make([]textinput.Model, 4)

			// Main Prompt
			m.settingsInputs[0] = textinput.New()
			m.settingsInputs[0].SetValue(m.appSettings.MainPrompt)
			m.settingsInputs[0].CharLimit = 500
			m.settingsInputs[0].Focus()

			// Memory Allotment
			m.settingsInputs[1] = textinput.New()
			m.settingsInputs[1].SetValue(m.appSettings.MemoryAllotment)
			m.settingsInputs[1].CharLimit = 20

			// User Name
			m.settingsInputs[2] = textinput.New()
			m.settingsInputs[2].SetValue(m.appSettings.UserName)
			m.settingsInputs[2].CharLimit = 50

			// Chat Timeout
			m.settingsInputs[3] = textinput.New()
			m.settingsInputs[3].SetValue(fmt.Sprintf("%d", m.appSettings.ChatTimeout))
			m.settingsInputs[3].CharLimit = 10
		case 5:
			// Stop Ollama container
			go stopOllamaContainer()
			return m, showStatus("🛑 Ollama container stopped to free memory")
		case 6:
			m.viewMode = ViewCleanup
		case 7:
			m.viewMode = ViewCleanupChats
		case 8:
			m.viewMode = ViewModelManager
			m.modelSelection = m.modelConfig.CurrentProfile
		case 9:
			return m, tea.Quit
		}
		return m, nil
	}

	var cmd tea.Cmd
	m.menuTable, cmd = m.menuTable.Update(msg)
	return m, cmd
}

// In ui.go, replace the updateChat function with this:

func (m model) updateChat(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg.String() {
	case "esc":
		if len(m.chatMessages) > 0 {
			if err := m.saveChatLog(); err == nil {
				// Don't show error, just silently fail
			}
		}
		m.viewMode = ViewMenu
		m.chatTextArea.Blur()
		m.chatMessages = []ChatMessage{}
		m.chatState = ChatStateInit
		m.chatStreamBuffer.Reset()
		m.chatStreaming = false
		// Stop Ollama container to free memory when exiting chat
		go stopOllamaContainer()
		return m, nil

	case "ctrl+l":
		// Clear chat without exiting
		if m.chatState == ChatStateReady {
			return m, func() tea.Msg { return ClearChatMsg{} }
		}

	case "ctrl+s":
		// Save chat log manually
		if len(m.chatMessages) > 0 {
			if err := m.saveChatLog(); err == nil {
				return m, showStatus("💾 Chat saved")
			} else {
				return m, showStatus("❌ Failed to save chat")
			}
		}

	case "tab":
		if m.chatState == ChatStateReady && !m.chatStreaming {
			m.modelConfig.CurrentProfile = (m.modelConfig.CurrentProfile + 1) % len(m.modelConfig.Profiles)
			m.saveModelConfig()
			return m, showStatus(fmt.Sprintf("🔄 Switched to %s", m.modelConfig.Profiles[m.modelConfig.CurrentProfile].Name))
		}

	case "shift+tab":
		if m.chatState == ChatStateReady && !m.chatStreaming {
			m.modelConfig.CurrentProfile = (m.modelConfig.CurrentProfile - 1 + len(m.modelConfig.Profiles)) % len(m.modelConfig.Profiles)
			m.saveModelConfig()
			return m, showStatus(fmt.Sprintf("🔄 Switched to %s", m.modelConfig.Profiles[m.modelConfig.CurrentProfile].Name))
		}
	case "y", "Y":
		if m.chatState == ChatStateModelNotAvailable {
			// User wants to pull the model
			m.chatState = ChatStateCheckingModel
			m.updateChatLines()
			return m, tea.Batch(
				m.pullModel(),
				m.chatSpinner.Tick,
			)
		}
	case "n", "N":
		if m.chatState == ChatStateModelNotAvailable {
			// User declined to pull the model, go back to menu
			m.viewMode = ViewMenu
			m.chatTextArea.Blur()
			m.chatMessages = []ChatMessage{}
			m.chatState = ChatStateInit
			return m, nil
		}
	case "enter":
		if m.chatState == ChatStateReady && strings.TrimSpace(m.chatTextArea.Value()) != "" {
			// Send message
			userMsg := strings.TrimSpace(m.chatTextArea.Value())
			m.chatMessages = append(m.chatMessages, ChatMessage{
				Role:      "user",
				Content:   userMsg,
				Timestamp: time.Now(),
			})
			m.chatTextArea.Reset()
			m.chatState = ChatStateLoading
			m.updateChatLines()
			return m, tea.Batch(
				sendChatMessage(userMsg, m.getCurrentProfile(), m.appSettings, m.chatMessages[:len(m.chatMessages)-1], m.attachedResources),
				m.chatSpinner.Tick,
			)
		}

	case "up", "down", "pgup", "pgdown", "home", "end":
		// Handle scrolling with the new clean function
		m.handleChatScroll(msg.String())
		return m, nil
	}

	// Update textarea when in ready state and not streaming
	if m.chatState == ChatStateReady && !m.chatStreaming {
		m.chatTextArea, cmd = m.chatTextArea.Update(msg)
	}

	return m, cmd
}

func (m model) updatePlaceholder(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	// Handle ViewCleanupChats separately first
	if m.viewMode == ViewCleanupChats {
		switch msg.String() {
		case "esc", "q":
			m.viewMode = ViewMenu
			return m, nil
		case "1":
			if count, err := cleanupOldChats(m.currentDir, 1); err == nil {
				m.viewMode = ViewMenu
				return m, showStatus(fmt.Sprintf("🗑️ Deleted %d chat logs older than 1 day", count))
			}
		case "7":
			if count, err := cleanupOldChats(m.currentDir, 7); err == nil {
				m.viewMode = ViewMenu
				return m, showStatus(fmt.Sprintf("🗑️ Deleted %d chat logs older than 7 days", count))
			}
		case "3":
			if count, err := cleanupOldChats(m.currentDir, 30); err == nil {
				m.viewMode = ViewMenu
				return m, showStatus(fmt.Sprintf("🗑️ Deleted %d chat logs older than 30 days", count))
			}
		case "9":
			if count, err := cleanupOldChats(m.currentDir, 90); err == nil {
				m.viewMode = ViewMenu
				return m, showStatus(fmt.Sprintf("🗑️ Deleted %d chat logs older than 90 days", count))
			}
		case "a":
			if count, err := cleanupOldChats(m.currentDir, 0); err == nil {
				m.viewMode = ViewMenu
				return m, showStatus(fmt.Sprintf("🗑️ Deleted all %d chat logs", count))
			}
		}
		return m, nil
	}

	// Handle other placeholder views
	switch msg.String() {
	case "esc", "q":
		m.viewMode = ViewMenu
		return m, nil
	case "up", "k":
		if m.viewMode == ViewGlobalResources && len(m.globalRes) > 0 {
			if m.cursor > 0 {
				m.cursor--
			}
		}
	case "down", "j":
		if m.viewMode == ViewGlobalResources && len(m.globalRes) > 0 {
			if m.cursor < len(m.globalRes)-1 {
				m.cursor++
			}
		}
	case "enter", " ", "v":
		if m.viewMode == ViewGlobalResources && len(m.globalRes) > 0 && m.cursor < len(m.globalRes) {
			m.viewMode = ViewDetails
			m.selectedRes = &m.globalRes[m.cursor]
			m.fromGlobal = true
			if m.selectedRes != nil {
				content, err := os.ReadFile(m.selectedRes.Path)
				if err == nil {
					m.updateFileLines(string(content))
				}
			}
		}
	case "e":
		if m.viewMode == ViewGlobalResources {
			return m, nil
		}
	case "r":
		if m.viewMode == ViewGlobalResources {
			m.scanGlobalResources()
			return m, showStatus("🔄 Global resources refreshed")
		}
	case "p":
		// Pull from global to local
		if m.viewMode == ViewGlobalResources && len(m.globalRes) > 0 && m.cursor < len(m.globalRes) {
			res := &m.globalRes[m.cursor]
			m.confirmDialog = &ConfirmDialog{
				Action:      ConfirmPull,
				Message:     fmt.Sprintf("Pull '%s' from global to project templates?\n\nThis will copy the file to your project's templates directory.", res.Name),
				Resource:    res,
				PreviousView: ViewGlobalResources,
			}
			m.viewMode = ViewConfirmDialog
		}
		return m, nil
	}

	return m, cmd
}

func (m model) updateEdit(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg.String() {
	case "esc":
		m.editMode = false
		m.inputs = nil
		if m.editField == 2 {
			m.textInput.Blur()
		}
		m.editField = 0
		return m, showStatus("❌ Edit cancelled")
	case "enter":
		if m.editField == 2 && len(m.inputs) == 0 {
			m.filterTag = m.textInput.Value()
			m.applyFilter()
			m.updateTableData()
			m.editMode = false
			m.editField = 0
			m.textInput.Blur()
			return m, showStatus("✅ Search applied")
		} else if m.selectedRes != nil && len(m.inputs) > 0 {
			m.saveEdit()
			m.editMode = false
			m.inputs = nil
			return m, showStatus("✅ Changes saved")
		}
		return m, nil
	case "tab":
		if len(m.inputs) > 0 {
			m.editField = (m.editField + 1) % len(m.inputs)
			for i := range m.inputs {
				m.inputs[i].Blur()
			}
			m.inputs[m.editField].Focus()
		}
		return m, nil
	case "shift+tab":
		if len(m.inputs) > 0 {
			m.editField = (m.editField - 1 + len(m.inputs)) % len(m.inputs)
			for i := range m.inputs {
				m.inputs[i].Blur()
			}
			m.inputs[m.editField].Focus()
		}
		return m, nil
	default:
		if len(m.inputs) > 0 {
			m.inputs[m.editField], cmd = m.inputs[m.editField].Update(msg)
			return m, cmd
		} else if m.editField == 2 && len(m.inputs) == 0 {
			m.textInput, cmd = m.textInput.Update(msg)
			return m, cmd
		}
	}

	return m, nil
}

func (m model) updateResourceManager(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg.String() {
	case "q", "ctrl+c":
		// Stop Ollama container to free memory when exiting app
		go stopOllamaContainer()
		m.viewMode = ViewMenu
		return m, nil
	case "esc":
		if m.filterTag != "" {
			m.filterTag = ""
			m.applyFilter()
			m.updateTableData()
			return m, showStatus("🔄 Search cleared")
		}
		m.viewMode = ViewMenu
		return m, nil
	case "?":
		m.showHelp = !m.showHelp
		return m, nil
	case "r":
		m.scanResources()
		m.updateTableData()
		return m, showStatus("🔄 Resources refreshed")
	case "n", "a":
		m.viewMode = ViewCreate
		m.textInput.SetValue("")
		m.textInput.Focus()
		return m, nil
	case "e":
		m.selectedRes = m.getSelectedResource()
		if m.selectedRes != nil {
			m.startEditing()
		}
		return m, nil
	case "f":
		m.editMode = true
		m.editField = 2
		m.textInput.SetValue(m.filterTag)
		m.textInput.Focus()
		return m, nil
	case "i", "enter", " ", "v":
		m.viewMode = ViewDetails
		m.selectedRes = m.getSelectedResource()
		m.fromGlobal = false
		if m.selectedRes != nil {
			content, err := os.ReadFile(m.selectedRes.Path)
			if err == nil {
				m.updateFileLines(string(content))
			}
		}
		return m, nil
	case "d":
		res := m.getSelectedResource()
		if res != nil {
			m.confirmDialog = &ConfirmDialog{
				Action:      ConfirmDelete,
				Message:     fmt.Sprintf("Are you sure you want to delete '%s'?\n\nThis action cannot be undone.", res.Name),
				Resource:    res,
				PreviousView: ViewResourceManager,
			}
			m.viewMode = ViewConfirmDialog
		}
		return m, nil
	case "p":
		// Push to global
		res := m.getSelectedResource()
		if res != nil {
			m.confirmDialog = &ConfirmDialog{
				Action:      ConfirmPush,
				Message:     fmt.Sprintf("Push '%s' to global templates directory?\n\nThis will copy the file to ~/.local/share/dwight/templates/", res.Name),
				Resource:    res,
				PreviousView: ViewResourceManager,
			}
			m.viewMode = ViewConfirmDialog
		}
		return m, nil
	// Add sorting keys
	case "s":
		// Cycle through sort options
		switch m.sortBy {
		case "name":
			m.sortBy = "type"
		case "type":
			m.sortBy = "size"
		case "size":
			m.sortBy = "modified"
		case "modified":
			m.sortBy = "name"
		default:
			m.sortBy = "name"
		}
		m.applyFilter() // This will re-sort
		m.updateTableData()
		return m, showStatus(fmt.Sprintf("🔄 Sorted by %s", m.sortBy))
	case "S":
		// Toggle sort direction
		m.sortDesc = !m.sortDesc
		m.applyFilter() // This will re-sort
		m.updateTableData()
		direction := "ascending"
		if m.sortDesc {
			direction = "descending"
		}
		return m, showStatus(fmt.Sprintf("🔄 Sort direction: %s", direction))
	}

	m.table, cmd = m.table.Update(msg)
	return m, cmd
}

func (m model) updateDetails(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "q":
		if m.fromGlobal {
			m.viewMode = ViewGlobalResources
			m.fromGlobal = false
		} else {
			m.viewMode = ViewResourceManager
		}
		return m, nil
	case "e":
		if m.selectedRes != nil && !m.fromGlobal {
			m.startEditing()
		}
		return m, nil
	default:
		// Handle scrolling with the new clean function
		m.handleFileScroll(msg.String())
	}

	return m, nil
}

func (m model) updateCreate(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg.String() {
	case "esc":
		m.viewMode = ViewResourceManager
		m.textInput.Blur()
		return m, nil
	case "enter":
		filename := m.textInput.Value()
		if filename != "" {
			filePath := filepath.Join(m.currentDir, filename)
			os.WriteFile(filePath, []byte("# New AI Resource\n\n"), 0644)
			m.scanResources()
			m.updateTableData()
			m.viewMode = ViewResourceManager
			m.textInput.Blur()
			return m, showStatus("📄 Resource created")
		}
		return m, nil
	}

	m.textInput, cmd = m.textInput.Update(msg)
	return m, cmd
}

func (m *model) startEditing() {
	m.editMode = true
	m.editField = 0

	m.inputs = make([]textinput.Model, 2)

	m.inputs[0] = textinput.New()
	m.inputs[0].SetValue(m.selectedRes.Description)
	m.inputs[0].Focus()
	m.inputs[0].CharLimit = 200

	m.inputs[1] = textinput.New()
	m.inputs[1].SetValue(strings.Join(m.selectedRes.Tags, ", "))
	m.inputs[1].CharLimit = 100
}

func (m *model) saveEdit() {
	if m.selectedRes == nil || len(m.inputs) < 2 {
		return
	}

	m.selectedRes.Description = m.inputs[0].Value()

	tagString := m.inputs[1].Value()
	if tagString != "" {
		m.selectedRes.Tags = strings.Split(tagString, ",")
		for i, tag := range m.selectedRes.Tags {
			m.selectedRes.Tags[i] = strings.TrimSpace(tag)
		}
	} else {
		m.selectedRes.Tags = []string{}
	}

	m.saveResourceMetadata(m.selectedRes)

	m.scanResources()
	m.updateTableData()
}

func (m *model) updateTableData() {
	var rows []table.Row

	for _, res := range m.filteredRes {
		tags := strings.Join(res.Tags, ", ")
		if len(tags) > 18 {
			tags = tags[:18] + "..."
		}

		size := formatSize(res.Size)
		modTime := res.ModTime.Format("01-02 15:04")

		displayPath := res.Path
		if m.projectRoot != "" {
			if relPath, err := filepath.Rel(m.projectRoot, res.Path); err == nil && !strings.HasPrefix(relPath, "..") {
				displayPath = relPath
			}
		}
		if len(displayPath) > 28 {
			displayPath = "..." + displayPath[len(displayPath)-25:]
		}

		rows = append(rows, table.Row{
			res.Name,
			res.Type,
			size,
			tags,
			modTime,
			displayPath,
		})
	}

	m.table.SetRows(rows)
}

func (m *model) adjustLayout() {
	tableHeight := m.height - 12
	if tableHeight < 5 {
		tableHeight = 5
	}

	availableWidth := m.width - 10
	nameWidth := max(25, availableWidth/6)
	typeWidth := 12
	sizeWidth := 10
	tagsWidth := max(20, availableWidth/5)
	modifiedWidth := 15
	pathWidth := max(30, availableWidth/3)

	columns := []table.Column{
		{Title: "Name", Width: nameWidth},
		{Title: "Type", Width: typeWidth},
		{Title: "Size", Width: sizeWidth},
		{Title: "Tags", Width: tagsWidth},
		{Title: "Modified", Width: modifiedWidth},
		{Title: "Path", Width: pathWidth},
	}

	m.table.SetColumns(columns)
	m.table.SetHeight(tableHeight)

	menuColumns := []table.Column{
		{Title: "Option", Width: max(30, availableWidth/3)},
		{Title: "Description", Width: max(50, 2*availableWidth/3)},
	}
	m.menuTable.SetColumns(menuColumns)
	m.menuTable.SetHeight(min(8, tableHeight))

	m.viewport.Width = m.width - 4
	m.viewport.Height = tableHeight

	// Update custom viewers size
	// Chat: header(5) + input(5) + footer(2) + margins(3) = 15 lines overhead
	m.chatMaxLines = m.height - 15
	if m.chatMaxLines < 10 {
		m.chatMaxLines = 10
	}

	m.fileMaxLines = m.height - 12
	if m.fileMaxLines < 5 {
		m.fileMaxLines = 5
	}

	// Update chatTextArea dimensions - use almost full width
	textAreaWidth := m.width - 6
	if textAreaWidth < 40 {
		textAreaWidth = 40
	}
	m.chatTextArea.SetWidth(textAreaWidth)

	// Responsive text area height based on terminal size
	textAreaHeight := 3
	if m.height > 40 {
		textAreaHeight = 4
	}
	if m.height > 60 {
		textAreaHeight = 5
	}
	m.chatTextArea.SetHeight(textAreaHeight)
}

func formatSize(size int64) string {
	if size < 1024 {
		return fmt.Sprintf("%dB", size)
	} else if size < 1024*1024 {
		return fmt.Sprintf("%.1fKB", float64(size)/1024)
	} else {
		return fmt.Sprintf("%.1fMB", float64(size)/(1024*1024))
	}
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func (m model) updateConfirmDialog(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.confirmDialog == nil {
		return m, nil
	}

	switch msg.String() {
	case "y", "Y":
		// Execute the confirmed action
		switch m.confirmDialog.Action {
		case ConfirmDelete:
			return m.executeDelete()
		case ConfirmPush:
			return m.executePush()
		case ConfirmPull:
			return m.executePull()
		case ConfirmDeleteModel:
			return m.executeDeleteModel()
		}
	case "n", "N", "esc":
		// Cancel the action
		previousView := m.confirmDialog.PreviousView
		m.confirmDialog = nil
		m.viewMode = previousView
		return m, nil
	}

	return m, nil
}

func (m model) executeDelete() (tea.Model, tea.Cmd) {
	if m.confirmDialog == nil || m.confirmDialog.Resource == nil {
		m.confirmDialog = nil
		m.viewMode = ViewResourceManager
		return m, showStatus("❌ Delete failed: no resource selected")
	}

	res := m.confirmDialog.Resource
	if err := os.Remove(res.Path); err != nil {
		m.confirmDialog = nil
		m.viewMode = ViewResourceManager
		return m, showStatus(fmt.Sprintf("❌ Delete failed: %v", err))
	}

	// Remove from project metadata
	if m.projectMeta != nil {
		relPath, err := filepath.Rel(m.projectRoot, res.Path)
		if err != nil {
			relPath = res.Name
		}
		delete(m.projectMeta.Resources, relPath)
		m.saveProjectMetadata()
	}

	// Update view
	m.confirmDialog = nil
	m.viewMode = ViewResourceManager
	m.scanResources()
	m.updateTableData()
	return m, showStatus("🗑️ Resource deleted")
}

func (m model) executePush() (tea.Model, tea.Cmd) {
	if m.confirmDialog == nil || m.confirmDialog.Resource == nil {
		m.confirmDialog = nil
		m.viewMode = ViewResourceManager
		return m, showStatus("❌ Push failed: no resource selected")
	}

	res := m.confirmDialog.Resource
	if err := m.pushResourceToGlobal(res); err != nil {
		m.confirmDialog = nil
		m.viewMode = ViewResourceManager
		return m, showStatus(fmt.Sprintf("❌ Push failed: %v", err))
	}

	m.confirmDialog = nil
	m.viewMode = ViewResourceManager
	return m, showStatus("📤 Resource pushed to global")
}

func (m model) executePull() (tea.Model, tea.Cmd) {
	if m.confirmDialog == nil || m.confirmDialog.Resource == nil {
		m.confirmDialog = nil
		m.viewMode = ViewGlobalResources
		return m, showStatus("❌ Pull failed: no resource selected")
	}

	res := m.confirmDialog.Resource
	if err := m.pullResourceFromGlobal(res); err != nil {
		m.confirmDialog = nil
		m.viewMode = ViewGlobalResources
		return m, showStatus(fmt.Sprintf("❌ Pull failed: %v", err))
	}

	// Rescan local resources
	m.scanResources()
	m.updateTableData()

	m.confirmDialog = nil
	m.viewMode = ViewGlobalResources
	return m, showStatus("📥 Resource pulled to project")
}

func (m model) executeDeleteModel() (tea.Model, tea.Cmd) {
	if len(m.modelConfig.Profiles) <= 1 || m.modelSelection >= len(m.modelConfig.Profiles) {
		m.confirmDialog = nil
		m.viewMode = ViewModelManager
		return m, showStatus("❌ Cannot delete last profile")
	}

	profileName := m.modelConfig.Profiles[m.modelSelection].Name

	// Remove the selected profile
	m.modelConfig.Profiles = append(
		m.modelConfig.Profiles[:m.modelSelection],
		m.modelConfig.Profiles[m.modelSelection+1:]...,
	)

	// Adjust current profile index if needed
	if m.modelConfig.CurrentProfile >= len(m.modelConfig.Profiles) {
		m.modelConfig.CurrentProfile = len(m.modelConfig.Profiles) - 1
	}

	// Adjust selection index if needed
	if m.modelSelection >= len(m.modelConfig.Profiles) {
		m.modelSelection = len(m.modelConfig.Profiles) - 1
	}

	m.saveModelConfig()
	m.confirmDialog = nil
	m.viewMode = ViewModelManager
	return m, showStatus(fmt.Sprintf("🗑️ Profile '%s' deleted", profileName))
}

func (m *model) handleFileScroll(key string) {
	maxScroll := len(m.fileLines) - m.fileMaxLines
	if maxScroll < 0 {
		maxScroll = 0
	}

	switch key {
	case "up":
		if m.fileScrollPos > 0 {
			m.fileScrollPos--
		}
	case "down":
		if m.fileScrollPos < maxScroll {
			m.fileScrollPos++
		}
	case "pgup":
		m.fileScrollPos -= m.fileMaxLines / 2
		if m.fileScrollPos < 0 {
			m.fileScrollPos = 0
		}
	case "pgdown":
		m.fileScrollPos += m.fileMaxLines / 2
		if m.fileScrollPos > maxScroll {
			m.fileScrollPos = maxScroll
		}
	case "home":
		m.fileScrollPos = 0
	case "end":
		m.fileScrollPos = maxScroll
	}
}

func (m *model) getVisibleChatLines() []string {
	if len(m.chatLines) == 0 {
		return []string{"💬 Start a conversation with the AI assistant..."}
	}

	start := m.chatScrollPos
	end := start + m.chatMaxLines

	if start < 0 {
		start = 0
	}
	if end > len(m.chatLines) {
		end = len(m.chatLines)
	}
	if start >= len(m.chatLines) {
		return []string{"(no content)"}
	}

	return m.chatLines[start:end]
}

func (m *model) getVisibleFileLines() []string {
	if len(m.fileLines) == 0 {
		return []string{"(empty file)"}
	}

	start := m.fileScrollPos
	end := start + m.fileMaxLines

	if start < 0 {
		start = 0
	}
	if end > len(m.fileLines) {
		end = len(m.fileLines)
	}
	if start >= len(m.fileLines) {
		return []string{"(no content)"}
	}

	return m.fileLines[start:end]
}

func (m *model) updateChatLines() {
	contentWidth := m.width - 4 // Leave margin
	if contentWidth < 20 {
		contentWidth = 20
	}

	m.chatLines = []string{}

	if len(m.chatMessages) == 0 && !m.chatStreaming {
		m.chatLines = append(m.chatLines, "💬 Start a conversation...")
		m.chatLines = append(m.chatLines, "")
		m.chatLines = append(m.chatLines, "Enter: send | Tab: switch model | Esc: menu")
		m.chatScrollPos = 0
		return
	}

	// Use cached formatted lines when possible
	for i := range m.chatMessages {
		msg := &m.chatMessages[i]

		// Check if we need to reformat (width changed or not yet cached)
		if msg.lastWidth != contentWidth || len(msg.formattedLines) == 0 {
			msg.formattedLines = m.formatMessage(msg, contentWidth)
			msg.lastWidth = contentWidth
		}

		// Append cached lines
		m.chatLines = append(m.chatLines, msg.formattedLines...)
	}

	// Show streaming content
	if m.chatStreaming && m.chatStreamBuffer.Len() > 0 {
		header := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#34D399")).Render("🤖 Assistant:")
		m.chatLines = append(m.chatLines, header)

		// Format streaming content with markdown
		formatted := formatMessageContent(m.chatStreamBuffer.String())
		wrapped := wrapText(formatted, contentWidth)
		for _, line := range wrapped {
			m.chatLines = append(m.chatLines, "  "+line)
		}
		m.chatLines = append(m.chatLines, "")
	}

	if m.chatState == ChatStateLoading {
		m.chatLines = append(m.chatLines, fmt.Sprintf("%s Thinking...", m.chatSpinner.View()))
	}

	// Auto-scroll to bottom when new content is added
	if len(m.chatLines) > m.chatMaxLines {
		m.chatScrollPos = len(m.chatLines) - m.chatMaxLines
	} else {
		m.chatScrollPos = 0
	}
}

// formatMessage formats a single message and returns the lines (cached per message)
func (m *model) formatMessage(msg *ChatMessage, contentWidth int) []string {
	var lines []string

	if msg.Role == "user" {
		userLabel := "👤 You:"
		userLabelStyled := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#60A5FA")).Render(userLabel)
		lines = append(lines, userLabelStyled)
		wrapped := wrapText(msg.Content, contentWidth)
		lines = append(lines, wrapped...)
		lines = append(lines, "")
	} else {
		header := "🤖 AI:"
		if msg.Duration > 0 {
			tokPerSec := 0.0
			if msg.Duration.Seconds() > 0 && msg.TotalTokens > 0 {
				tokPerSec = float64(msg.TotalTokens-msg.PromptTokens) / msg.Duration.Seconds()
			}
			header = fmt.Sprintf("🤖 AI: %.1fs • %.0f tok/s", msg.Duration.Seconds(), tokPerSec)
		}
		headerStyled := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#34D399")).Render(header)
		lines = append(lines, headerStyled)

		// Simple formatting without excessive styling
		wrapped := wrapText(msg.Content, contentWidth)
		lines = append(lines, wrapped...)
		lines = append(lines, "")
	}

	return lines
}

// Clean file content renderer
func (m *model) updateFileLines(content string) {
	contentWidth := m.width - 4 // Leave margin
	if contentWidth < 20 {
		contentWidth = 20
	}

	m.fileLines = wrapText(content, contentWidth)
	m.fileScrollPos = 0 // Start at top
}

// Clean scroll handling for chat
func (m *model) handleChatScroll(key string) {
	maxScroll := len(m.chatLines) - m.chatMaxLines
	if maxScroll < 0 {
		maxScroll = 0
	}

	switch key {
	case "up":
		if m.chatScrollPos > 0 {
			m.chatScrollPos--
		}
	case "down":
		if m.chatScrollPos < maxScroll {
			m.chatScrollPos++
		}
	case "pgup":
		m.chatScrollPos -= m.chatMaxLines / 2
		if m.chatScrollPos < 0 {
			m.chatScrollPos = 0
		}
	case "pgdown":
		m.chatScrollPos += m.chatMaxLines / 2
		if m.chatScrollPos > maxScroll {
			m.chatScrollPos = maxScroll
		}
	case "home":
		m.chatScrollPos = 0
	case "end":
		m.chatScrollPos = maxScroll
	}
}
