package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const (
	ollamaURL = "http://localhost:11434"
)

// Chat message styles
var (
	userStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#60A5FA")).
			Bold(true).
			MarginBottom(1)

	assistantStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#34D399")).
			Bold(true).
			MarginBottom(1)

	messageContentStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#E5E7EB")).
				MarginLeft(2).
				MarginBottom(1)

	chatHeaderStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#7C3AED")).
			Background(lipgloss.Color("#1F2937")).
			Bold(true).
			Padding(0, 1).
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#374151"))

	inputBoxStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#374151")).
			Padding(0, 1)

	statusStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FBBF24")).
			Bold(true)

	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#EF4444")).
			Bold(true)

	codeBlockStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#A78BFA")).
			Background(lipgloss.Color("#1F2937")).
			Padding(0, 1)
)

// Docker management functions
func stopOllamaContainer() error {
	cmd := exec.Command("docker", "stop", "ollama")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to stop Ollama container: %v", err)
	}
	return nil
}

func ensureDockerImageExists() error {
	cmd := exec.Command("docker", "images", "ollama/ollama", "--format", "{{.Repository}}")
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to check Docker images: %v", err)
	}

	if !strings.Contains(string(output), "ollama/ollama") {
		cmd = exec.Command("docker", "pull", "ollama/ollama")
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to pull Ollama image: %v", err)
		}
	}
	return nil
}

func ensureOllamaContainer() error {
	if err := ensureDockerImageExists(); err != nil {
		return err
	}

	cmd := exec.Command("docker", "ps", "--filter", "name=ollama", "--format", "{{.Names}}")
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to check Docker containers: %v", err)
	}

	if strings.Contains(string(output), "ollama") {
		return nil
	}

	cmd = exec.Command("docker", "ps", "-a", "--filter", "name=ollama", "--format", "{{.Names}}")
	output, err = cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to check Docker containers: %v", err)
	}

	if strings.Contains(string(output), "ollama") {
		cmd = exec.Command("docker", "start", "ollama")
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to start Ollama container: %v", err)
		}
		time.Sleep(3 * time.Second)
		return nil
	}

	cmd = exec.Command("docker", "run", "-d",
		"--name", "ollama",
		"-p", "11434:11434",
		"-v", "ollama:/root/.ollama",
		"ollama/ollama")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to run Ollama container: %v", err)
	}

	time.Sleep(5 * time.Second)
	return nil
}

func pullModelIfNeeded(modelName string) error {
	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Get(ollamaURL + "/api/tags")
	if err != nil {
		return fmt.Errorf("failed to connect to Ollama: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Ollama API error: %d", resp.StatusCode)
	}

	var tags struct {
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&tags); err != nil {
		return fmt.Errorf("failed to decode response: %v", err)
	}

	for _, model := range tags.Models {
		if strings.HasPrefix(model.Name, modelName) {
			return nil
		}
	}

	pullReq := map[string]string{"name": modelName}
	jsonData, _ := json.Marshal(pullReq)

	pullClient := &http.Client{Timeout: 60 * time.Second}
	pullResp, err := pullClient.Post(ollamaURL+"/api/pull", "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to start model pull: %v", err)
	}
	defer pullResp.Body.Close()

	if pullResp.StatusCode != http.StatusOK {
		return fmt.Errorf("model pull request failed: %d", pullResp.StatusCode)
	}

	io.ReadAll(pullResp.Body)

	maxWaitTime := 10 * time.Minute
	checkInterval := 10 * time.Second
	startTime := time.Now()

	for time.Since(startTime) < maxWaitTime {
		resp2, err := client.Get(ollamaURL + "/api/tags")
		if err != nil {
			time.Sleep(checkInterval)
			continue
		}

		if resp2.StatusCode != http.StatusOK {
			resp2.Body.Close()
			time.Sleep(checkInterval)
			continue
		}

		var tags2 struct {
			Models []struct {
				Name string `json:"name"`
			} `json:"models"`
		}

		if err := json.NewDecoder(resp2.Body).Decode(&tags2); err != nil {
			resp2.Body.Close()
			time.Sleep(checkInterval)
			continue
		}

		for _, model := range tags2.Models {
			if strings.HasPrefix(model.Name, modelName) {
				resp2.Body.Close()
				return nil
			}
		}

		resp2.Body.Close()
		time.Sleep(checkInterval)
	}

	return fmt.Errorf("model pull timed out after %v", maxWaitTime)
}

