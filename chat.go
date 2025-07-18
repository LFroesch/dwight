// Replace the chat.go file with this enhanced version:

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const (
	ollamaURL = "http://localhost:11434"
	modelName = "qwen2.5-coder:7b"
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

// Docker management functions
func stopOllamaContainer() error {
	// Stop the Ollama container to free memory
	cmd := exec.Command("docker", "stop", "ollama")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to stop Ollama container: %v", err)
	}
	return nil
}

func ensureOllamaContainer() error {
	// Check if Ollama container is running
	cmd := exec.Command("docker", "ps", "--filter", "name=ollama", "--format", "{{.Names}}")
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to check Docker containers: %v", err)
	}

	if strings.Contains(string(output), "ollama") {
		return nil // Container is already running
	}

	// Check if container exists but is stopped
	cmd = exec.Command("docker", "ps", "-a", "--filter", "name=ollama", "--format", "{{.Names}}")
	output, err = cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to check Docker containers: %v", err)
	}

	if strings.Contains(string(output), "ollama") {
		// Container exists but is stopped, start it
		cmd = exec.Command("docker", "start", "ollama")
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to start Ollama container: %v", err)
		}
		// Wait a bit for container to start
		time.Sleep(3 * time.Second)
		return nil
	}

	// Container doesn't exist, create and run it
	cmd = exec.Command("docker", "run", "-d",
		"--name", "ollama",
		"-p", "11434:11434",
		"-v", "ollama:/root/.ollama",
		"ollama/ollama")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to run Ollama container: %v", err)
	}

	// Wait for container to start
	time.Sleep(5 * time.Second)
	return nil
}

func pullModelIfNeeded() error {
	// Check if model exists by trying to list it
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

	// Check if model already exists
	for _, model := range tags.Models {
		if strings.HasPrefix(model.Name, modelName) {
			return nil // Model already exists
		}
	}

	// Model doesn't exist, start pulling it
	pullReq := map[string]string{"name": modelName}
	jsonData, _ := json.Marshal(pullReq)

	// Start the pull process (don't wait for completion)
	pullClient := &http.Client{Timeout: 60 * time.Second}
	pullResp, err := pullClient.Post(ollamaURL+"/api/pull", "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to start model pull: %v", err)
	}
	defer pullResp.Body.Close()

	if pullResp.StatusCode != http.StatusOK {
		return fmt.Errorf("model pull request failed: %d", pullResp.StatusCode)
	}

	// Consume the response to avoid connection issues
	io.ReadAll(pullResp.Body)

	// Now wait for the model to be available by polling
	maxWaitTime := 10 * time.Minute
	checkInterval := 10 * time.Second
	startTime := time.Now()

	for time.Since(startTime) < maxWaitTime {
		// Check if model is now available
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

		// Check if model is now available
		for _, model := range tags2.Models {
			if strings.HasPrefix(model.Name, modelName) {
				resp2.Body.Close()
				return nil // Model is ready!
			}
		}

		resp2.Body.Close()

		// Wait before checking again
		time.Sleep(checkInterval)
	}

	return fmt.Errorf("model pull timed out after %v", maxWaitTime)
}

func checkOllamaModel() tea.Cmd {
	return func() tea.Msg {
		// First ensure Ollama container is running
		if err := ensureOllamaContainer(); err != nil {
			return CheckModelMsg{Available: false, Err: fmt.Errorf("Failed to start Ollama container: %v", err)}
		}

		// Then check if the model is available and pull if needed
		if err := pullModelIfNeeded(); err != nil {
			return CheckModelMsg{Available: false, Err: fmt.Errorf("Failed to ensure model availability: %v", err)}
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

// Helper function to wrap text to specified width
func wrapText(text string, width int) string {
	if width <= 0 {
		return text
	}
	
	words := strings.Fields(text)
	if len(words) == 0 {
		return text
	}
	
	var result strings.Builder
	var line strings.Builder
	
	for _, word := range words {
		// If adding this word would exceed width, start new line
		if line.Len() > 0 && line.Len()+len(word)+1 > width {
			result.WriteString(line.String())
			result.WriteString("\n")
			line.Reset()
		}
		
		// Add word to current line
		if line.Len() > 0 {
			line.WriteString(" ")
		}
		line.WriteString(word)
	}
	
	// Add remaining text
	if line.Len() > 0 {
		result.WriteString(line.String())
	}
	
	return result.String()
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
			// Wrap user message at ~80 characters
			wrappedContent := wrapText(msg.Content, 80)
			content.WriteString(messageContentStyle.Render(wrappedContent))
			content.WriteString("\n")
		} else {
			content.WriteString(assistantStyle.Render("🤖 Assistant:"))
			content.WriteString("\n")
			// Wrap assistant message at ~80 characters
			wrappedContent := wrapText(msg.Content, 80)
			content.WriteString(messageContentStyle.Render(wrappedContent))
			content.WriteString("\n")
		}
	}
	return content.String()
}
