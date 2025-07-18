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
			// Remove the selected profile
			m.modelConfig.Profiles = append(
				m.modelConfig.Profiles[:m.modelSelection],
				m.modelConfig.Profiles[m.modelSelection+1:]...,
			)
			if m.modelConfig.CurrentProfile >= len(m.modelConfig.Profiles) {
				m.modelConfig.CurrentProfile = len(m.modelConfig.Profiles) - 1
			}
			if m.modelSelection >= len(m.modelConfig.Profiles) {
				m.modelSelection = len(m.modelConfig.Profiles) - 1
			}
			m.saveModelConfig()
			return m, showStatus("🗑️ Profile deleted")
		}
		return m, showStatus("❌ Cannot delete last profile")
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
		if m.viewMode == ViewChatPlaceholder {
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
		case ViewGlobalResourcesPlaceholder, ViewSettingsPlaceholder, ViewCleanupPlaceholder, ViewCleanupChats:
			return m.updatePlaceholder(msg)
		case ViewChatPlaceholder:
			return m.updateChat(msg)
		case ViewModelManager:
			return m.updateModelManager(msg)
		case ViewModelCreate:
			return m.updateModelCreate(msg)
		case ViewModelPull:
			return m.updateModelPull(msg)
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
		} else {
			m.chatState = ChatStateReady
		}
		return m, nil

	case ResponseMsg:
		if msg.Err != nil {
			m.chatErr = msg.Err
			m.chatState = ChatStateError
		} else {
			m.chatMessages = append(m.chatMessages, ChatMessage{
				Role:         "assistant",
				Content:      msg.Content,
				Duration:     msg.Duration,
				PromptTokens: msg.PromptTokens,
				TotalTokens:  msg.TotalTokens,
			})
			m.chatState = ChatStateReady
			// Update text viewer content and auto-scroll to bottom
			m.updateChatLines()
		}
		return m, nil

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
			// Replace the existing case 1 with this:
			m.viewMode = ViewChatPlaceholder
			m.chatState = ChatStateCheckingModel
			m.chatInput.Focus()
			m.chatMessages = []ChatMessage{} // Clear previous messages
			m.updateChatLines()              // Initialize text viewer with empty content
			return m, tea.Batch(
				m.checkOllamaModel(),
				m.chatSpinner.Tick,
			)
		case 2:
			m.viewMode = ViewGlobalResourcesPlaceholder
			m.scanGlobalResources()
		case 3:
			m.viewMode = ViewSettingsPlaceholder
		case 4:
			// Stop Ollama container
			go stopOllamaContainer()
			return m, showStatus("🛑 Ollama container stopped to free memory")
		case 5:
			m.viewMode = ViewCleanupPlaceholder
		case 6:
			m.viewMode = ViewCleanupChats
		case 7:
			m.viewMode = ViewModelManager
			m.modelSelection = m.modelConfig.CurrentProfile
		case 8:
			return m, tea.Quit
		}
		return m, nil
	}

	var cmd tea.Cmd
	m.menuTable, cmd = m.menuTable.Update(msg)
	return m, cmd
}

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
		m.chatInput.Blur()
		m.chatMessages = []ChatMessage{}
		m.chatState = ChatStateInit
		// Stop Ollama container to free memory when exiting chat
		go stopOllamaContainer()
		return m, nil
	case "tab":
		if m.chatState == ChatStateReady {
			m.modelConfig.CurrentProfile = (m.modelConfig.CurrentProfile + 1) % len(m.modelConfig.Profiles)
			m.saveModelConfig()
			return m, showStatus(fmt.Sprintf("Switched to %s", m.modelConfig.Profiles[m.modelConfig.CurrentProfile].Name))
		}
	case "shift+tab":
		if m.chatState == ChatStateReady {
			m.modelConfig.CurrentProfile = (m.modelConfig.CurrentProfile - 1 + len(m.modelConfig.Profiles)) % len(m.modelConfig.Profiles)
			m.saveModelConfig()
			return m, showStatus(fmt.Sprintf("Switched to %s", m.modelConfig.Profiles[m.modelConfig.CurrentProfile].Name))
		}
	case "enter":
		if m.chatState == ChatStateReady && m.chatInput.Value() != "" {
			userMsg := m.chatInput.Value()
			m.chatMessages = append(m.chatMessages, ChatMessage{Role: "user", Content: userMsg})
			m.chatInput.SetValue("")
			m.chatState = ChatStateLoading
			// Update text viewer content and auto-scroll to bottom
			m.updateChatLines()
			return m, tea.Batch(
				sendChatMessage(userMsg, m.getCurrentProfile()),
				m.chatSpinner.Tick, // Add this to start the spinner animation
			)
		}
	}

	if m.chatState == ChatStateReady {
		m.chatInput, cmd = m.chatInput.Update(msg)
	}

	// Handle scrolling for custom chat viewer
	switch msg.String() {
	case "pgup", "up":
		m.chatScrollPos -= m.chatMaxLines / 2
		if m.chatScrollPos < 0 {
			m.chatScrollPos = 0
		}
	case "pgdown", "down":
		m.chatScrollPos += m.chatMaxLines / 2
		if m.chatScrollPos > len(m.chatLines)-m.chatMaxLines {
			m.chatScrollPos = len(m.chatLines) - m.chatMaxLines
		}
	case "home":
		m.chatScrollPos = 0
	case "end":
		if len(m.chatLines) > m.chatMaxLines {
			m.chatScrollPos = len(m.chatLines) - m.chatMaxLines
		}
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
		if m.viewMode == ViewGlobalResourcesPlaceholder && len(m.globalRes) > 0 {
			if m.cursor > 0 {
				m.cursor--
			}
		}
	case "down", "j":
		if m.viewMode == ViewGlobalResourcesPlaceholder && len(m.globalRes) > 0 {
			if m.cursor < len(m.globalRes)-1 {
				m.cursor++
			}
		}
	case "enter", " ", "v":
		if m.viewMode == ViewGlobalResourcesPlaceholder && len(m.globalRes) > 0 && m.cursor < len(m.globalRes) {
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
		if m.viewMode == ViewGlobalResourcesPlaceholder {
			return m, nil
		}
	case "r":
		if m.viewMode == ViewGlobalResourcesPlaceholder {
			m.scanGlobalResources()
			return m, showStatus("🔄 Global resources refreshed")
		}
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
	case "c":
		m.createTemplate()
		return m, showStatus("📝 Template created")
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
			os.Remove(res.Path)

			if m.projectMeta != nil {
				relPath, err := filepath.Rel(m.projectRoot, res.Path)
				if err != nil {
					relPath = res.Name
				}
				delete(m.projectMeta.Resources, relPath)
				m.saveProjectMetadata()
			}

			m.scanResources()
			m.updateTableData()
			return m, showStatus("🗑️ Resource deleted")
		}
		return m, nil
	case "t":
		if m.projectRoot != "" {
			templatePath := filepath.Join(m.projectRoot, "template.md")
			m.createDefaultTemplate(templatePath)
			m.scanResources()
			m.updateTableData()
			return m, showStatus("📝 Template regenerated")
		}
		return m, showStatus("❌ No project root found")
	}

	m.table, cmd = m.table.Update(msg)
	return m, cmd
}

