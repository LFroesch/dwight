package main

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// updateModelLibrary handles key events in the model library view
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
		return m, nil

	case "down", "j":
		maxSelection := len(m.libraryModels) - 1
		if m.libraryFilter != "" {
			// Count filtered models
			count := 0
			filterLower := strings.ToLower(m.libraryFilter)
			for _, model := range m.libraryModels {
				if strings.Contains(strings.ToLower(model.Name), filterLower) ||
					strings.Contains(strings.ToLower(model.Description), filterLower) {
					count++
				}
			}
			maxSelection = count - 1
		}
		if m.librarySelection < maxSelection {
			m.librarySelection++
		}
		return m, nil

	case "r":
		// Refresh installed models list
		return m, refreshInstalledModels()

	case "enter":
		// Install selected model
		if m.librarySelection < len(m.libraryModels) {
			modelName := m.libraryModels[m.librarySelection].Name

			// Check if already installed
			if checkModelInstalled(modelName, m.installedModels) {
				return m, showStatus(fmt.Sprintf("✅ %s is already installed", modelName))
			}

			// Start pulling the model
			m.viewMode = ViewModelPull
			m.modelPullName = modelName
			m.modelPullStatus = fmt.Sprintf("🔄 Installing %s...", modelName)
			m.modelPullError = nil
			return m, pullOllamaModel(modelName)
		}
		return m, nil
	}

	return m, nil
}

// refreshInstalledModels fetches the list of installed models
func refreshInstalledModels() tea.Cmd {
	return func() tea.Msg {
		models, err := getInstalledModels()
		if err != nil {
			return statusMsg{message: fmt.Sprintf("❌ Failed to refresh: %v", err)}
		}
		return installedModelsMsg{models: models}
	}
}

type installedModelsMsg struct {
	models []OllamaModel
}
