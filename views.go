package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

func (m model) View() string {
	errorStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#EF4444")).
		Bold(true)

	successStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#10B981")).
		Bold(true)

	helpStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#6B7280"))

	switch m.viewMode {
	case ViewMenu:
		return m.viewMenu()
	case ViewResourceManager:
		if m.showHelp {
			return m.viewHelp()
		}
		if m.editMode && len(m.inputs) > 0 {
			return m.editView()
		}
		return m.viewResourceManager(errorStyle, successStyle, helpStyle)
	case ViewDetails:
		if m.editMode && len(m.inputs) > 0 {
			return m.editView()
		}
		return m.viewDetails()
	case ViewCreate:
		return m.viewCreate()
	case ViewChatPlaceholder:
		return m.viewChat()
	case ViewGlobalResourcesPlaceholder:
		return m.viewGlobalResources()
	case ViewSettingsPlaceholder:
		return m.viewPlaceholder("Settings", "⚙️ Settings configuration coming soon...", "This feature will provide options to configure Dwight preferences, paths, and behavior.")
	case ViewCleanupPlaceholder:
		return m.viewPlaceholder("Clean Up Resources", "🧹 Resource cleanup coming soon...", "This feature will help identify and remove unused or outdated AI resources.")
	case ViewCleanupChats:
		return m.viewCleanupChats()
	case ViewModelManager:
		return m.viewModelManager()
	case ViewModelCreate:
		return m.viewModelCreate()
	case ViewModelPull:
		return m.viewModelPull()
	}

	return ""
}

func (m model) viewModelManager() string {
	titleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#7C3AED")).
		Bold(true)
	title := titleStyle.Render("🤖 Model Manager")

	selectedStyle := lipgloss.NewStyle().
		Background(lipgloss.Color("#7C3AED")).
		Foreground(lipgloss.Color("#F3F4F6")).
		Bold(true)

	normalStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#E5E7EB"))

	var content strings.Builder
	content.WriteString("📋 Available Profiles:\n\n")

	for i, profile := range m.modelConfig.Profiles {
		indicator := "  "
		if i == m.modelConfig.CurrentProfile {
			indicator = "✓ "
		}

		line := fmt.Sprintf("%s%-20s | %-25s | Temp: %.1f",
			indicator, profile.Name, profile.Model, profile.Temperature)

		if i == m.modelSelection {
			content.WriteString(selectedStyle.Render("> " + line))
		} else {
			content.WriteString(normalStyle.Render("  " + line))
		}
		content.WriteString("\n")
	}

	footer := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#60A5FA")).
		Render("Commands: ") +
		lipgloss.NewStyle().Foreground(lipgloss.Color("#34D399")).Render("↑↓: navigate, Enter: set default") +
		lipgloss.NewStyle().Foreground(lipgloss.Color("#6B7280")).Render(" • ") +
		lipgloss.NewStyle().Foreground(lipgloss.Color("#FBBF24")).Render("n: new profile, e: edit, p: pull model, d: delete") +
		lipgloss.NewStyle().Foreground(lipgloss.Color("#6B7280")).Render(" • ") +
		lipgloss.NewStyle().Foreground(lipgloss.Color("#F87171")).Render("esc: back")

	return lipgloss.JoinVertical(lipgloss.Left, title, "", content.String(), "", footer)
}

func (m model) viewModelCreate() string {
	titleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#7C3AED")).
		Bold(true)

	title := "📝 Create New Profile"
	if m.viewMode == ViewModelCreate && m.editField >= 0 && m.editField < len(m.modelConfig.Profiles) {
		title = fmt.Sprintf("✏️ Edit Profile: %s", m.modelConfig.Profiles[m.editField].Name)
	}
	title = titleStyle.Render(title)

	var fields []string
	labels := []string{"Profile Name:", "Model Name:", "System Prompt:", "Temperature (0.0-1.0):"}

	for i, input := range m.modelInputs {
		labelStyle := lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#34D399"))

		label := labelStyle.Render(labels[i])
		fields = append(fields, label+"\n"+input.View())
	}

	content := lipgloss.JoinVertical(lipgloss.Top, fields...)

	footer := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#60A5FA")).
		Render("Commands: ") +
		lipgloss.NewStyle().Foreground(lipgloss.Color("#34D399")).Render("Tab: next field, Enter: save") +
		lipgloss.NewStyle().Foreground(lipgloss.Color("#6B7280")).Render(" • ") +
		lipgloss.NewStyle().Foreground(lipgloss.Color("#F87171")).Render("esc: cancel")

	return lipgloss.JoinVertical(lipgloss.Left, title, "", content, "", footer)
}