func (m model) updateDetails(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg.String() {
	case "esc", "q":
		if m.fromGlobal {
			m.viewMode = ViewGlobalResourcesPlaceholder
			m.fromGlobal = false
		} else {
			m.viewMode = ViewResourceManager
		}
		return m, nil
	case "e":
		if m.selectedRes != nil && !m.fromGlobal {
			m.startEditing()
		} else if m.fromGlobal {
			return m, nil
		}
		return m, nil
	}

	// Handle scrolling for file viewer
	switch msg.String() {
	case "pgup", "up":
		m.fileScrollPos -= m.fileMaxLines / 2
		if m.fileScrollPos < 0 {
			m.fileScrollPos = 0
		}
	case "pgdown", "down":
		m.fileScrollPos += m.fileMaxLines / 2
		if m.fileScrollPos > len(m.fileLines)-m.fileMaxLines {
			m.fileScrollPos = len(m.fileLines) - m.fileMaxLines
		}
	case "home":
		m.fileScrollPos = 0
	case "end":
		if len(m.fileLines) > m.fileMaxLines {
			m.fileScrollPos = len(m.fileLines) - m.fileMaxLines
		}
	}
	return m, cmd
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

	m.inputs = make([]textinput.Model, 3)

	m.inputs[0] = textinput.New()
	m.inputs[0].SetValue(m.selectedRes.Description)
	m.inputs[0].Focus()
	m.inputs[0].CharLimit = 200

	m.inputs[1] = textinput.New()
	m.inputs[1].SetValue(strings.Join(m.selectedRes.Tags, ", "))
	m.inputs[1].CharLimit = 100

	m.inputs[2] = textinput.New()
	m.inputs[2].SetValue(m.selectedRes.Type)
	m.inputs[2].CharLimit = 50
}

