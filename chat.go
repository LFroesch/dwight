// Replace the chat.go file with this enhanced version:

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const (
	ollamaURL = "http://localhost:11434"
	modelName = "qwen2.5:0.5b"
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
)

func checkOllamaModel() tea.Cmd {
	return func() tea.Msg {
		client := &http.Client{Timeout: 10 * time.Second}

		resp, err := client.Get(ollamaURL + "/api/tags")
		if err != nil {
			return CheckModelMsg{Available: false, Err: fmt.Errorf("Ollama not running. Start with: docker run -d -p 11434:11434 ollama/ollama")}
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return CheckModelMsg{Available: false, Err: fmt.Errorf("Ollama API error: %d. Try: docker restart ollama", resp.StatusCode)}
		}

		var tags struct {
			Models []struct {
				Name string `json:"name"`
			} `json:"models"`
		}

		if err := json.NewDecoder(resp.Body).Decode(&tags); err != nil {
			return CheckModelMsg{Available: false, Err: fmt.Errorf("Failed to decode response: %v", err)}
		}

		for _, model := range tags.Models {
			if strings.HasPrefix(model.Name, modelName) {
				return CheckModelMsg{Available: true, Err: nil}
			}
		}

		// Try to pull the model
		pullReq := map[string]string{"name": modelName}
		jsonData, _ := json.Marshal(pullReq)

		pullResp, err := client.Post(ollamaURL+"/api/pull", "application/json", bytes.NewBuffer(jsonData))
		if err != nil {
			return CheckModelMsg{Available: false, Err: fmt.Errorf("Failed to pull model: %v", err)}
		}
		defer pullResp.Body.Close()

		if pullResp.StatusCode != http.StatusOK {
			return CheckModelMsg{Available: false, Err: fmt.Errorf("Model pull failed: %d", pullResp.StatusCode)}
		}

		return CheckModelMsg{Available: true, Err: nil}
	}
}

func sendChatMessage(userMsg string) tea.Cmd {
	return func() tea.Msg {
		client := &http.Client{Timeout: 30 * time.Second}

		requestBody := map[string]interface{}{
			"model":  modelName,
			"prompt": userMsg,
			"stream": false,
		}

		jsonData, _ := json.Marshal(requestBody)

		resp, err := client.Post(ollamaURL+"/api/generate", "application/json", bytes.NewBuffer(jsonData))
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
			Response string `json:"response"`
		}

		if err := json.Unmarshal(body, &response); err != nil {
			return ResponseMsg{Err: fmt.Errorf("Failed to parse response: %v", err)}
		}

		return ResponseMsg{Content: response.Response, Err: nil}
	}
}

// Helper function to render chat messages for viewport
func renderChatHistory(messages []ChatMessage) string {
	if len(messages) == 0 {
		emptyStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#6B7280")).
			Italic(true)
		return emptyStyle.Render("💬 Start a conversation with the AI assistant...\n\n")
	}

	var content strings.Builder
	for _, msg := range messages {
		if msg.Role == "user" {
			content.WriteString(userStyle.Render("👤 You:"))
			content.WriteString("\n")
			content.WriteString(messageContentStyle.Render(msg.Content))
			content.WriteString("\n")
		} else {
			content.WriteString(assistantStyle.Render("🤖 Assistant:"))
			content.WriteString("\n")
			content.WriteString(messageContentStyle.Render(msg.Content))
			content.WriteString("\n")
		}
	}
	return content.String()
}