func (m model) viewModelPull() string {
	titleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#7C3AED")).
		Bold(true)
	title := titleStyle.Render("📥 Pull Model from Ollama")

	var content string
	if m.modelPullError != nil {
		errorStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#EF4444")).
			Bold(true)
		content = errorStyle.Render(fmt.Sprintf("❌ Error pulling %s: %v", m.modelPullName, m.modelPullError))
	} else {
		statusStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FBBF24"))
		content = statusStyle.Render(m.modelPullStatus)
	}

	footer := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#60A5FA")).
		Render("Commands: ") +
		lipgloss.NewStyle().Foreground(lipgloss.Color("#F87171")).Render("esc: back to model manager")

	return lipgloss.JoinVertical(lipgloss.Left, title, "", content, "", footer)
}

func (m model) viewCleanupChats() string {
	titleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#7C3AED")).
		Bold(true)
	title := titleStyle.Render("🧹 Clean Up Chat Logs")

	contentStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#E5E7EB"))

	chatsDir := filepath.Join(m.currentDir, "chats")
	chatCount := 0
	totalSize := int64(0)

	if files, err := os.ReadDir(chatsDir); err == nil {
		for _, file := range files {
			if !file.IsDir() && strings.HasSuffix(file.Name(), ".txt") {
				chatCount++
				if info, err := file.Info(); err == nil {
					totalSize += info.Size()
				}
			}
		}
	}

	content := contentStyle.Render(fmt.Sprintf(
		"📁 Chat logs directory: %s\n\n"+
			"📊 Total chat logs: %d\n"+
			"💾 Total size: %s\n\n"+
			"Press a number key to delete chats older than:\n"+
			"  1 - 1 day\n"+
			"  7 - 7 days\n"+
			"  3 - 30 days\n"+
			"  9 - 90 days\n"+
			"  a - Delete ALL chat logs\n",
		chatsDir, chatCount, formatSize(totalSize)))

	footer := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#60A5FA")).
		Render("Commands: ") +
		lipgloss.NewStyle().Foreground(lipgloss.Color("#F87171")).Render("esc: back to menu")

	return lipgloss.JoinVertical(lipgloss.Left, title, "", content, "", footer)
}

func (m model) viewMenu() string {
	titleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#7C3AED")).
		Bold(true)
	title := titleStyle.Render("🤖 dwight - AI Resource Manager")

	var statusMessage string
	if m.statusMsg != "" && time.Now().Before(m.statusExpiry) {
		if strings.Contains(m.statusMsg, "❌") || strings.Contains(m.statusMsg, "Failed") {
			statusMessage = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#EF4444")).
				Bold(true).
				Render("Status: " + m.statusMsg)
		} else {
			statusMessage = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#10B981")).
				Bold(true).
				Render("Status: " + m.statusMsg)
		}
	}

	helpStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#60A5FA"))
	footer := helpStyle.Render("Commands: ") +
		lipgloss.NewStyle().Foreground(lipgloss.Color("#34D399")).Render("↑↓: navigate") +
		lipgloss.NewStyle().Foreground(lipgloss.Color("#6B7280")).Render(" • ") +
		lipgloss.NewStyle().Foreground(lipgloss.Color("#FBBF24")).Render("enter/space: select") +
		lipgloss.NewStyle().Foreground(lipgloss.Color("#6B7280")).Render(" • ") +
		lipgloss.NewStyle().Foreground(lipgloss.Color("#F87171")).Render("q: quit")

	var parts []string
	parts = append(parts, title)
	parts = append(parts, m.menuTable.View())

	if statusMessage != "" {
		parts = append(parts, statusMessage)
	}

	parts = append(parts, footer)

	return lipgloss.JoinVertical(lipgloss.Left, parts...)
}

