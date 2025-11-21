package main

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// renderChatHeader renders the enhanced chat header with model info and stats
func (m model) renderChatHeader() string {
	profile := m.getCurrentProfile()

	headerBox := lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#374151")).
		Padding(0, 1).
		Width(m.width - 4)

	// Model info
	modelStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#7C3AED")).
		Bold(true)

	// Stats style
	statsStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#60A5FA"))

	// Calculate total tokens in conversation
	totalTokens := 0
	promptTokens := 0
	for _, msg := range m.chatMessages {
		totalTokens += msg.TotalTokens
		promptTokens += msg.PromptTokens
	}

	// Context window info
	contextLimit := getContextWindowSize(profile.Model)
	contextPercent := 0
	if contextLimit > 0 {
		contextPercent = (totalTokens * 100) / contextLimit
	}

	// Build header content
	var header strings.Builder
	header.WriteString(modelStyle.Render(fmt.Sprintf("🤖 %s", profile.Name)))
	header.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#6B7280")).Render(" | "))
	header.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#34D399")).Render(profile.Model))

	// Stats line
	header.WriteString("\n")
	header.WriteString(statsStyle.Render(fmt.Sprintf("📊 Messages: %d | Tokens: %d/%d (%d%%) | Temp: %.1f",
		len(m.chatMessages),
		totalTokens,
		contextLimit,
		contextPercent,
		profile.Temperature)))

	// Show attached resources if any
	if len(m.attachedResources) > 0 {
		header.WriteString("\n")
		resourcesStr := strings.Join(m.attachedResources, ", ")
		if len(resourcesStr) > 60 {
			resourcesStr = resourcesStr[:57] + "..."
		}
		header.WriteString(lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FBBF24")).
			Render(fmt.Sprintf("📎 Context: %s", resourcesStr)))
	}

	// Show current conversation if loaded
	if m.currentConversation != nil {
		header.WriteString("\n")
		header.WriteString(lipgloss.NewStyle().
			Foreground(lipgloss.Color("#A78BFA")).
			Render(fmt.Sprintf("💾 %s", m.currentConversation.Title)))
	}

	return headerBox.Render(header.String())
}

// renderChatFooter renders the enhanced chat footer with helpful shortcuts
func (m model) renderChatFooter() string {
	footerStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#6B7280"))

	commandStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#34D399"))

	var shortcuts strings.Builder
	shortcuts.WriteString(commandStyle.Render("Enter"))
	shortcuts.WriteString(footerStyle.Render(": send | "))
	shortcuts.WriteString(commandStyle.Render("Ctrl+L"))
	shortcuts.WriteString(footerStyle.Render(": clear | "))
	shortcuts.WriteString(commandStyle.Render("Ctrl+S"))
	shortcuts.WriteString(footerStyle.Render(": save | "))
	shortcuts.WriteString(commandStyle.Render("Ctrl+O"))
	shortcuts.WriteString(footerStyle.Render(": open | "))
	shortcuts.WriteString(commandStyle.Render("Ctrl+R"))
	shortcuts.WriteString(footerStyle.Render(": attach | "))
	shortcuts.WriteString(commandStyle.Render("Tab"))
	shortcuts.WriteString(footerStyle.Render(": next model | "))
	shortcuts.WriteString(commandStyle.Render("Esc"))
	shortcuts.WriteString(footerStyle.Render(": menu"))

	return shortcuts.String()
}

// renderMessageWithTokens renders a single message with token information
func (m model) renderMessageWithTokens(msg ChatMessage, index int) []string {
	var lines []string

	// Message header with role and timestamp
	var header string
	if msg.Role == "user" {
		userStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#60A5FA")).
			Bold(true)
		header = userStyle.Render(fmt.Sprintf("👤 You • %s", msg.Timestamp.Format("3:04 PM")))
	} else {
		assistantStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#34D399")).
			Bold(true)

		// Add token and timing info for assistant messages
		tokenInfo := ""
		if msg.Duration > 0 && msg.TotalTokens > 0 {
			responseTokens := msg.TotalTokens - msg.PromptTokens
			tokenInfo = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#A78BFA")).
				Render(fmt.Sprintf(" • %.1fs | prompt: %d, response: %d",
					msg.Duration.Seconds(),
					msg.PromptTokens,
					responseTokens))
		}

		header = assistantStyle.Render(fmt.Sprintf("🤖 Assistant • %s", msg.Timestamp.Format("3:04 PM"))) + tokenInfo
	}

	lines = append(lines, header)

	// Message content with formatting
	contentLines := strings.Split(formatMessageContent(msg.Content), "\n")
	for _, line := range contentLines {
		lines = append(lines, "  "+line)
	}

	// Add spacing between messages
	lines = append(lines, "")

	return lines
}

