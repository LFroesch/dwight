package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"dwight/internal/ollama"
	s "dwight/internal/styles"
	"dwight/internal/storage"

	"github.com/charmbracelet/glamour"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// =============================================================================
// Chat rendering
// =============================================================================

func (m *model) updateChatLines() {
	contentWidth := m.width - 4
	if contentWidth < 20 {
		contentWidth = 20
	}

	// Remember whether user was at the bottom before rebuilding lines.
	oldMax := len(m.chatLines) - m.chatMaxLines
	if oldMax < 0 {
		oldMax = 0
	}
	wasAtBottom := len(m.chatLines) == 0 || m.chatScrollPos >= oldMax

	m.chatLines = nil

	if len(m.chatMessages) == 0 && !m.chatStreaming {
		profile := m.currentProfile()
		m.chatLines = append(m.chatLines,
			"",
			s.Dim.Render("  What can I help you with?"),
			"",
			s.Dim.Render("  Type a message, or use @filename to reference files."),
			s.Dim.Render(fmt.Sprintf("  Model: %s", profile.Model)),
			"",
		)
		_ = wasAtBottom
		m.chatScrollPos = 0
		return
	}

	for i := range m.chatMessages {
		msg := &m.chatMessages[i]
		if msg.lastWidth != contentWidth || len(msg.formattedLines) == 0 {
			msg.formattedLines = formatChatMessage(msg, contentWidth)
			msg.lastWidth = contentWidth
		}
		if m.chatCopyMode && i == m.chatCopyIdx && len(msg.formattedLines) > 0 {
			// Highlight selected message header in copy mode
			highlighted := make([]string, len(msg.formattedLines))
			copy(highlighted, msg.formattedLines)
			highlighted[0] = s.Selected.Render("► ") + highlighted[0]
			m.chatLines = append(m.chatLines, highlighted...)
		} else {
			m.chatLines = append(m.chatLines, msg.formattedLines...)
		}
	}

	// Streaming content — use fallback renderer (glamour is expensive mid-stream)
	if m.chatStreaming && m.chatStreamBuffer != "" {
		header := s.AssistantMsg.Render("Assistant:")
		m.chatLines = append(m.chatLines, header)
		formatted := formatMarkdownFallback(m.chatStreamBuffer)
		for _, line := range wrapText(formatted, contentWidth) {
			m.chatLines = append(m.chatLines, "  "+line)
		}
		m.chatLines = append(m.chatLines, "")
	}

	// Auto-scroll to bottom only if user was already there (preserves manual scroll during streaming).
	newMax := len(m.chatLines) - m.chatMaxLines
	if newMax < 0 {
		newMax = 0
	}
	if wasAtBottom {
		m.chatScrollPos = newMax
	} else if m.chatScrollPos > newMax {
		m.chatScrollPos = newMax
	}
}

func formatChatMessage(msg *ChatMessage, width int) []string {
	var lines []string

	if msg.Role == "user" {
		timeStr := ""
		if !msg.Timestamp.IsZero() {
			timeStr = msg.Timestamp.Format("3:04PM") + " "
		}
		lines = append(lines, s.UserMsg.Render(timeStr+"You:"))
		lines = append(lines, wrapText(msg.Content, width)...)
		lines = append(lines, "")
	} else {
		timeStr := ""
		if !msg.Timestamp.IsZero() {
			timeStr = msg.Timestamp.Format("3:04PM") + " "
		}
		header := timeStr + "AI:"
		if msg.Duration > 0 && msg.TotalTokens > 0 {
			respTokens := msg.TotalTokens - msg.PromptTokens
			tokPerSec := 0.0
			if msg.Duration.Seconds() > 0 {
				tokPerSec = float64(respTokens) / msg.Duration.Seconds()
			}
			header = fmt.Sprintf("%sAI: %.1fs · %.0f tok/s · %d tok",
				timeStr, msg.Duration.Seconds(), tokPerSec, respTokens)
		}
		lines = append(lines, s.AssistantMsg.Render(header))
		rendered := renderMarkdown(msg.Content, width)
		for _, line := range strings.Split(rendered, "\n") {
			lines = append(lines, line)
		}
		lines = append(lines, "")
	}
	return lines
}

