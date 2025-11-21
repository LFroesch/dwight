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

	content.WriteString(dimStyle.Render("Popular Models (press Enter to install):"))
	content.WriteString("\n\n")

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
	} else {
		// Calculate visible window (estimate 2 lines per model with tags)
		maxVisibleModels := (m.height - 10) / 2 // Account for header/footer
		if maxVisibleModels < 5 {
			maxVisibleModels = 5
		}

		// Calculate scroll offset to keep selection visible
		scrollOffset := 0
		if m.librarySelection >= maxVisibleModels {
			scrollOffset = m.librarySelection - maxVisibleModels + 1
		}

		// Display only visible models
		endIdx := scrollOffset + maxVisibleModels
		if endIdx > len(models) {
			endIdx = len(models)
		}

		for i := scrollOffset; i < endIdx; i++ {
			model := models[i]
			// Check if installed
			isInstalled := checkModelInstalled(model.Name, m.installedModels)
			model.Installed = isInstalled

			// Create model line with name, description, and size
			line := fmt.Sprintf("%-25s %s (%s)",
				model.Name,
				truncate(model.Description, 50),
				model.Size)

			if i == m.librarySelection {
				if isInstalled {
					content.WriteString(selectedStyle.Render("> ✓ " + line))
				} else {
					content.WriteString(selectedStyle.Render("> " + line))
				}
			} else if isInstalled {
				content.WriteString(installedStyle.Render("✓ " + line))
			} else {
				content.WriteString(normalStyle.Render(line))
			}
			content.WriteString("\n")

			// Show tags on next line
			if len(model.Tags) > 0 {
				tags := strings.Join(model.Tags, ", ")
				tagLine := fmt.Sprintf("  Tags: %s", tags)
				content.WriteString(dimStyle.Render(tagLine))
				content.WriteString("\n")
			}
		}

		// Show scroll indicator if there are more items
		if len(models) > maxVisibleModels {
			scrollInfo := dimStyle.Render(fmt.Sprintf("\n[Showing %d-%d of %d models]",
				scrollOffset+1, endIdx, len(models)))
			content.WriteString(scrollInfo)
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
