package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"dwight/internal/ollama"
	"dwight/internal/storage"
	s "dwight/internal/styles"

	"github.com/charmbracelet/lipgloss"
)

func (m model) View() string {
	if m.showHelp {
		return m.renderHelp()
	}
	var content string
	switch m.viewMode {
	case ViewMenu:
		content = m.viewMenu()
	case ViewChat:
		if m.showResourcePicker {
			content = m.viewResourcePicker()
		} else {
			content = m.viewChat()
		}
		// Overlay @ autocomplete popup on top of chat
		if m.showAtComplete {
			content = m.overlayAtComplete(content)
		}
	case ViewConversationList:
		content = m.viewConversationList()
	case ViewConversationExport:
		content = m.viewConversationExport()
	case ViewSettings:
		content = m.viewSettings()
	case ViewModelManager:
		content = m.viewModelManager()
	case ViewModelCreate:
		content = m.viewModelCreate()
	case ViewModelPull:
		content = m.viewModelPull()
	case ViewModelLibrary:
		content = m.viewModelLibrary()
	case ViewConfirmDialog:
		content = m.viewConfirmDialog()
	default:
		return ""
	}
	return m.fitToTerminal(content)
}

func (m model) fitToTerminal(content string) string {
	if m.height <= 0 || m.width <= 0 {
		return content
	}
	lines := strings.Split(content, "\n")
	if len(lines) > m.height {
		lines = lines[:m.height]
	}
	return strings.Join(lines, "\n")
}

// =============================================================================
// Shared: header + status
// =============================================================================

func (m model) renderStatus() string {
	if m.statusMsg != "" && time.Now().Before(m.statusExp) {
		if strings.Contains(m.statusMsg, "Failed") || strings.Contains(m.statusMsg, "Cannot") {
			return s.Error.Render(m.statusMsg)
		}
		return s.Success.Render(m.statusMsg)
	}
	return ""
}

// =============================================================================
// Menu
// =============================================================================

func (m model) viewMenu() string {
	profile := ""
	if len(m.modelConfig.Profiles) > 0 {
		p := m.modelConfig.Current()
		profile = s.Dim.Render("provider/model: ") + s.Success.Render(fmt.Sprintf("%s · %s", storage.NormalizeProvider(p.Provider), p.Model))
	}

	headerLine := s.Title.Render("dwight")
	if profile != "" {
		headerLine += s.Dim.Render("  ·  ") + profile
	}
	subtitle := s.Dim.Render("terminal AI assistant")

	var items strings.Builder
	for i, item := range m.menuItems {
		if i == m.menuCursor {
			items.WriteString(s.Selected.Render(fmt.Sprintf(" %-26s", item.Name)))
			items.WriteString(s.Dim.Render(" " + item.Desc))
		} else {
			items.WriteString(s.Normal.Render(fmt.Sprintf("  %-25s", item.Name)))
			items.WriteString(s.Dim.Render(" " + item.Desc))
		}
		items.WriteString("\n")
	}

	menuBox := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(s.Purple).
		Padding(0, 1).
		Render(strings.TrimRight(items.String(), "\n"))

	footer := s.Footer("j/k", "navigate", "enter", "select", "q", "quit", "?", "help")
	status := m.renderStatus()

	parts := []string{headerLine, subtitle, "", menuBox}
	if status != "" {
		parts = append(parts, "", status)
	}
	parts = append(parts, "", footer)
	return lipgloss.JoinVertical(lipgloss.Left, parts...)
}

// =============================================================================
// Chat
// =============================================================================