// checkModelAvailable checks if a model exists without pulling it
func checkModelAvailable(modelName string) (bool, error) {
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(ollamaURL + "/api/tags")
	if err != nil {
		return false, fmt.Errorf("failed to connect to Ollama: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("Ollama API error: %d", resp.StatusCode)
	}

	var tags struct {
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&tags); err != nil {
		return false, fmt.Errorf("failed to decode response: %v", err)
	}

	// Check if model exists
	for _, model := range tags.Models {
		if strings.HasPrefix(model.Name, modelName) {
			return true, nil
		}
	}

	return false, nil
}

func (m *model) checkOllamaModel() tea.Cmd {
	return func() tea.Msg {
		modelName := m.getCurrentProfile().Model

		// First ensure Ollama container is running
		if err := ensureOllamaContainer(); err != nil {
			return CheckModelMsg{Available: false, ModelName: modelName, Err: fmt.Errorf("Failed to start Ollama container: %v", err)}
		}

		// Check if the model is available (without pulling)
		available, err := checkModelAvailable(modelName)
		if err != nil {
			return CheckModelMsg{Available: false, ModelName: modelName, Err: fmt.Errorf("Failed to check model availability: %v", err)}
		}

		return CheckModelMsg{Available: available, ModelName: modelName, Err: nil}
	}
}

func (m *model) pullModel() tea.Cmd {
	return func() tea.Msg {
		modelName := m.getCurrentProfile().Model
		if err := pullModelIfNeeded(modelName); err != nil {
			return CheckModelMsg{Available: false, ModelName: modelName, Err: fmt.Errorf("Failed to pull model: %v", err)}
		}
		return CheckModelMsg{Available: true, ModelName: modelName, Err: nil}
	}
}

// Streaming chat function - collects response with streaming API
func sendChatMessageStreaming(userMsg string, profile ModelProfile, appSettings AppSettings, chatHistory []ChatMessage, attachedResources []string) tea.Cmd {
	return func() tea.Msg {
		startTime := time.Now()
		client := &http.Client{Timeout: time.Duration(appSettings.ChatTimeout) * time.Second}

		var messages []map[string]string

		systemPrompt := profile.SystemPrompt
		if appSettings.MainPrompt != "" {
			systemPrompt = appSettings.MainPrompt + "\n\n" + systemPrompt
		}

		// Add attached resources to system prompt for RAG
		if len(attachedResources) > 0 {
			var resourceContent strings.Builder
			resourceContent.WriteString("\n\n=== ATTACHED RESOURCES ===\n\n")
			for _, path := range attachedResources {
				if data, err := os.ReadFile(path); err == nil {
					resourceContent.WriteString(fmt.Sprintf("--- File: %s ---\n", filepath.Base(path)))
					resourceContent.WriteString(string(data))
					resourceContent.WriteString("\n\n")
				}
			}
			resourceContent.WriteString("=== END RESOURCES ===\n\n")
			resourceContent.WriteString("Use the above resources to provide context when answering questions.")
			systemPrompt += resourceContent.String()
		}

		if systemPrompt != "" {
			messages = append(messages, map[string]string{
				"role":    "system",
				"content": systemPrompt,
			})
		}

		for _, msg := range chatHistory {
			if msg.Role == "user" || msg.Role == "assistant" {
				messages = append(messages, map[string]string{
					"role":    msg.Role,
					"content": msg.Content,
				})
			}
		}

		messages = append(messages, map[string]string{
			"role":    "user",
			"content": userMsg,
		})

		requestBody := map[string]interface{}{
			"model":       profile.Model,
			"messages":    messages,
			"temperature": profile.Temperature,
			"stream":      true,
		}

		jsonData, _ := json.Marshal(requestBody)

		resp, err := client.Post(ollamaURL+"/api/chat", "application/json", bytes.NewBuffer(jsonData))
		if err != nil {
			return ResponseMsg{Err: fmt.Errorf("Request failed: %v", err)}
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return ResponseMsg{Err: fmt.Errorf("API error: %d", resp.StatusCode)}
		}

		// Read the streaming response
		scanner := bufio.NewScanner(resp.Body)
		var fullContent strings.Builder
		var promptTokens, totalTokens int

		for scanner.Scan() {
			line := scanner.Bytes()
			if len(line) == 0 {
				continue
			}

			var streamResp struct {
				Message struct {
					Content string `json:"content"`
				} `json:"message"`
				Done            bool `json:"done"`
				PromptEvalCount int  `json:"prompt_eval_count"`
				EvalCount       int  `json:"eval_count"`
			}

			if err := json.Unmarshal(line, &streamResp); err != nil {
				continue
			}

			if streamResp.Message.Content != "" {
				fullContent.WriteString(streamResp.Message.Content)
			}

			if streamResp.Done {
				promptTokens = streamResp.PromptEvalCount
				totalTokens = streamResp.PromptEvalCount + streamResp.EvalCount
				break
			}
		}

		if err := scanner.Err(); err != nil {
			return ResponseMsg{Err: fmt.Errorf("Stream error: %v", err)}
		}

		duration := time.Since(startTime)
		return ResponseMsg{
			Content:      fullContent.String(),
			Duration:     duration,
			PromptTokens: promptTokens,
			TotalTokens:  totalTokens,
			Err:          nil,
		}
	}
}

// Non-streaming fallback
func sendChatMessage(userMsg string, profile ModelProfile, appSettings AppSettings, chatHistory []ChatMessage, attachedResources []string) tea.Cmd {
	return func() tea.Msg {
		startTime := time.Now()
		timeout := time.Duration(appSettings.ChatTimeout) * time.Second
		client := &http.Client{Timeout: timeout}

		var messages []map[string]string

		systemPrompt := profile.SystemPrompt
		if appSettings.MainPrompt != "" {
			systemPrompt = appSettings.MainPrompt + "\n\n" + systemPrompt
		}

		// Add attached resources to system prompt for RAG
		if len(attachedResources) > 0 {
			var resourceContent strings.Builder
			resourceContent.WriteString("\n\n=== ATTACHED RESOURCES ===\n\n")
			for _, path := range attachedResources {
				if data, err := os.ReadFile(path); err == nil {
					resourceContent.WriteString(fmt.Sprintf("--- File: %s ---\n", filepath.Base(path)))
					resourceContent.WriteString(string(data))
					resourceContent.WriteString("\n\n")
				}
			}
			resourceContent.WriteString("=== END RESOURCES ===\n\n")
			resourceContent.WriteString("Use the above resources to provide context when answering questions.")
			systemPrompt += resourceContent.String()
		}

		if systemPrompt != "" {
			messages = append(messages, map[string]string{
				"role":    "system",
				"content": systemPrompt,
			})
		}

		for _, msg := range chatHistory {
			if msg.Role == "user" || msg.Role == "assistant" {
				messages = append(messages, map[string]string{
					"role":    msg.Role,
					"content": msg.Content,
				})
			}
		}

		messages = append(messages, map[string]string{
			"role":    "user",
			"content": userMsg,
		})

		requestBody := map[string]interface{}{
			"model":       profile.Model,
			"messages":    messages,
			"temperature": profile.Temperature,
			"stream":      false,
		}

		jsonData, _ := json.Marshal(requestBody)

		resp, err := client.Post(ollamaURL+"/api/chat", "application/json", bytes.NewBuffer(jsonData))
		if err != nil {
			return ResponseMsg{Err: fmt.Errorf("Request failed: %v", err)}
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return ResponseMsg{Err: fmt.Errorf("API error: %d", resp.StatusCode)}
		}

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return ResponseMsg{Err: fmt.Errorf("Failed to read response: %v", err)}
		}

		var response struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
			PromptEvalCount int `json:"prompt_eval_count"`
			EvalCount       int `json:"eval_count"`
		}

		if err := json.Unmarshal(body, &response); err != nil {
			return ResponseMsg{Err: fmt.Errorf("Failed to parse response: %v", err)}
		}
		duration := time.Since(startTime)
		return ResponseMsg{
			Content:      response.Message.Content,
			Duration:     duration,
			PromptTokens: response.PromptEvalCount,
			TotalTokens:  response.PromptEvalCount + response.EvalCount,
			Err:          nil,
		}
	}
}