func (m model) viewResourceManager(errorStyle, successStyle, helpStyle lipgloss.Style) string {
	if len(m.filteredRes) == 0 {
		emptyStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#6B7280")).
			MarginTop(1).
			MarginBottom(1)

		content := emptyStyle.Render("📋 No AI resources found.\n\n💡 Press 'n' to add your first resource!")
		footer := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#60A5FA")).
			Render("Commands: ") +
			lipgloss.NewStyle().Foreground(lipgloss.Color("#FBBF24")).Render("n/a: add resource") +
			lipgloss.NewStyle().Foreground(lipgloss.Color("#6B7280")).Render(" • ") +
			lipgloss.NewStyle().Foreground(lipgloss.Color("#F87171")).Render("esc: back")

		return lipgloss.JoinVertical(lipgloss.Left, content, footer)
	}

	templateCount := 0
	promptCount := 0
	for _, res := range m.filteredRes {
		switch res.Type {
		case "template":
			templateCount++
		case "prompt":
			promptCount++
		}
	}

	titleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#7C3AED")).
		Bold(true)
	title := titleStyle.Render("📂 Resource Manager")

	statsStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#9CA3AF")).
		Background(lipgloss.Color("#111827")).
		Padding(0, 1).
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#374151"))
	statsText := statsStyle.Render(fmt.Sprintf("📊 Resources: %d total | %d prompts",
		len(m.filteredRes), promptCount))

	stats := lipgloss.JoinHorizontal(lipgloss.Center, title, "  ", statsText)

	info := fmt.Sprintf("📁 %s", m.currentDir)
	if m.projectRoot != "" && m.projectRoot != m.currentDir {
		info = fmt.Sprintf("📁 %s (project: %s)", m.currentDir, filepath.Base(m.projectRoot))
	}
	if m.filterTag != "" {
		info += fmt.Sprintf(" | 🔍 Search: %s", m.filterTag)
	}

	var statusMessage string
	if m.statusMsg != "" && time.Now().Before(m.statusExpiry) {
		if strings.Contains(m.statusMsg, "❌") || strings.Contains(m.statusMsg, "Failed") {
			statusMessage = errorStyle.Render("Status: " + m.statusMsg)
		} else {
			statusMessage = successStyle.Render("Status: " + m.statusMsg)
		}
	}

	var footer string
	navStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#60A5FA"))
	actionStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#34D399"))
	editStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FBBF24"))
	systemStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#F87171"))
	if m.editMode && m.editField == 2 {
		editStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#F59E0B")).
			Bold(true)
		footer = editStyle.Render(fmt.Sprintf("🔍 Editing Search: %s", m.textInput.View())) +
			helpStyle.Render(" | Commands: enter: save • esc: cancel")
	} else if m.filterTag != "" {
		filterStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#10B981")).
			Bold(true)
		footer = filterStyle.Render(fmt.Sprintf("🔍 Search Applied: %s", m.filterTag)) +
			helpStyle.Render(" | Commands: esc: clear search • f: change search • ") + actionStyle.
			Render("space/enter/v: view") + helpStyle.Render(" • ") + editStyle.Render("\ne: edit • d: delete") + helpStyle.Render(" • ") + systemStyle.Render("r: refresh")
	} else {
		commandsHelp := []string{
			navStyle.Render("↑↓: navigate"),
			actionStyle.Render("space/enter/v: view"),
			actionStyle.Render("i: details"),
			editStyle.Render("e: edit"),
			editStyle.Render("n/a: add"),
			editStyle.Render("f: search"),
			editStyle.Render("t: template"),
			systemStyle.Render("d: delete"),
			systemStyle.Render("r: refresh"),
			systemStyle.Render("esc: menu"),
		}
		footer = helpStyle.Render("Commands: " + strings.Join(commandsHelp[:3], " • ") + " • " + strings.Join(commandsHelp[3:], " • "))
	}

	var parts []string
	parts = append(parts, stats)
	parts = append(parts, info)
	parts = append(parts, m.table.View())

	if statusMessage != "" {
		parts = append(parts, statusMessage)
	}

	parts = append(parts, footer)

	return lipgloss.JoinVertical(lipgloss.Left, parts...)
}