// renderMarkdown renders markdown content using glamour with a dark theme.
// Falls back to plain text if glamour fails.
func renderMarkdown(content string, width int) string {
	if width < 20 {
		width = 20
	}
	r, err := glamour.NewTermRenderer(
		glamour.WithStylePath("dark"),
		glamour.WithWordWrap(width),
	)
	if err != nil {
		return content
	}
	out, err := r.Render(content)
	if err != nil {
		return content
	}
	// Glamour adds leading/trailing newlines — trim trailing only to avoid double-blank
	return strings.TrimRight(out, "\n")
}

// Kept for use in streaming preview (plain text, no glamour needed)
func formatMarkdownFallback(content string) string {
	rawLines := strings.Split(content, "\n")
	var formatted []string
	inCodeBlock := false

	for _, line := range rawLines {
		if strings.HasPrefix(line, "```") {
			inCodeBlock = !inCodeBlock
			continue
		}
		if inCodeBlock {
			formatted = append(formatted, s.CodeBlock.Render("  "+line))
		} else if strings.HasPrefix(line, "# ") || strings.HasPrefix(line, "## ") {
			formatted = append(formatted, lipgloss.NewStyle().Bold(true).Foreground(s.Violet).Render(line))
		} else if strings.HasPrefix(line, "- ") || strings.HasPrefix(line, "* ") {
			formatted = append(formatted, lipgloss.NewStyle().Foreground(s.Blue).Render("  •  "+line[2:]))
		} else {
			formatted = append(formatted, line)
		}
	}
	return strings.Join(formatted, "\n")
}

func (m *model) getVisibleChatLines() []string {
	if len(m.chatLines) == 0 {
		return []string{"Start a conversation..."}
	}
	start := m.chatScrollPos
	end := start + m.chatMaxLines
	if start < 0 {
		start = 0
	}
	if end > len(m.chatLines) {
		end = len(m.chatLines)
	}
	if start >= len(m.chatLines) {
		return []string{"(no content)"}
	}
	return m.chatLines[start:end]
}

func (m *model) handleChatScroll(key string) {
	maxScroll := len(m.chatLines) - m.chatMaxLines
	if maxScroll < 0 {
		maxScroll = 0
	}
	switch key {
	case "up":
		if m.chatScrollPos > 0 {
			m.chatScrollPos--
		}
	case "down":
		if m.chatScrollPos < maxScroll {
			m.chatScrollPos++
		}
	case "pgup":
		m.chatScrollPos -= m.chatMaxLines / 2
		if m.chatScrollPos < 0 {
			m.chatScrollPos = 0
		}
	case "pgdown":
		m.chatScrollPos += m.chatMaxLines / 2
		if m.chatScrollPos > maxScroll {
			m.chatScrollPos = maxScroll
		}
	case "home":
		m.chatScrollPos = 0
	case "end":
		m.chatScrollPos = maxScroll
	}
}

// =============================================================================
// Text wrapping
// =============================================================================

func wrapText(text string, width int) []string {
	if width < 10 {
		width = 10
	}
	var lines []string
	for _, paragraph := range strings.Split(text, "\n") {
		if strings.TrimSpace(paragraph) == "" {
			lines = append(lines, "")
			continue
		}
		words := strings.Fields(paragraph)
		if len(words) == 0 {
			lines = append(lines, "")
			continue
		}
		currentLine := ""
		for _, word := range words {
			if len(word) > width {
				if currentLine != "" {
					lines = append(lines, currentLine)
					currentLine = ""
				}
				for len(word) > width {
					lines = append(lines, word[:width])
					word = word[width:]
				}
				if len(word) > 0 {
					currentLine = word
				}
				continue
			}
			testLine := currentLine
			if testLine != "" {
				testLine += " "
			}
			testLine += word
			if len(testLine) > width {
				if currentLine != "" {
					lines = append(lines, currentLine)
				}
				currentLine = word
			} else {
				currentLine = testLine
			}
		}
		if currentLine != "" {
			lines = append(lines, currentLine)
		}
	}
	return lines
}