// Helper function to wrap text to specified width
func wrapText(text string, width int) []string {
	if width < 10 {
		width = 10
	}

	var lines []string
	paragraphs := strings.Split(text, "\n")

	for _, paragraph := range paragraphs {
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

func (m *model) saveChatLog() error {
	if len(m.chatMessages) == 0 {
		return nil
	}

	chatsDir := filepath.Join(m.currentDir, "chats")
	if err := os.MkdirAll(chatsDir, 0755); err != nil {
		return err
	}

	now := time.Now()
	filename := fmt.Sprintf("%02d_%02d_%02d_%d_%02d_%s_%s.txt",
		now.Month(), now.Day(), now.Year()%100,
		now.Hour()%12, now.Minute(),
		map[bool]string{true: "PM", false: "AM"}[now.Hour() >= 12],
		strings.ReplaceAll(m.getCurrentProfile().Model, ":", "_"))

	filepath := filepath.Join(chatsDir, filename)
	profileName, profileModel := m.getCurrentProfile().Name, m.getCurrentProfile().Model
	var content strings.Builder
	content.WriteString(fmt.Sprintf("Chat Log - %s\n", now.Format("January 2, 2006 3:04 PM")))
	content.WriteString(fmt.Sprintf("Model: %s\n", profileModel))
	content.WriteString(fmt.Sprintf("Profile: %s\n", profileName))
	content.WriteString(strings.Repeat("=", 50) + "\n\n")

	for _, msg := range m.chatMessages {
		if msg.Role == "user" {
			content.WriteString("USER:\n")
			content.WriteString(msg.Content)
			content.WriteString("\n\n")
		} else {
			content.WriteString("ASSISTANT")
			if msg.Duration > 0 {
				responseTokens := msg.TotalTokens - msg.PromptTokens
				content.WriteString(fmt.Sprintf(" (%.1fs | prompt: %d, response: %d)", msg.Duration.Seconds(), msg.PromptTokens, responseTokens))
			}
			content.WriteString(":\n")
			content.WriteString(msg.Content)
			content.WriteString("\n\n")
		}
		content.WriteString(strings.Repeat("-", 30) + "\n\n")
	}

	return os.WriteFile(filepath, []byte(content.String()), 0644)
}

func cleanupOldChats(dir string, daysOld int) (int, error) {
	chatsDir := filepath.Join(dir, "chats")
	if _, err := os.Stat(chatsDir); os.IsNotExist(err) {
		return 0, nil
	}

	cutoffTime := time.Now().AddDate(0, 0, -daysOld)
	deletedCount := 0

	files, err := os.ReadDir(chatsDir)
	if err != nil {
		return 0, err
	}

	for _, file := range files {
		if file.IsDir() {
			continue
		}

		info, err := file.Info()
		if err != nil {
			continue
		}

		if info.ModTime().Before(cutoffTime) {
			fullPath := filepath.Join(chatsDir, file.Name())
			if err := os.Remove(fullPath); err == nil {
				deletedCount++
			}
		}
	}

	return deletedCount, nil
}

// Simple markdown-like formatting for code blocks
func formatMessageContent(content string) string {
	lines := strings.Split(content, "\n")
	var formatted []string
	inCodeBlock := false

	for _, line := range lines {
		if strings.HasPrefix(line, "```") {
			inCodeBlock = !inCodeBlock
			continue
		}

		if inCodeBlock {
			formatted = append(formatted, codeBlockStyle.Render("  "+line))
		} else if strings.HasPrefix(line, "# ") {
			formatted = append(formatted, lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#A78BFA")).Render(line))
		} else if strings.HasPrefix(line, "## ") {
			formatted = append(formatted, lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#8B5CF6")).Render(line))
		} else if strings.HasPrefix(line, "- ") || strings.HasPrefix(line, "* ") {
			formatted = append(formatted, lipgloss.NewStyle().Foreground(lipgloss.Color("#60A5FA")).Render("  • "+line[2:]))
		} else if strings.Contains(line, "`") && !inCodeBlock {
			// Inline code
			parts := strings.Split(line, "`")
			var styledParts []string
			for i, part := range parts {
				if i%2 == 1 {
					styledParts = append(styledParts, lipgloss.NewStyle().Foreground(lipgloss.Color("#A78BFA")).Render(part))
				} else {
					styledParts = append(styledParts, part)
				}
			}
			formatted = append(formatted, strings.Join(styledParts, ""))
		} else {
			formatted = append(formatted, line)
		}
	}

	return strings.Join(formatted, "\n")
}

// Load chat history files from the chats directory
func (m *model) loadChatHistoryFiles() error {
	chatsDir := filepath.Join(m.currentDir, "chats")
	if _, err := os.Stat(chatsDir); os.IsNotExist(err) {
		m.chatHistoryFiles = []ChatHistoryFile{}
		return nil
	}

	files, err := os.ReadDir(chatsDir)
	if err != nil {
		return err
	}

	m.chatHistoryFiles = []ChatHistoryFile{}
	for _, file := range files {
		if file.IsDir() || !strings.HasSuffix(file.Name(), ".txt") {
			continue
		}

		info, err := file.Info()
		if err != nil {
			continue
		}

		// Extract model name from filename (format: MM_DD_YY_H_MM_AM/PM_ModelName.txt)
		parts := strings.Split(file.Name(), "_")
		modelName := "unknown"
		if len(parts) > 6 {
			modelName = strings.TrimSuffix(strings.Join(parts[6:], "_"), ".txt")
			modelName = strings.ReplaceAll(modelName, "_", ":")
		}

		m.chatHistoryFiles = append(m.chatHistoryFiles, ChatHistoryFile{
			Filename:  file.Name(),
			Path:      filepath.Join(chatsDir, file.Name()),
			Timestamp: info.ModTime(),
			Model:     modelName,
			Size:      info.Size(),
		})
	}

	// Sort by timestamp, most recent first
	sort.Slice(m.chatHistoryFiles, func(i, j int) bool {
		return m.chatHistoryFiles[i].Timestamp.After(m.chatHistoryFiles[j].Timestamp)
	})

	return nil
}

// Load a chat from a history file
func (m *model) loadChatFromHistory(filepath string) error {
	content, err := os.ReadFile(filepath)
	if err != nil {
		return err
	}

	lines := strings.Split(string(content), "\n")
	m.chatMessages = []ChatMessage{}

	var currentMessage *ChatMessage
	var contentBuilder strings.Builder

	for _, line := range lines {
		// Check for user message start
		if line == "USER:" {
			// Save previous message if exists
			if currentMessage != nil {
				currentMessage.Content = strings.TrimSpace(contentBuilder.String())
				m.chatMessages = append(m.chatMessages, *currentMessage)
			}
			currentMessage = &ChatMessage{Role: "user"}
			contentBuilder.Reset()
			continue
		}

		// Check for assistant message start
		if strings.HasPrefix(line, "ASSISTANT") {
			// Save previous message if exists
			if currentMessage != nil {
				currentMessage.Content = strings.TrimSpace(contentBuilder.String())
				m.chatMessages = append(m.chatMessages, *currentMessage)
			}
			currentMessage = &ChatMessage{Role: "assistant"}
			contentBuilder.Reset()
			continue
		}

		// Skip separator lines and header lines
		if strings.HasPrefix(line, "===") || strings.HasPrefix(line, "---") ||
			strings.HasPrefix(line, "Chat Log -") ||
			strings.HasPrefix(line, "Model:") ||
			strings.HasPrefix(line, "Profile:") ||
			line == "" {
			continue
		}

		// Add content to current message
		if currentMessage != nil {
			if contentBuilder.Len() > 0 {
				contentBuilder.WriteString("\n")
			}
			contentBuilder.WriteString(line)
		}
	}

	// Save last message
	if currentMessage != nil {
		currentMessage.Content = strings.TrimSpace(contentBuilder.String())
		m.chatMessages = append(m.chatMessages, *currentMessage)
	}

	m.updateChatLines()
	return nil
}
