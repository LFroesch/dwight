package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// updateConversationList handles key events in the conversation list view
func (m model) updateConversationList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "q":
		m.viewMode = ViewMenu
		m.conversationSearch = ""
		return m, nil

	case "up", "k":
		if m.selectedConv > 0 {
			m.selectedConv--
		}
		return m, nil

	case "down", "j":
		if m.selectedConv < len(m.conversations)-1 {
			m.selectedConv++
		}
		return m, nil

	case "enter":
		// Load selected conversation
		if m.selectedConv < len(m.conversations) {
			conv := m.conversations[m.selectedConv]
			if err := m.loadConversationIntoChat(conv.ID); err == nil {
				m.viewMode = ViewChat
				m.chatState = ChatStateReady
				m.chatTextArea.Focus()
				return m, showStatus(fmt.Sprintf("📂 Loaded: %s", conv.Title))
			} else {
				return m, showStatus(fmt.Sprintf("❌ Failed to load conversation: %v", err))
			}
		}
		return m, nil

	case "d":
		// Delete conversation
		if m.selectedConv < len(m.conversations) {
			conv := m.conversations[m.selectedConv]
			if err := m.deleteConversation(conv.ID); err == nil {
				// Reload conversation list
				if convs, err := m.listConversations(); err == nil {
					m.conversations = convs
					if m.selectedConv >= len(m.conversations) && m.selectedConv > 0 {
						m.selectedConv--
					}
				}
				return m, showStatus(fmt.Sprintf("🗑️ Deleted: %s", conv.Title))
			} else {
				return m, showStatus(fmt.Sprintf("❌ Failed to delete: %v", err))
			}
		}
		return m, nil

	case "e":
		// Export conversation
		if m.selectedConv < len(m.conversations) {
			m.viewMode = ViewConversationExport
		}
		return m, nil

	case "/":
		// Search conversations (TODO: implement search input)
		return m, showStatus("🔍 Search coming soon...")
	}

	return m, nil
}

// updateConversationExport handles key events in the export view
func (m model) updateConversationExport(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.viewMode = ViewConversationList
		return m, nil

	case "1":
		// Export as Markdown
		return m, m.exportCurrentConversation("markdown")

	case "2":
		// Export as JSON
		return m, m.exportCurrentConversation("json")

	case "3":
		// Export as Plain Text
		return m, m.exportCurrentConversation("text")
	}

	return m, nil
}

// exportCurrentConversation exports the selected conversation
func (m *model) exportCurrentConversation(format string) tea.Cmd {
	return func() tea.Msg {
		if m.selectedConv >= len(m.conversations) {
			return statusMsg{message: "❌ No conversation selected"}
		}

		conv := m.conversations[m.selectedConv]
		exportDir := filepath.Join(m.currentDir, "exports")
		os.MkdirAll(exportDir, 0755)

		var content string
		var extension string
		var err error

		switch format {
		case "markdown":
			content, err = m.exportConversationMarkdown(conv.ID)
			extension = ".md"
		case "json":
			content, err = m.exportConversationJSON(conv.ID)
			extension = ".json"
		case "text":
			// Plain text export
			fullConv, loadErr := m.loadConversation(conv.ID)
			if loadErr != nil {
				return statusMsg{message: fmt.Sprintf("❌ Failed to load: %v", loadErr)}
			}
			var text strings.Builder
			text.WriteString(fmt.Sprintf("%s\n", fullConv.Title))
			text.WriteString(fmt.Sprintf("Model: %s\n", fullConv.Model))
			text.WriteString(strings.Repeat("=", 50) + "\n\n")
			for _, msg := range fullConv.Messages {
				if msg.Role == "user" {
					text.WriteString("USER:\n")
				} else {
					text.WriteString("ASSISTANT:\n")
				}
				text.WriteString(msg.Content + "\n\n")
				text.WriteString(strings.Repeat("-", 30) + "\n\n")
			}
			content = text.String()
			extension = ".txt"
		default:
			return statusMsg{message: "❌ Unknown export format"}
		}

		if err != nil {
			return statusMsg{message: fmt.Sprintf("❌ Export failed: %v", err)}
		}

		// Create safe filename
		safeTitle := strings.ReplaceAll(conv.Title, "/", "-")
		safeTitle = strings.ReplaceAll(safeTitle, " ", "_")
		if len(safeTitle) > 50 {
			safeTitle = safeTitle[:50]
		}

		filename := filepath.Join(exportDir, fmt.Sprintf("%s_%s%s",
			safeTitle,
			conv.Created.Format("20060102_150405"),
			extension))

		if err := os.WriteFile(filename, []byte(content), 0644); err != nil {
			return statusMsg{message: fmt.Sprintf("❌ Failed to write file: %v", err)}
		}

		return statusMsg{message: fmt.Sprintf("✅ Exported to: %s", filename)}
	}
}