func (m *model) saveEdit() {
	if m.selectedRes == nil || len(m.inputs) < 3 {
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

	m.selectedRes.Type = m.inputs[2].Value()

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
	tableHeight := m.height - 6
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
	m.chatMaxLines = m.height - 10
	if m.chatMaxLines < 5 {
		m.chatMaxLines = 5
	}

	m.fileMaxLines = m.height - 10
	if m.fileMaxLines < 5 {
		m.fileMaxLines = 5
	}
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

func (m *model) updateChatLines() {
	m.chatLines = []string{}

	if len(m.chatMessages) == 0 {
		m.chatLines = append(m.chatLines, "💬 Start a conversation with the AI assistant...")
		m.chatLines = append(m.chatLines, "")
		return
	}

	for _, msg := range m.chatMessages {
		if msg.Role == "user" {
			m.chatLines = append(m.chatLines, "👤 You:")
			// Split long lines
			lines := strings.Split(msg.Content, "\n")
			for _, line := range lines {
				if len(line) > 80 {
					// Simple word wrap
					words := strings.Fields(line)
					currentLine := ""
					for _, word := range words {
						if len(currentLine)+len(word)+1 > 80 {
							if currentLine != "" {
								m.chatLines = append(m.chatLines, currentLine)
								currentLine = word
							} else {
								m.chatLines = append(m.chatLines, word)
							}
						} else {
							if currentLine == "" {
								currentLine = word
							} else {
								currentLine += " " + word
							}
						}
					}
					if currentLine != "" {
						m.chatLines = append(m.chatLines, currentLine)
					}
				} else {
					m.chatLines = append(m.chatLines, line)
				}
			}
			m.chatLines = append(m.chatLines, "")
		} else {
			header := "🤖 Assistant:"
			if msg.Duration > 0 {
				header = fmt.Sprintf("🤖 Assistant: (%.1fs, %d tokens)", msg.Duration.Seconds(), msg.TotalTokens)
			}
			m.chatLines = append(m.chatLines, header)

			// Split long lines
			lines := strings.Split(msg.Content, "\n")
			for _, line := range lines {
				if len(line) > 80 {
					// Simple word wrap
					words := strings.Fields(line)
					currentLine := ""
					for _, word := range words {
						if len(currentLine)+len(word)+1 > 80 {
							if currentLine != "" {
								m.chatLines = append(m.chatLines, currentLine)
								currentLine = word
							} else {
								m.chatLines = append(m.chatLines, word)
							}
						} else {
							if currentLine == "" {
								currentLine = word
							} else {
								currentLine += " " + word
							}
						}
					}
					if currentLine != "" {
						m.chatLines = append(m.chatLines, currentLine)
					}
				} else {
					m.chatLines = append(m.chatLines, line)
				}
			}
			m.chatLines = append(m.chatLines, "")
		}
	}

	if m.chatState == ChatStateLoading {
		m.chatLines = append(m.chatLines, fmt.Sprintf("%s Thinking...", m.chatSpinner.View()))
	}

	// Auto scroll to bottom
	if len(m.chatLines) > m.chatMaxLines {
		m.chatScrollPos = len(m.chatLines) - m.chatMaxLines
	}
}

func (m *model) updateFileLines(content string) {
	m.fileLines = []string{}

	lines := strings.Split(content, "\n")
	for _, line := range lines {
		if len(line) > 80 {
			// Simple word wrap
			words := strings.Fields(line)
			currentLine := ""
			for _, word := range words {
				if len(currentLine)+len(word)+1 > 80 {
					if currentLine != "" {
						m.fileLines = append(m.fileLines, currentLine)
						currentLine = word
					} else {
						m.fileLines = append(m.fileLines, word)
					}
				} else {
					if currentLine == "" {
						currentLine = word
					} else {
						currentLine += " " + word
					}
				}
			}
			if currentLine != "" {
				m.fileLines = append(m.fileLines, currentLine)
			}
		} else {
			m.fileLines = append(m.fileLines, line)
		}
	}

	// Start at top
	m.fileScrollPos = 0
}
