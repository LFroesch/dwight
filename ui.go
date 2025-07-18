package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/table"
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
		m.statusExpiry = time.Now().Add(3 * time.Second)
		return m, nil

	case tickMsg:
		m.lastUpdate = time.Time(msg)
		return m, tickCmd()

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.adjustLayout()
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
		case ViewGlobalResourcesPlaceholder, ViewSettingsPlaceholder, ViewCleanupPlaceholder:
			return m.updatePlaceholder(msg)
		case ViewChatPlaceholder:
			return m.updateChat(msg)
		}

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
			m.chatMessages = append(m.chatMessages, ChatMessage{Role: "assistant", Content: msg.Content})
			m.chatState = ChatStateReady
		}
		return m, nil
	}

	return m, nil
}

func (m model) updateMenu(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
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
			return m, tea.Batch(
				checkOllamaModel(),
				m.chatSpinner.Tick,
			)
		case 2:
			m.viewMode = ViewGlobalResourcesPlaceholder
			m.scanGlobalResources()
		case 3:
			m.viewMode = ViewSettingsPlaceholder
		case 4:
			m.viewMode = ViewCleanupPlaceholder
		case 5:
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
	case "esc", "q":
		m.viewMode = ViewMenu
		m.chatInput.Blur()
		m.chatMessages = []ChatMessage{}
		m.chatState = ChatStateInit
		return m, nil
	case "enter":
		if m.chatState == ChatStateReady && m.chatInput.Value() != "" {
			userMsg := m.chatInput.Value()
			m.chatMessages = append(m.chatMessages, ChatMessage{Role: "user", Content: userMsg})
			m.chatInput.SetValue("")
			m.chatState = ChatStateLoading
			return m, sendChatMessage(userMsg)
		}
	}

	if m.chatState == ChatStateReady {
		m.chatInput, cmd = m.chatInput.Update(msg)
	}

	return m, cmd
}

func (m model) updatePlaceholder(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
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
					m.viewport.Width = m.width - 4
					m.viewport.Height = m.height - 10
					m.viewport.SetContent(string(content))
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
				m.viewport.Width = m.width - 4
				m.viewport.Height = m.height - 10
				m.viewport.SetContent(string(content))
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

	m.viewport, cmd = m.viewport.Update(msg)
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
