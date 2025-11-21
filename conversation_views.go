package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// viewConversationList displays all saved conversations
func (m model) viewConversationList() string {
	titleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#A78BFA")).
		Bold(true)
	title := titleStyle.Render("💬 Conversation History")

	selectedStyle := lipgloss.NewStyle().
		Background(lipgloss.Color("#7C3AED")).
		Foreground(lipgloss.Color("#FFFFFF")).
		Bold(true)

	normalStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FFFFFF"))

	dimStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#9CA3AF"))

	var content strings.Builder

	if m.conversationSearch != "" {
		content.WriteString(dimStyle.Render(fmt.Sprintf("🔍 Search: %s\n\n", m.conversationSearch)))
	}

	if len(m.conversations) == 0 {
		content.WriteString(dimStyle.Render("No conversations found. Start chatting to create one!\n"))
	} else {
		content.WriteString(dimStyle.Render(fmt.Sprintf("📚 %d conversations", len(m.conversations))))
		content.WriteString("\n\n")

		// Display conversations
		for i, conv := range m.conversations {
			// Format timestamp
			timeStr := formatTimeAgo(conv.LastModified)

			// Create conversation line
			line := fmt.Sprintf("%-50s | %s | %d msgs | %s",
				truncate(conv.Title, 48),
				truncate(conv.Model, 20),
				conv.MessageCount,
				timeStr)

			if i == m.selectedConv {
				content.WriteString(selectedStyle.Render("> " + line))
			} else {
				content.WriteString(normalStyle.Render("  " + line))
			}
			content.WriteString("\n")
		}
	}

	footer := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#60A5FA")).
		Render("\nCommands: ") +
		lipgloss.NewStyle().Foreground(lipgloss.Color("#10B981")).Render("↑↓/Enter") +
		lipgloss.NewStyle().Foreground(lipgloss.Color("#9CA3AF")).Render(" • ") +
		lipgloss.NewStyle().Foreground(lipgloss.Color("#FBBF24")).Render("d/e/s") +
		lipgloss.NewStyle().Foreground(lipgloss.Color("#9CA3AF")).Render(" • ") +
		lipgloss.NewStyle().Foreground(lipgloss.Color("#F87171")).Render("esc")

	return lipgloss.JoinVertical(lipgloss.Left, title, "", content.String(), footer)
}

// viewConversationExport displays export options
func (m model) viewConversationExport() string {
	titleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#A78BFA")).
		Bold(true)

	if m.selectedConv >= len(m.conversations) {
		return titleStyle.Render("❌ No conversation selected")
	}

	conv := m.conversations[m.selectedConv]
	title := titleStyle.Render(fmt.Sprintf("📤 Export: %s", conv.Title))

	optionStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFFFF"))
	options := []string{
		optionStyle.Render("1. Export as Markdown (.md)"),
		optionStyle.Render("2. Export as JSON (.json)"),
		optionStyle.Render("3. Export as Plain Text (.txt)"),
	}

	content := strings.Join(options, "\n")

	footer := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#60A5FA")).
		Render("\nCommands: ") +
		lipgloss.NewStyle().Foreground(lipgloss.Color("#10B981")).Render("1-3") +
		lipgloss.NewStyle().Foreground(lipgloss.Color("#9CA3AF")).Render(" • ") +
		lipgloss.NewStyle().Foreground(lipgloss.Color("#F87171")).Render("esc")

	return lipgloss.JoinVertical(lipgloss.Left, title, "", content, footer)
}

// Helper functions

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}

func formatTimeAgo(t time.Time) string {
	duration := time.Since(t)

	if duration < time.Minute {
		return "just now"
	}
	if duration < time.Hour {
		mins := int(duration.Minutes())
		if mins == 1 {
			return "1 min ago"
		}
		return fmt.Sprintf("%d mins ago", mins)
	}
	if duration < 24*time.Hour {
		hours := int(duration.Hours())
		if hours == 1 {
			return "1 hour ago"
		}
		return fmt.Sprintf("%d hours ago", hours)
	}
	if duration < 7*24*time.Hour {
		days := int(duration.Hours() / 24)
		if days == 1 {
			return "1 day ago"
		}
		return fmt.Sprintf("%d days ago", days)
	}
	if duration < 30*24*time.Hour {
		weeks := int(duration.Hours() / 24 / 7)
		if weeks == 1 {
			return "1 week ago"
		}
		return fmt.Sprintf("%d weeks ago", weeks)
	}
	if duration < 365*24*time.Hour {
		months := int(duration.Hours() / 24 / 30)
		if months == 1 {
			return "1 month ago"
		}
		return fmt.Sprintf("%d months ago", months)
	}

	years := int(duration.Hours() / 24 / 365)
	if years == 1 {
		return "1 year ago"
	}
	return fmt.Sprintf("%d years ago", years)
}

func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}