func (m model) viewChat() string {
	profile := m.currentProfile()
	provider := storage.NormalizeProvider(profile.Provider)

	// Header: model name + context bar + stats
	header := s.Title.Render("dwight") +
		s.Dim.Render(" | profile: ") + s.Title.Render(profile.Name) +
		s.Dim.Render(" | provider: ") + s.Success.Render(provider) +
		s.Dim.Render(" | model: ") + s.Success.Render(profile.Model)

	// Use the last message's TotalTokens — Ollama reports cumulative session tokens per call.
	totalTokens := 0
	if len(m.chatMessages) > 0 {
		totalTokens = m.chatMessages[len(m.chatMessages)-1].TotalTokens
	}
	ctxSize := 0
	if provider == "ollama" {
		ctxSize = ollama.ContextWindowSize(profile.Model)
	}

	if totalTokens > 0 && ctxSize > 0 {
		// Context usage bar
		pct := float64(totalTokens) / float64(ctxSize)
		barWidth := 15
		filled := int(pct * float64(barWidth))
		if filled > barWidth {
			filled = barWidth
		}
		bar := strings.Repeat("█", filled) + strings.Repeat("░", barWidth-filled)
		barColor := s.Green
		if pct > 0.7 {
			barColor = s.Yellow
		}
		if pct > 0.9 {
			barColor = s.Red
		}
		header += s.Dim.Render(" [") +
			lipgloss.NewStyle().Foreground(barColor).Render(bar) +
			s.Dim.Render(fmt.Sprintf("] %dk/%dk", totalTokens/1000, ctxSize/1000))
	}

	// Show attached file count
	atFileCount := 0
	if len(m.chatMessages) > 0 {
		lastUserMsg := ""
		for i := len(m.chatMessages) - 1; i >= 0; i-- {
			if m.chatMessages[i].Role == "user" {
				lastUserMsg = m.chatMessages[i].Content
				break
			}
		}
		for _, word := range strings.Fields(lastUserMsg) {
			if strings.HasPrefix(word, "@") && len(word) > 1 {
				atFileCount++
			}
		}
	}
	if len(m.attachedResources) > 0 || atFileCount > 0 {
		total := len(m.attachedResources) + atFileCount
		header += s.Dim.Render(fmt.Sprintf(" | %d files", total))
	}

	// Scroll indicator
	if len(m.chatLines) > m.chatMaxLines {
		maxScroll := len(m.chatLines) - m.chatMaxLines
		header += s.Dim.Render(fmt.Sprintf(" [%d/%d]", m.chatScrollPos+m.chatMaxLines, len(m.chatLines)))
		_ = maxScroll
	}

	// Chat content
	var content []string
	switch m.chatState {
	case ChatStateInit, ChatStateCheckingModel:
		content = []string{"Checking model..."}
	case ChatStateModelNotAvailable:
		content = []string{s.Warning.Render(fmt.Sprintf("Ollama model '%s' not available. Y to pull, N to cancel", m.modelPullName))}
	case ChatStateError:
		errMsg := "Error occurred"
		if m.chatErr != nil {
			errMsg = m.chatErr.Error()
		}
		content = []string{s.Error.Render(errMsg)}
	case ChatStateReady, ChatStateLoading:
		content = m.getVisibleChatLines()
	case ChatStateReview:
		content = m.getVisibleChatLines()
	}

	// Input area
	var inputArea string
	switch m.chatState {
	case ChatStateReady:
		inputArea = m.viewChatComposer()
	case ChatStateLoading:
		inputArea = s.Warning.Render(fmt.Sprintf("%s Generating...", m.chatSpinner.View()))
	case ChatStateReview:
		inputArea = m.viewReviewBar()
	}

	var footer string
	switch {
	case m.chatCopyMode:
		footer = s.Footer("j/k", "navigate", "space", "mark", "y/enter", "copy", "esc", "cancel")
	case m.chatState == ChatStateReview:
		footer = s.Footer("a", "accept", "r", "refine", "n", "skip")
	case m.chatState == ChatStateLoading || m.chatStreaming:
		footer = s.Footer("esc", "interrupt", "ctrl+c", "interrupt")
	default:
		footer = s.Footer("enter", "send", "alt+enter", "newline", "up/down", "move cursor", "pgup/dn", "scroll chat", "ctrl+y", "copy msg", "ctrl+n", "new")
	}
	status := m.renderStatus()

	result := header + "\n\n" + strings.Join(content, "\n")
	if inputArea != "" {
		result += "\n\n" + inputArea
	}
	if status != "" {
		result += "\n" + status
	}
	result += "\n" + footer
	return result
}