// renderAttachedResourcesPicker shows available resources to attach
func (m model) renderAttachedResourcesPicker() string {
	titleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#A78BFA")).
		Bold(true)

	title := titleStyle.Render("📎 Attach Resources to Chat")

	selectedStyle := lipgloss.NewStyle().
		Background(lipgloss.Color("#7C3AED")).
		Foreground(lipgloss.Color("#FFFFFF")).
		Bold(true)

	normalStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FFFFFF"))

	attachedStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#10B981"))

	dimStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#9CA3AF"))

	var content strings.Builder
	content.WriteString(dimStyle.Render("Select resources to provide context to the AI:\n\n"))

	// Show available resources
	for i, res := range m.resources {
		isAttached := false
		for _, attached := range m.attachedResources {
			if attached == res.Path {
				isAttached = true
				break
			}
		}

		indicator := "  "
		if isAttached {
			indicator = "✓ "
		}

		line := fmt.Sprintf("%s%-30s | %s | %s",
			indicator,
			truncate(res.Name, 28),
			res.Type,
			formatBytes(res.Size))

		if i == m.cursor {
			content.WriteString(selectedStyle.Render("> " + line))
		} else if isAttached {
			content.WriteString(attachedStyle.Render("  " + line))
		} else {
			content.WriteString(normalStyle.Render("  " + line))
		}
		content.WriteString("\n")
	}

	footer := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#60A5FA")).
		Render("\nCommands: ") +
		lipgloss.NewStyle().Foreground(lipgloss.Color("#10B981")).Render("↑↓/Space/Enter") +
		lipgloss.NewStyle().Foreground(lipgloss.Color("#9CA3AF")).Render(" • ") +
		lipgloss.NewStyle().Foreground(lipgloss.Color("#FBBF24")).Render("a: all, c: clear") +
		lipgloss.NewStyle().Foreground(lipgloss.Color("#9CA3AF")).Render(" • ") +
		lipgloss.NewStyle().Foreground(lipgloss.Color("#F87171")).Render("esc")

	return lipgloss.JoinVertical(lipgloss.Left, title, "", content.String(), footer)
}

// renderChatStateMessage renders status messages for different chat states
func (m model) renderChatStateMessage() string {
	errorStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#EF4444")).
		Bold(true)

	warningStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FBBF24")).
		Bold(true)

	infoStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#60A5FA"))

	switch m.chatState {
	case ChatStateInit:
		return infoStyle.Render("🔄 Initializing chat...")
	case ChatStateCheckingModel:
		return infoStyle.Render(fmt.Sprintf("%s Checking if model is available...", m.chatSpinner.View()))
	case ChatStateModelNotAvailable:
		return warningStyle.Render(fmt.Sprintf("⚠️  Model '%s' not found. Press 'p' to pull it or 'esc' to go back.", m.modelPullName))
	case ChatStateLoading:
		return infoStyle.Render(fmt.Sprintf("%s Waiting for response...", m.chatSpinner.View()))
	case ChatStateError:
		if m.chatErr != nil {
			return errorStyle.Render(fmt.Sprintf("❌ Error: %v", m.chatErr))
		}
		return errorStyle.Render("❌ An error occurred")
	case ChatStateReady:
		return infoStyle.Render("✅ Ready to chat! Type your message below.")
	default:
		return ""
	}
}

// renderModelQuickSwitcher shows available models for quick switching
func (m model) renderModelQuickSwitcher() string {
	var models strings.Builder
	models.WriteString(lipgloss.NewStyle().
		Foreground(lipgloss.Color("#7C3AED")).
		Bold(true).
		Render("Available Models: "))

	for i, profile := range m.modelConfig.Profiles {
		if i == m.modelConfig.CurrentProfile {
			models.WriteString(lipgloss.NewStyle().
				Background(lipgloss.Color("#7C3AED")).
				Foreground(lipgloss.Color("#F3F4F6")).
				Bold(true).
				Render(" " + profile.Name + " "))
		} else {
			models.WriteString(lipgloss.NewStyle().
				Foreground(lipgloss.Color("#60A5FA")).
				Render(" " + profile.Name + " "))
		}
		if i < len(m.modelConfig.Profiles)-1 {
			models.WriteString(" ")
		}
	}

	return models.String()
}