// =============================================================================
// Conversation save/load helpers
// =============================================================================

func (m *model) saveCurrentChat() error {
	if len(m.chatMessages) == 0 {
		return fmt.Errorf("no messages")
	}

	var conv *storage.Conversation
	if m.currentConversation != nil {
		conv = m.currentConversation
		conv.Messages = chatToConvMessages(m.chatMessages)
	} else {
		profile := m.currentProfile()
		convMsgs := chatToConvMessages(m.chatMessages)
		title := storage.GenerateTitle(convMsgs)
		conv = &storage.Conversation{
			ID:          storage.NewConversationSlug(title),
			Title:       title,
			Model:       profile.Model,
			ProfileName: profile.Name,
			Created:     time.Now(),
			Messages:    convMsgs,
			Tags:        []string{},
			WorkingDir:  m.workContext.WorkingDir,
			GitRoot:     m.workContext.GitRoot,
			OriginHint:  m.workContext.OriginHint,
		}
		m.currentConversation = conv
	}
	return storage.SaveConversation(conv)
}

func (m *model) exportConversation(format string) tea.Cmd {
	return func() tea.Msg {
		if m.selectedConv >= len(m.conversations) {
			return statusMsg{message: "No conversation selected"}
		}
		conv := m.conversations[m.selectedConv]
		loaded, err := storage.LoadConversation(conv.ID)
		if err != nil {
			return statusMsg{message: fmt.Sprintf("Failed to load: %v", err)}
		}

		exportDir := filepath.Join(m.currentDir, "exports")
		os.MkdirAll(exportDir, 0755)

		var content, ext string
		switch format {
		case "markdown":
			content = storage.ExportMarkdown(loaded)
			ext = ".md"
		case "json":
			content, _ = storage.ExportJSON(loaded)
			ext = ".json"
		}

		safeTitle := strings.ReplaceAll(conv.Title, "/", "-")
		safeTitle = strings.ReplaceAll(safeTitle, " ", "_")
		if len(safeTitle) > 50 {
			safeTitle = safeTitle[:50]
		}
		filename := filepath.Join(exportDir, safeTitle+"_"+conv.Created.Format("20060102")+ext)
		if err := os.WriteFile(filename, []byte(content), 0644); err != nil {
			return statusMsg{message: fmt.Sprintf("Failed to write: %v", err)}
		}
		return statusMsg{message: fmt.Sprintf("Exported to %s", filename)}
	}
}

// Convert between display ChatMessage and storage ConvMessage
func chatToConvMessages(msgs []ChatMessage) []storage.ConvMessage {
	out := make([]storage.ConvMessage, len(msgs))
	for i, m := range msgs {
		out[i] = storage.ConvMessage{
			Role: m.Role, Content: m.Content, Timestamp: m.Timestamp,
			Duration: m.Duration, PromptTokens: m.PromptTokens, TotalTokens: m.TotalTokens,
		}
	}
	return out
}

func convMessagesToChat(msgs []storage.ConvMessage) []ChatMessage {
	out := make([]ChatMessage, len(msgs))
	for i, m := range msgs {
		out[i] = ChatMessage{
			Role: m.Role, Content: m.Content, Timestamp: m.Timestamp,
			Duration: m.Duration, PromptTokens: m.PromptTokens, TotalTokens: m.TotalTokens,
		}
	}
	return out
}

// =============================================================================
// File scanning
// =============================================================================