func (m model) viewChatComposer() string {
	lineCount := m.chatComposerLineCount(m.chatTextArea.Width())
	meta := []string{
		s.Dim.Render(fmt.Sprintf("draft %d line", lineCount)),
		s.Dim.Render("enter sends"),
		s.Dim.Render("alt+enter adds a newline"),
		s.Dim.Render("up/down moves inside the draft"),
		s.Dim.Render("pgup/pgdn scrolls chat history"),
	}
	if hidden := lineCount - m.chatTextArea.Height(); hidden > 0 {
		meta = append(meta, s.Warning.Render(fmt.Sprintf("%d more line(s) above", hidden)))
	}

	borderColor := s.DarkGray
	if m.chatState == ChatStateReady {
		borderColor = s.Purple
	}

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Padding(0, 1).
		Render(m.chatTextArea.View())

	return strings.Join(meta, s.Dim.Render("  ·  ")) + "\n" + box
}

// =============================================================================
// Conversations
// =============================================================================

func (m model) viewConversationList() string {
	title := s.Title.Render("Conversation History")
	status := m.renderStatus()

	var content strings.Builder
	if len(m.conversations) == 0 {
		content.WriteString(s.Dim.Render("No conversations yet. Start chatting!"))
	} else {
		lib := storage.ConversationsDir()
		content.WriteString(s.Dim.Render(fmt.Sprintf("%d conversations · %s\n\n", len(m.conversations), lib)))
		content.WriteString("\n")
		for i, conv := range m.conversations {
			timeStr := formatTimeAgo(conv.LastModified)
			where := conv.Where()
			if where == "" {
				where = "—"
			}
			line := fmt.Sprintf("%-30s | %-26s | %-12s | %d msgs | %s",
				truncateStr(conv.Title, 28),
				truncateStr(where, 24),
				truncateStr(conv.Model, 10),
				conv.MessageCount,
				timeStr)
			if i == m.selectedConv {
				content.WriteString(s.Selected.Render("> " + line))
			} else {
				content.WriteString(s.Normal.Render("  " + line))
			}
			content.WriteString("\n")
		}
	}

	footer := s.Footer("j/k", "navigate", "enter", "load", "d", "delete", "e", "export", "esc", "back")
	parts := []string{title, "", content.String()}
	if status != "" {
		parts = append(parts, "", status)
	}
	parts = append(parts, "", footer)
	return lipgloss.JoinVertical(lipgloss.Left, parts...)
}

func (m model) viewConversationExport() string {
	if m.selectedConv >= len(m.conversations) {
		return s.Error.Render("No conversation selected")
	}
	conv := m.conversations[m.selectedConv]
	title := s.Title.Render(fmt.Sprintf("Export: %s", conv.Title))
	options := s.Normal.Render("1. Markdown (.md)\n2. JSON (.json)")
	footer := s.Footer("1-2", "export", "esc", "back")
	status := m.renderStatus()
	parts := []string{title, "", options}
	if status != "" {
		parts = append(parts, "", status)
	}
	parts = append(parts, "", footer)
	return lipgloss.JoinVertical(lipgloss.Left, parts...)
}

// =============================================================================
// Settings
// =============================================================================

func (m model) viewSettings() string {
	title := s.Title.Render("Settings")
	labels := []string{"System Prompt:", "Your Name:", "Chat Timeout (seconds):"}
	var fields []string
	for i, input := range m.settingsInputs {
		label := s.Success.Render(labels[i])
		fields = append(fields, label+"\n"+input.View())
	}
	content := lipgloss.JoinVertical(lipgloss.Top, fields...)
	footer := s.Footer("tab", "next field", "enter", "save", "esc", "cancel")
	return lipgloss.JoinVertical(lipgloss.Left, title, "", content, "", footer)
}

// =============================================================================
// Model Manager
// =============================================================================

func (m model) viewModelManager() string {
	title := s.Title.Render("Model Manager")

	var content strings.Builder
	for i, p := range m.modelConfig.Profiles {
		indicator := "  "
		if i == m.modelConfig.CurrentProfile {
			indicator = "* "
		}
		line := fmt.Sprintf("%s%-18s | %-7s | %-23s | Temp: %.1f", indicator, p.Name, storage.NormalizeProvider(p.Provider), p.Model, p.Temperature)
		if i == m.modelSelection {
			content.WriteString(s.Selected.Render("> " + line))
		} else {
			content.WriteString(s.Normal.Render("  " + line))
		}
		content.WriteString("\n")
	}

	footer := s.Footer("j/k", "navigate", "enter", "set default", "n", "new", "e", "edit", "b", "browse Ollama library", "p", "pull Ollama model", "d", "delete", "esc", "back")
	return lipgloss.JoinVertical(lipgloss.Left, title, "", content.String(), "", footer)
}

