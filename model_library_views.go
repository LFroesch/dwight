package main

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// viewModelLibrary displays the Ollama model library with install options
func (m model) viewModelLibrary() string {
	titleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#A78BFA")).
		Bold(true)
	title := titleStyle.Render("📚 Ollama Model Library")

	selectedStyle := lipgloss.NewStyle().
		Background(lipgloss.Color("#7C3AED")).
		Foreground(lipgloss.Color("#FFFFFF")).
		Bold(true)

	normalStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FFFFFF"))

	installedStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#10B981"))

	dimStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#9CA3AF"))

	var content strings.Builder

	if m.libraryFilter != "" {
		content.WriteString(dimStyle.Render(fmt.Sprintf("🔍 Filter: %s\n\n", m.libraryFilter)))
	}

	content.WriteString(dimStyle.Render("Popular Models (press Enter to install):\n\n"))

	// Filter models if filter is active
	models := m.libraryModels
	if m.libraryFilter != "" {
		filtered := []OllamaLibraryModel{}
		filterLower := strings.ToLower(m.libraryFilter)
		for _, model := range models {
			if strings.Contains(strings.ToLower(model.Name), filterLower) ||
				strings.Contains(strings.ToLower(model.Description), filterLower) {
				filtered = append(filtered, model)
			}
		}
		models = filtered
	}

	if len(models) == 0 {
		content.WriteString(dimStyle.Render("No models match your filter.\n"))
	}

	// Display models
	for i, model := range models {
		// Check if installed
		isInstalled := checkModelInstalled(model.Name, m.installedModels)
		model.Installed = isInstalled

		indicator := "  "
		if isInstalled {
			indicator = "✓ "
		}

		// Create model line with name, description, and size
		line := fmt.Sprintf("%s%-25s %s (%s)",
			indicator,
			model.Name,
			truncate(model.Description, 50),
			model.Size)

		if i == m.librarySelection {
			content.WriteString(selectedStyle.Render("> " + line))
		} else if isInstalled {
			content.WriteString(installedStyle.Render("  " + line))
		} else {
			content.WriteString(normalStyle.Render("  " + line))
		}
		content.WriteString("\n")

		// Show tags on next line
		if len(model.Tags) > 0 {
			tags := strings.Join(model.Tags, ", ")
			tagLine := fmt.Sprintf("    Tags: %s", tags)
			if i == m.librarySelection {
				content.WriteString(dimStyle.Render(tagLine))
			} else {
				content.WriteString(dimStyle.Render(tagLine))
			}
			content.WriteString("\n")
		}
	}

	footer := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#60A5FA")).
		Render("\nCommands: ") +
		lipgloss.NewStyle().Foreground(lipgloss.Color("#10B981")).Render("↑↓/Enter: install") +
		lipgloss.NewStyle().Foreground(lipgloss.Color("#9CA3AF")).Render(" • ") +
		lipgloss.NewStyle().Foreground(lipgloss.Color("#FBBF24")).Render("/: filter, r: refresh") +
		lipgloss.NewStyle().Foreground(lipgloss.Color("#9CA3AF")).Render(" • ") +
		lipgloss.NewStyle().Foreground(lipgloss.Color("#F87171")).Render("esc")

	return lipgloss.JoinVertical(lipgloss.Left, title, "", content.String(), footer)
}