func (m model) viewDetails() string {
	if m.selectedRes == nil {
		titleStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#7C3AED")).
			Bold(true)
		title := titleStyle.Render("📄 Resource Details")

		content := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#6B7280")).
			Render("No resource selected")

		footer := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#60A5FA")).
			Render("Commands: ") +
			lipgloss.NewStyle().Foreground(lipgloss.Color("#F87171")).Render("esc: back")

		return lipgloss.JoinVertical(lipgloss.Left, title, "", content, "", footer)
	}

	titleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#7C3AED")).
		Bold(true)
	title := titleStyle.Render("📄 " + m.selectedRes.Name)

	detailStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#E5E7EB"))

	details := fmt.Sprintf("📁 %s\n", m.selectedRes.Path)
	details += fmt.Sprintf("🏷️  %s\n", m.selectedRes.Type)
	details += fmt.Sprintf("📊 %s\n", formatSize(m.selectedRes.Size))
	details += fmt.Sprintf("🕒 %s\n", m.selectedRes.ModTime.Format("2006-01-02 15:04:05"))

	if len(m.selectedRes.Tags) > 0 {
		details += fmt.Sprintf("🏷️  Tags: %s\n", strings.Join(m.selectedRes.Tags, ", "))
	}

	if m.selectedRes.Description != "" {
		details += fmt.Sprintf("📝 %s\n", m.selectedRes.Description)
	}

	details += "\n" + strings.Repeat("─", min(50, m.width-4)) + "\n"

	footerParts := []string{
		lipgloss.NewStyle().Foreground(lipgloss.Color("#60A5FA")).Render("Commands: "),
		lipgloss.NewStyle().Foreground(lipgloss.Color("#34D399")).Render("↑↓: scroll"),
		lipgloss.NewStyle().Foreground(lipgloss.Color("#6B7280")).Render(" • "),
	}

	if !m.fromGlobal {
		footerParts = append(footerParts,
			lipgloss.NewStyle().Foreground(lipgloss.Color("#FBBF24")).Render("e: edit"),
			lipgloss.NewStyle().Foreground(lipgloss.Color("#6B7280")).Render(" • "),
		)
	}

	footerParts = append(footerParts,
		lipgloss.NewStyle().Foreground(lipgloss.Color("#F87171")).Render("esc: back"))

	footer := strings.Join(footerParts, "")

	viewportStyle := lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#374151")).
		Padding(0, 1)

	viewportContent := viewportStyle.Render(m.viewport.View())

	return lipgloss.JoinVertical(lipgloss.Left,
		title,
		"",
		detailStyle.Render(details),
		viewportContent,
		"",
		footer,
	)
}

func (m model) viewCreate() string {
	titleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#7C3AED")).
		Bold(true)
	title := titleStyle.Render("📄 Create New Resource")

	contentStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#E5E7EB"))

	content := contentStyle.Render(fmt.Sprintf("Create new resource in: %s\n\nFilename: %s",
		m.currentDir, m.textInput.View()))

	footer := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#60A5FA")).
		Render("Commands: ") +
		lipgloss.NewStyle().Foreground(lipgloss.Color("#34D399")).Render("enter: create") +
		lipgloss.NewStyle().Foreground(lipgloss.Color("#6B7280")).Render(" • ") +
		lipgloss.NewStyle().Foreground(lipgloss.Color("#F87171")).Render("esc: cancel")

	return lipgloss.JoinVertical(lipgloss.Left, title, "", content, "", footer)
}

func (m model) viewPlaceholder(feature, emoji, description string) string {
	if m.viewMode == ViewGlobalResourcesPlaceholder {
		return m.viewGlobalResources()
	}

	titleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#7C3AED")).
		Bold(true)
	title := titleStyle.Render(feature)

	contentStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#6B7280")).
		MarginTop(1).
		MarginBottom(1)

	content := contentStyle.Render(emoji + "\n\n" + description)

	footer := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#60A5FA")).
		Render("Commands: ") +
		lipgloss.NewStyle().Foreground(lipgloss.Color("#F87171")).Render("esc: back to menu")

	return lipgloss.JoinVertical(lipgloss.Left, title, content, footer)
}