func (m *model) scanAttachableFiles() []string {
	var files []string
	entries, err := os.ReadDir(m.currentDir)
	if err != nil {
		return files
	}
	validExts := map[string]bool{".md": true, ".txt": true, ".json": true, ".yaml": true, ".yml": true}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(e.Name()))
		if validExts[ext] {
			files = append(files, filepath.Join(m.currentDir, e.Name()))
		}
	}
	return files
}

// copyToClipboard writes text to the system clipboard.
// Tries wl-copy (Wayland/WSLg), clip.exe (WSL2), then xclip.
func copyToClipboard(text string) error {
	cmds := [][]string{
		{"wl-copy"},
		{"clip.exe"},
		{"xclip", "-selection", "clipboard"},
		{"xsel", "--clipboard", "--input"},
	}
	for _, args := range cmds {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Stdin = strings.NewReader(text)
		if err := cmd.Run(); err == nil {
			return nil
		}
	}
	return fmt.Errorf("no clipboard tool found (wl-copy, clip.exe, xclip, xsel)")
}

func readFileContent(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// =============================================================================
// Library filter
// =============================================================================

// =============================================================================
// @ autocomplete — project file scanning + fuzzy matching
// =============================================================================

// scanProjectFiles recursively scans the project directory for code files.
// Results are cached until the directory changes.
func (m *model) scanProjectFiles() []string {
	if m.fileCacheDir == m.currentDir && len(m.fileCache) > 0 {
		return m.fileCache
	}

	var files []string

	filepath.WalkDir(m.currentDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}

		rel, _ := filepath.Rel(m.currentDir, path)
		if rel == "." {
			return nil
		}

		// Skip hidden dirs and common noise
		if d.IsDir() {
			name := d.Name()
			if strings.HasPrefix(name, ".") || name == "node_modules" || name == "vendor" ||
				name == "__pycache__" || name == "conversations" || name == "exports" {
				return filepath.SkipDir
			}
			// Depth limit
			if strings.Count(rel, string(os.PathSeparator)) >= 4 {
				return filepath.SkipDir
			}
			return nil
		}

		// Filter by extension
		ext := strings.ToLower(filepath.Ext(path))
		codeExts := map[string]bool{
			".go": true, ".py": true, ".js": true, ".ts": true, ".tsx": true, ".jsx": true,
			".rs": true, ".rb": true, ".java": true, ".c": true, ".h": true, ".cpp": true,
			".md": true, ".txt": true, ".json": true, ".yaml": true, ".yml": true, ".toml": true,
			".sql": true, ".sh": true, ".bash": true, ".zsh": true, ".css": true, ".html": true,
			".xml": true, ".csv": true, ".log": true, ".cfg": true, ".conf": true, ".env": true,
			".mod": true, ".sum": true, ".lock": true, ".dockerfile": true,
		}
		name := strings.ToLower(d.Name())
		if codeExts[ext] || name == "makefile" || name == "dockerfile" || name == ".gitignore" {
			files = append(files, rel)
		}

		return nil
	})

	m.fileCache = files
	m.fileCacheDir = m.currentDir
	return files
}


// fuzzyMatch filters files by a query using character-sequence matching (fzf-style).
// All query chars must appear in order in the path. Scoring favors basename matches,
// consecutive runs, and shorter paths.
func fuzzyMatch(files []string, query string) []string {
	if query == "" {
		return files
	}
	lQuery := strings.ToLower(query)

	type scored struct {
		path  string
		score int
	}
	var matches []scored

	for _, f := range files {
		lPath := strings.ToLower(f)
		lBase := strings.ToLower(filepath.Base(f))

		ok, score := fuzzyScore(lPath, lBase, lQuery)
		if !ok {
			continue
		}
		matches = append(matches, scored{f, score})
	}

	sort.Slice(matches, func(i, j int) bool {
		return matches[i].score > matches[j].score
	})
	result := make([]string, len(matches))
	for i, m := range matches {
		result[i] = m.path
	}
	return result
}