func (m model) viewModelCreate() string {
	titleText := "New Profile"
	if m.editingProfile >= 0 {
		titleText = "Edit Profile"
	}
	title := s.Title.Render(titleText)

	labels := []string{"Name:", "Provider:", "Model:", "System Prompt:", "Temperature (0-1):"}
	var fields []string
	for i, input := range m.modelInputs {
		label := s.Success.Render(labels[i])
		fields = append(fields, label+"\n"+input.View())
	}
	content := lipgloss.JoinVertical(lipgloss.Top, fields...)
	footer := s.Footer("tab", "next field", "enter", "save", "esc", "cancel")
	return lipgloss.JoinVertical(lipgloss.Left, title, "", content, "", footer)
}

func (m model) viewModelPull() string {
	title := s.Title.Render("Pull Model")
	var content string
	if m.modelPullError != nil {
		content = s.Error.Render(fmt.Sprintf("Error pulling %s: %v", m.modelPullName, m.modelPullError))
	} else {
		content = s.Warning.Render(m.modelPullStatus)
	}
	footer := s.Footer("esc", "back")
	return lipgloss.JoinVertical(lipgloss.Left, title, "", content, "", footer)
}

func (m model) viewModelLibrary() string {
	title := s.Title.Render("Ollama Model Library")
	if m.libraryFilter != "" {
		title += s.Dim.Render("  filter: ") + s.Success.Render(m.libraryFilter)
	} else {
		title += s.Dim.Render("  (type to filter)")
	}

	var content strings.Builder
	models := m.getFilteredLibrary()

	if len(models) == 0 {
		content.WriteString(s.Dim.Render("No models match filter."))
	} else {
		maxVisible := (m.height - 10) / 2
		if maxVisible < 5 {
			maxVisible = 5
		}
		scrollOff := 0
		if m.librarySelection >= maxVisible {
			scrollOff = m.librarySelection - maxVisible + 1
		}
		end := scrollOff + maxVisible
		if end > len(models) {
			end = len(models)
		}

		for i := scrollOff; i < end; i++ {
			mdl := models[i]
			installed := ollama.IsModelInstalled(mdl.Name, m.installedModels)
			line := fmt.Sprintf("%-25s %s (%s)", mdl.Name, truncateStr(mdl.Description, 45), mdl.Size)

			if i == m.librarySelection {
				prefix := "> "
				if installed {
					prefix = "> * "
				}
				content.WriteString(s.Selected.Render(prefix + line))
			} else if installed {
				content.WriteString(s.Success.Render("* " + line))
			} else {
				content.WriteString(s.Normal.Render("  " + line))
			}
			content.WriteString("\n")

			if len(mdl.Tags) > 0 {
				content.WriteString(s.Dim.Render("    Tags: " + strings.Join(mdl.Tags, ", ")))
				content.WriteString("\n")
			}
		}

		if len(models) > maxVisible {
			content.WriteString(s.Dim.Render(fmt.Sprintf("\n[%d-%d of %d]", scrollOff+1, end, len(models))))
		}
	}

	footer := s.Footer("j/k", "navigate", "enter", "install", "type", "filter", "backspace", "clear filter", "r", "refresh", "esc", "back")
	return lipgloss.JoinVertical(lipgloss.Left, title, "", content.String(), "", footer)
}

// =============================================================================
// Code Block Review
// =============================================================================