func (m model) viewGlobalResources() string {
	titleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#7C3AED")).
		Bold(true)
	title := titleStyle.Render("🌐 Global Resources")

	infoStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#9CA3AF"))
	dwightDir := filepath.Dir(m.config.TemplatesDir)
	info := infoStyle.Render(fmt.Sprintf("📁 %s", dwightDir))

	if len(m.globalRes) == 0 {
		emptyStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#6B7280")).
			MarginTop(1).
			MarginBottom(1)

		content := emptyStyle.Render("📋 No global resources found.\n\n💡 Add files to your global dwight directory!")
		footer := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#60A5FA")).
			Render("Commands: ") +
			lipgloss.NewStyle().Foreground(lipgloss.Color("#34D399")).Render("r: refresh") +
			lipgloss.NewStyle().Foreground(lipgloss.Color("#6B7280")).Render(" • ") +
			lipgloss.NewStyle().Foreground(lipgloss.Color("#F87171")).Render("esc: back")

		return lipgloss.JoinVertical(lipgloss.Left, title, "", info, "", content, "", footer)
	}

	selectedStyle := lipgloss.NewStyle().
		Background(lipgloss.Color("#7C3AED")).
		Foreground(lipgloss.Color("#F3F4F6")).
		Bold(true)

	normalStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#E5E7EB"))

	var rows []string
	for i, res := range m.globalRes {
		size := formatSize(res.Size)
		modTime := res.ModTime.Format("01-02 15:04")

		row := fmt.Sprintf("%-25s %-12s %10s %15s",
			res.Name,
			res.Type,
			size,
			modTime)

		if i == m.cursor {
			row = selectedStyle.Render("> " + row)
		} else {
			row = normalStyle.Render("  " + row)
		}

		rows = append(rows, row)
	}

	headerStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#F3F4F6")).
		Background(lipgloss.Color("#1F2937")).
		Bold(true)

	header := headerStyle.Render(fmt.Sprintf("  %-25s %-12s %10s %15s", "Name", "Type", "Size", "Modified"))

	statsStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#9CA3AF"))
	stats := statsStyle.Render(fmt.Sprintf("📊 %d global resources", len(m.globalRes)))

	navStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#60A5FA"))
	actionStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#34D399"))
	systemStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#F87171"))

	footer := lipgloss.NewStyle().Foreground(lipgloss.Color("#60A5FA")).Render("Commands: ") +
		navStyle.Render("↑↓: navigate") +
		lipgloss.NewStyle().Foreground(lipgloss.Color("#6B7280")).Render(" • ") +
		actionStyle.Render("enter/v: view") +
		lipgloss.NewStyle().Foreground(lipgloss.Color("#6B7280")).Render(" • ") +
		actionStyle.Render("r: refresh") +
		lipgloss.NewStyle().Foreground(lipgloss.Color("#6B7280")).Render(" • ") +
		systemStyle.Render("esc: back")

	var statusMessage string
	if m.statusMsg != "" && time.Now().Before(m.statusExpiry) {
		if strings.Contains(m.statusMsg, "❌") || strings.Contains(m.statusMsg, "Failed") {
			statusMessage = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#EF4444")).
				Bold(true).
				Render("Status: " + m.statusMsg)
		} else {
			statusMessage = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#10B981")).
				Bold(true).
				Render("Status: " + m.statusMsg)
		}
	}

	var parts []string
	parts = append(parts, title)
	parts = append(parts, info)
	parts = append(parts, stats)
	parts = append(parts, header)
	parts = append(parts, strings.Join(rows, "\n"))

	if statusMessage != "" {
		parts = append(parts, statusMessage)
	}

	parts = append(parts, footer)

	return lipgloss.JoinVertical(lipgloss.Left, parts...)
}