// fuzzyScore checks if all query chars appear in path (in order) and returns a score.
// Higher score = better match.
func fuzzyScore(lPath, lBase, lQuery string) (bool, int) {
	// All chars must match in sequence
	pi := 0
	for _, qc := range lQuery {
		found := false
		for pi < len(lPath) {
			if rune(lPath[pi]) == qc {
				pi++
				found = true
				break
			}
			pi++
		}
		if !found {
			return false, 0
		}
	}

	score := 0
	// Strong bonus: query is a substring of basename (consecutive)
	if strings.Contains(lBase, lQuery) {
		score += 30
		if strings.HasPrefix(lBase, lQuery) {
			score += 20
		}
	}
	// Bonus: query is substring anywhere in path
	if strings.Contains(lPath, lQuery) {
		score += 10
	}
	// Penalty: depth (prefer shallower files)
	score -= strings.Count(lPath, string(os.PathSeparator)) * 3
	// Penalty: long paths
	score -= len(lPath) / 15
	return true, score
}

// resolveAtReferences expands @path references in a message to file contents.
func resolveAtReferences(msg string, baseDir string) string {
	// Find all @path patterns (not preceded by alphanumeric)
	words := strings.Fields(msg)
	var expanded strings.Builder
	for i, word := range words {
		if i > 0 {
			expanded.WriteString(" ")
		}
		if strings.HasPrefix(word, "@") && len(word) > 1 {
			path := word[1:]
			fullPath := filepath.Join(baseDir, path)
			if _, err := os.Stat(fullPath); err == nil {
				// Keep the @reference visible, append content after all references are processed
				expanded.WriteString(word)
				continue
			}
		}
		expanded.WriteString(word)
	}

	// Now append file contents for all @references
	result := expanded.String()
	var attachments strings.Builder
	for _, word := range words {
		if strings.HasPrefix(word, "@") && len(word) > 1 {
			path := word[1:]
			fullPath := filepath.Join(baseDir, path)
			if data, err := os.ReadFile(fullPath); err == nil {
				ext := filepath.Ext(path)
				lang := strings.TrimPrefix(ext, ".")
				fmt.Fprintf(&attachments, "\n\n--- %s ---\n```%s\n%s\n```", path, lang, strings.TrimRight(string(data), "\n"))
			}
		}
	}
	if attachments.Len() > 0 {
		result += attachments.String()
	}
	return result
}

// =============================================================================
// Code block extraction from AI responses
// =============================================================================

// extractCodeBlocks parses fenced code blocks from a response.
// Supports ```lang:path and ```lang path formats.
func extractCodeBlocks(content string) []CodeBlock {
	var blocks []CodeBlock
	lines := strings.Split(content, "\n")
	inBlock := false
	var current CodeBlock
	var blockLines []string

	for _, line := range lines {
		if strings.HasPrefix(line, "```") && !inBlock {
			inBlock = true
			meta := strings.TrimPrefix(line, "```")
			meta = strings.TrimSpace(meta)

			// Parse lang:path or lang path
			if idx := strings.Index(meta, ":"); idx > 0 && !strings.Contains(meta[:idx], " ") {
				current.Language = meta[:idx]
				current.Path = strings.TrimSpace(meta[idx+1:])
			} else if parts := strings.Fields(meta); len(parts) >= 2 {
				current.Language = parts[0]
				// Second part is path if it looks like one
				if strings.Contains(parts[1], ".") || strings.Contains(parts[1], "/") {
					current.Path = parts[1]
				}
			} else if meta != "" {
				current.Language = meta
			}
			blockLines = nil
			continue
		}
		if strings.HasPrefix(line, "```") && inBlock {
			inBlock = false
			current.Content = strings.Join(blockLines, "\n")
			blocks = append(blocks, current)
			current = CodeBlock{}
			continue
		}
		if inBlock {
			blockLines = append(blockLines, line)
		}
	}
	return blocks
}