func (m model) viewReviewBar() string {
	if len(m.codeBlocks) == 0 || m.reviewIndex >= len(m.codeBlocks) {
		return ""
	}

	// Find current actionable block
	idx := m.reviewIndex
	for idx < len(m.codeBlocks) && m.codeBlocks[idx].Path == "" {
		idx++
	}
	if idx >= len(m.codeBlocks) {
		return ""
	}

	block := m.codeBlocks[idx]
	var bar strings.Builder

	// Show what file is being proposed
	bar.WriteString(s.Title.Render(fmt.Sprintf("  %s", block.Path)))
	if block.Language != "" {
		bar.WriteString(s.Dim.Render(fmt.Sprintf(" (%s)", block.Language)))
	}
	bar.WriteString("\n")

	// Show diff if file exists, otherwise show preview
	fullPath := filepath.Join(m.currentDir, block.Path)
	if existing, err := os.ReadFile(fullPath); err == nil {
		diffLines := generateDiff(string(existing), block.Content, block.Path)
		maxDiff := 12
		if len(diffLines) > maxDiff+2 { // skip header lines
			diffLines = append(diffLines[:maxDiff+2], s.Dim.Render(fmt.Sprintf("  ... %d more lines", len(diffLines)-maxDiff-2)))
		}
		for _, line := range diffLines[2:] { // skip --- +++ headers
			if strings.HasPrefix(line, "+ ") {
				bar.WriteString(s.DiffAdd.Render(line) + "\n")
			} else if strings.HasPrefix(line, "- ") {
				bar.WriteString(s.DiffRemove.Render(line) + "\n")
			} else {
				bar.WriteString(s.Dim.Render(line) + "\n")
			}
		}
	} else {
		bar.WriteString(s.Success.Render("  [NEW FILE]") + "\n")
		preview := strings.Split(block.Content, "\n")
		maxPreview := 8
		if len(preview) > maxPreview {
			preview = append(preview[:maxPreview], s.Dim.Render(fmt.Sprintf("  ... %d more lines", len(preview)-maxPreview)))
		}
		for _, line := range preview {
			bar.WriteString(s.DiffAdd.Render("+ "+line) + "\n")
		}
	}

	// Count remaining
	remaining := 0
	for i := idx + 1; i < len(m.codeBlocks); i++ {
		if m.codeBlocks[i].Path != "" {
			remaining++
		}
	}
	if remaining > 0 {
		bar.WriteString(s.Dim.Render(fmt.Sprintf("  +%d more file(s)", remaining)) + "\n")
	}

	return bar.String()
}

// =============================================================================
// @ Autocomplete Overlay
// =============================================================================

func (m model) overlayAtComplete(base string) string {
	// Build the popup content
	var popup strings.Builder
	filterDisplay := m.atCompleteFilter
	if filterDisplay == "" {
		filterDisplay = ""
	}
	popup.WriteString(s.Title.Render("@ ") + s.Dim.Render(filterDisplay) + "\n")

	if len(m.atCompleteFiles) == 0 {
		popup.WriteString(s.Dim.Render("  no matches\n"))
	} else {
		maxShow := 10
		if maxShow > len(m.atCompleteFiles) {
			maxShow = len(m.atCompleteFiles)
		}

		// Scroll window around cursor
		start := 0
		if m.atCompleteCursor >= maxShow {
			start = m.atCompleteCursor - maxShow + 1
		}
		end := start + maxShow
		if end > len(m.atCompleteFiles) {
			end = len(m.atCompleteFiles)
			start = end - maxShow
			if start < 0 {
				start = 0
			}
		}

		for i := start; i < end; i++ {
			f := m.atCompleteFiles[i]
			if i == m.atCompleteCursor {
				popup.WriteString(s.Selected.Render("> "+f) + "\n")
			} else {
				popup.WriteString(s.Normal.Render("  "+f) + "\n")
			}
		}
		if len(m.atCompleteFiles) > maxShow {
			popup.WriteString(s.Dim.Render(fmt.Sprintf("  [%d/%d]", m.atCompleteCursor+1, len(m.atCompleteFiles))) + "\n")
		}
	}
	popup.WriteString(s.Footer("↑↓", "navigate", "tab/enter", "select", "esc", "cancel"))

	// Place popup above the textarea area (overlay on last N lines of base)
	popupStr := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(s.Purple).
		Padding(0, 1).
		Render(popup.String())

	baseLines := strings.Split(base, "\n")
	popupLines := strings.Split(popupStr, "\n")

	// Insert popup before the last few lines (textarea + footer)
	insertAt := len(baseLines) - 4
	if insertAt < 0 {
		insertAt = 0
	}

	var result []string
	result = append(result, baseLines[:insertAt]...)
	result = append(result, popupLines...)
	result = append(result, baseLines[insertAt:]...)
	return strings.Join(result, "\n")
}

// =============================================================================
// Resource Picker
// =============================================================================