// updateChatEnhanced adds new keyboard shortcuts for conversation management
func (m model) updateChatEnhanced(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Handle resource picker view
	if m.showResourcePicker {
		return m.updateResourcePicker(msg)
	}

	switch msg.String() {
	case "ctrl+o":
		// Open conversation list
		if m.chatState == ChatStateReady {
			if convs, err := m.listConversations(); err == nil {
				m.conversations = convs
				m.selectedConv = 0
				m.viewMode = ViewConversationList
				return m, nil
			} else {
				return m, showStatus(fmt.Sprintf("❌ Failed to load conversations: %v", err))
			}
		}

	case "ctrl+s":
		// Save current conversation
		if len(m.chatMessages) > 0 {
			if err := m.saveCurrentChat(); err == nil {
				return m, showStatus("💾 Conversation saved")
			} else {
				return m, showStatus(fmt.Sprintf("❌ Failed to save: %v", err))
			}
		}

	case "ctrl+r":
		// Toggle resource picker for RAG
		if m.chatState == ChatStateReady {
			m.showResourcePicker = !m.showResourcePicker
			m.cursor = 0
			m.scanResources() // Refresh resources
			return m, nil
		}

	case "ctrl+t":
		// Trim conversation to fit context window
		if len(m.chatMessages) > 0 {
			beforeCount := len(m.chatMessages)
			m.trimConversationToContext()
			afterCount := len(m.chatMessages)
			if beforeCount != afterCount {
				m.updateChatLines()
				return m, showStatus(fmt.Sprintf("✂️ Trimmed %d messages", beforeCount-afterCount))
			} else {
				return m, showStatus("✅ Conversation fits in context window")
			}
		}

	case "ctrl+n":
		// New conversation (clear current)
		if m.chatState == ChatStateReady {
			// Save current conversation if it has messages
			if len(m.chatMessages) > 0 {
				m.saveCurrentChat()
			}
			// Clear everything for new conversation
			m.chatMessages = []ChatMessage{}
			m.currentConversation = nil
			m.attachedResources = []string{}
			m.chatStreamBuffer.Reset()
			m.updateChatLines()
			return m, showStatus("📝 New conversation started")
		}
	}

	// If no custom shortcut was handled, use original updateChat
	return m.updateChat(msg)
}

// updateResourcePicker handles the resource picker view for RAG
func (m model) updateResourcePicker(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.showResourcePicker = false
		m.chatTextArea.Focus()
		return m, nil

	case "enter":
		m.showResourcePicker = false
		m.chatTextArea.Focus()
		return m, showStatus(fmt.Sprintf("📎 %d resources attached", len(m.attachedResources)))

	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
		return m, nil

	case "down", "j":
		if m.cursor < len(m.resources)-1 {
			m.cursor++
		}
		return m, nil

	case " ":
		// Toggle resource attachment
		if m.cursor < len(m.resources) {
			res := m.resources[m.cursor]
			isAttached := false
			attachIndex := -1

			// Check if already attached
			for i, path := range m.attachedResources {
				if path == res.Path {
					isAttached = true
					attachIndex = i
					break
				}
			}

			if isAttached {
				// Remove from attached
				m.attachedResources = append(
					m.attachedResources[:attachIndex],
					m.attachedResources[attachIndex+1:]...)
			} else {
				// Add to attached
				m.attachedResources = append(m.attachedResources, res.Path)
			}
		}
		return m, nil

	case "a":
		// Attach all resources
		m.attachedResources = []string{}
		for _, res := range m.resources {
			m.attachedResources = append(m.attachedResources, res.Path)
		}
		return m, showStatus(fmt.Sprintf("📎 Attached all %d resources", len(m.attachedResources)))

	case "c":
		// Clear all attachments
		m.attachedResources = []string{}
		return m, showStatus("🗑️ Cleared all attachments")
	}

	return m, nil
}

// getAttachedResourcesContent reads and concatenates attached resource contents
func (m *model) getAttachedResourcesContent() string {
	if len(m.attachedResources) == 0 {
		return ""
	}

	var content strings.Builder
	content.WriteString("=== ATTACHED RESOURCES ===\n\n")

	for _, path := range m.attachedResources {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}

		content.WriteString(fmt.Sprintf("--- File: %s ---\n", filepath.Base(path)))
		content.WriteString(string(data))
		content.WriteString("\n\n")
	}

	content.WriteString("=== END RESOURCES ===\n\n")
	return content.String()
}