// generateDiff creates a simple unified diff between old and new content.
func generateDiff(oldContent, newContent, path string) []string {
	oldLines := strings.Split(oldContent, "\n")
	newLines := strings.Split(newContent, "\n")

	var diff []string
	diff = append(diff, fmt.Sprintf("--- %s", path))
	diff = append(diff, fmt.Sprintf("+++ %s", path))

	// Simple line-by-line diff (not a real unified diff, but functional)
	maxLen := len(oldLines)
	if len(newLines) > maxLen {
		maxLen = len(newLines)
	}

	for i := 0; i < maxLen; i++ {
		var oldLine, newLine string
		if i < len(oldLines) {
			oldLine = oldLines[i]
		}
		if i < len(newLines) {
			newLine = newLines[i]
		}

		if i >= len(oldLines) {
			diff = append(diff, "+ "+newLine)
		} else if i >= len(newLines) {
			diff = append(diff, "- "+oldLine)
		} else if oldLine != newLine {
			diff = append(diff, "- "+oldLine)
			diff = append(diff, "+ "+newLine)
		} else {
			diff = append(diff, "  "+oldLine)
		}
	}
	return diff
}

// hasActionableBlocks returns true if any code block has a detected file path.
func hasActionableBlocks(blocks []CodeBlock) bool {
	for _, b := range blocks {
		if b.Path != "" {
			return true
		}
	}
	return false
}

// hasMoreActionableBlocks checks if there are actionable blocks at or after index.
func hasMoreActionableBlocks(blocks []CodeBlock, fromIndex int) bool {
	for i := fromIndex; i < len(blocks); i++ {
		if blocks[i].Path != "" {
			return true
		}
	}
	return false
}

// acceptCodeBlock writes the current review block to disk.
func (m model) acceptCodeBlock() (tea.Model, tea.Cmd) {
	if m.reviewIndex >= len(m.codeBlocks) {
		m.chatState = ChatStateReady
		m.chatTextArea.Focus()
		return m, nil
	}

	// Find next block with a path
	for m.reviewIndex < len(m.codeBlocks) && m.codeBlocks[m.reviewIndex].Path == "" {
		m.reviewIndex++
	}
	if m.reviewIndex >= len(m.codeBlocks) {
		m.chatState = ChatStateReady
		m.chatTextArea.Focus()
		return m, nil
	}

	block := m.codeBlocks[m.reviewIndex]
	fullPath := filepath.Join(m.currentDir, block.Path)

	// Ensure parent directory exists
	os.MkdirAll(filepath.Dir(fullPath), 0755)

	if err := os.WriteFile(fullPath, []byte(block.Content+"\n"), 0644); err != nil {
		m.reviewIndex++
		if !hasMoreActionableBlocks(m.codeBlocks, m.reviewIndex) {
			m.chatState = ChatStateReady
			m.codeBlocks = nil
			m.chatTextArea.Focus()
		}
		return m, showStatus(fmt.Sprintf("Failed to write %s: %v", block.Path, err))
	}

	msg := fmt.Sprintf("Wrote %s", block.Path)
	m.reviewIndex++
	if !hasMoreActionableBlocks(m.codeBlocks, m.reviewIndex) {
		m.chatState = ChatStateReady
		m.codeBlocks = nil
		m.reviewIndex = 0
		m.chatTextArea.Focus()
	}
	// Invalidate file cache since we wrote a file
	m.fileCache = nil
	return m, showStatus(msg)
}

func (m *model) getFilteredLibrary() []ollama.LibraryModel {
	if m.libraryFilter == "" {
		return m.libraryModels
	}
	filter := strings.ToLower(m.libraryFilter)
	var filtered []ollama.LibraryModel
	for _, mdl := range m.libraryModels {
		if strings.Contains(strings.ToLower(mdl.Name), filter) ||
			strings.Contains(strings.ToLower(mdl.Description), filter) {
			filtered = append(filtered, mdl)
		}
	}
	return filtered
}