func (m model) viewResourcePicker() string {
	title := s.Title.Render("Attach Resources")
	files := m.scanAttachableFiles()

	var content strings.Builder
	content.WriteString(s.Dim.Render("Space to toggle, Esc/Enter to close:\n\n"))

	for i, path := range files {
		isAttached := false
		for _, a := range m.attachedResources {
			if a == path {
				isAttached = true
				break
			}
		}
		indicator := "  "
		if isAttached {
			indicator = "* "
		}
		name := filepath.Base(path)
		line := fmt.Sprintf("%s%s", indicator, name)
		if i == m.pickerCursor {
			content.WriteString(s.Selected.Render("> " + line))
		} else if isAttached {
			content.WriteString(s.Success.Render("  " + line))
		} else {
			content.WriteString(s.Normal.Render("  " + line))
		}
		content.WriteString("\n")
	}

	footer := s.Footer("space", "toggle", "c", "clear all", "esc", "done")
	return lipgloss.JoinVertical(lipgloss.Left, title, "", content.String(), "", footer)
}

// =============================================================================
// Confirm Dialog
// =============================================================================

func (m model) viewConfirmDialog() string {
	if m.confirmDialog == nil {
		return ""
	}
	title := s.Warning.Render("Confirm")
	content := s.Normal.Render(m.confirmDialog.Message)
	footer := s.Footer("y", "yes", "n", "no", "esc", "cancel")
	return lipgloss.JoinVertical(lipgloss.Left, title, "", content, "", footer)
}

// =============================================================================
// Helpers
// =============================================================================

func truncateStr(str string, max int) string {
	if len(str) <= max {
		return str
	}
	return str[:max-3] + "..."
}

func formatTimeAgo(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		m := int(d.Minutes())
		if m == 1 {
			return "1 min ago"
		}
		return fmt.Sprintf("%d mins ago", m)
	case d < 24*time.Hour:
		h := int(d.Hours())
		if h == 1 {
			return "1 hour ago"
		}
		return fmt.Sprintf("%d hours ago", h)
	case d < 7*24*time.Hour:
		days := int(d.Hours() / 24)
		if days == 1 {
			return "1 day ago"
		}
		return fmt.Sprintf("%d days ago", days)
	default:
		return t.Format("Jan 2")
	}
}

func formatSize(size int64) string {
	if size < 1024 {
		return fmt.Sprintf("%dB", size)
	} else if size < 1024*1024 {
		return fmt.Sprintf("%.1fKB", float64(size)/1024)
	}
	return fmt.Sprintf("%.1fMB", float64(size)/(1024*1024))
}

func (m model) renderHelp() string {
	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(s.Purple).
		Padding(1, 2).
		Width(min(72, m.width-4))

	keys := []struct{ key, desc string }{
		{"j/k, ↑/↓", "Navigate"},
		{"enter", "Select / send message"},
		{"alt+enter", "Insert newline in chat draft"},
		{"↑/↓ in chat", "Move within a multiline draft"},
		{"pgup / pgdn", "Scroll chat history"},
		{"n", "New conversation"},
		{"e", "Edit / export"},
		{"d", "Delete"},
		{"ctrl+r", "Attach local files as context"},
		{"@file", "Reference a project file in chat"},
		{"alt+, / alt+.", "Switch model profile"},
		{"ctrl+o", "Export current chat to Markdown"},
		{"ctrl+y", "Copy one or more messages"},
		{"esc", "Back"},
		{"q", "Quit"},
		{"?", "Toggle this help"},
		{"ctrl+c", "In chat: clear draft, then close"},
	}

	var lines []string
	lines = append(lines, s.Title.Render("dwight — Help"))
	lines = append(lines, "")
	for _, k := range keys {
		lines = append(lines, fmt.Sprintf("  %s  %s",
			s.KeyStyle.Render(fmt.Sprintf("%-16s", k.key)), k.desc))
	}
	lines = append(lines, "")
	lines = append(lines, s.Dim.Render("Providers: use Ollama profiles for local models, Gemini profiles with GEMINI_API_KEY / GOOGLE_API_KEY for demos."))
	lines = append(lines, "")
	lines = append(lines, s.Dim.Render("Press ?, q, or esc to close"))

	return lipgloss.Place(m.width, m.height,
		lipgloss.Center, lipgloss.Center,
		box.Render(strings.Join(lines, "\n")))
}