func (m model) viewHelp() string {
	titleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#7C3AED")).
		Bold(true)
	title := titleStyle.Render("❓ Help - Resource Manager")

	helpContent := `
KEYBINDINGS:

Navigation:
    ↑/↓           Navigate resources
    Enter/Space/V View resource details
Actions:
    n/a           Create new resource
    e             Edit resource description
    d             Delete resource
    c             Create template from resource
    t             Regenerate project template file
    
Filters & Search:
    f             Search by name, tags, description, or type
    r             Refresh resource scan
    
Other:
    ?             Toggle this help
    esc           Back to menu
    q             Quit application

RESOURCE TYPES:
    template      Template files for reuse
    prompt        AI prompt files
    context       Context/background files
    dataset       Data files
    resource      General resource files

PROJECT METADATA:
    Project metadata stored in .dwight.json
    Contains tags, descriptions, and settings
    Automatically detects project root (.git, package.json, etc.)
    
CONFIGURATION:
    Config file: ~/.config/dwight/config.json
    Templates:   ~/.config/dwight/templates/
    
    Supported file types: .md, .txt, .json, .yaml, .yml, .py, .js, .ts
`

	contentStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#E5E7EB"))

	footer := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#60A5FA")).
		Render("Commands: ") +
		lipgloss.NewStyle().Foreground(lipgloss.Color("#F87171")).Render("?: close help")

	return lipgloss.JoinVertical(lipgloss.Left,
		title,
		contentStyle.Render(helpContent),
		footer,
	)
}

func (m model) viewChat() string {
	// Header
	profile := m.getCurrentProfile()
	headerStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#7C3AED")).
		Bold(true).
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#374151")).
		Padding(0, 1)
	header := headerStyle.Render(fmt.Sprintf("🤖 Ollama Chat - %s (%s)", profile.Name, profile.Model))
	modelHint := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#6B7280")).
		Italic(true).
		Render("Press Tab to change model")
	// Footer
	footerStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#60A5FA")).
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#374151")).
		Padding(0, 1)

	var footer string
	switch m.chatState {
	case ChatStateInit, ChatStateCheckingModel:
		footer = footerStyle.Render("Checking model availability...")
	case ChatStateError:
		footer = footerStyle.Render("❌ Error - Press Esc to return to menu")
	case ChatStateReady:
		footer = footerStyle.Render("Type your message: " + m.chatInput.View() + " | Enter: send, Esc: menu")
	case ChatStateLoading:
		footer = footerStyle.Render(fmt.Sprintf("%s Thinking...", m.chatSpinner.View()))
	}

	// Content area with viewport
	contentStyle := lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#374151")).
		Padding(0, 1)

	var content string
	switch m.chatState {
	case ChatStateInit, ChatStateCheckingModel:
		content = contentStyle.Render(fmt.Sprintf("%s Checking model availability...\n", m.chatSpinner.View()))
	case ChatStateError:
		content = contentStyle.Render(fmt.Sprintf("❌ Error: %v\n", m.chatErr))
	default:
		content = contentStyle.Render(m.chatViewport.View())
	}

	// Layout: header + content + footer
	return lipgloss.JoinVertical(lipgloss.Left, header, modelHint, content, footer)
}

func (m model) editView() string {
	if m.selectedRes == nil || len(m.inputs) == 0 {
		return ""
	}

	var fields []string
	labels := []string{"Description:", "Tags:", "Type:"}

	for i, input := range m.inputs {
		labelStyle := lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#34D399"))

		label := labelStyle.Render(labels[i])
		fields = append(fields, label+"\n"+input.View())
	}

	content := lipgloss.JoinVertical(lipgloss.Top, fields...)

	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#7C3AED"))
	header := headerStyle.Render("✏️ Editing Resource: " + m.selectedRes.Name)

	keyStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FBBF24"))
	actionStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#60A5FA"))
	bulletStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#6B7280"))

	footer := keyStyle.Render("tab") + ": " + actionStyle.Render("next field") + " " +
		bulletStyle.Render("•") + " " + keyStyle.Render("shift+tab") + ": " +
		actionStyle.Render("prev field") + " " + bulletStyle.Render("•") + " " +
		keyStyle.Render("enter") + ": " + actionStyle.Render("save") + " " +
		bulletStyle.Render("•") + " " + keyStyle.Render("esc") + ": " +
		actionStyle.Render("cancel")

	return lipgloss.JoinVertical(lipgloss.Top,
		header,
		"",
		content,
		"",
		footer,
	)
}
