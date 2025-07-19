// Replace the chat.go file with this enhanced version:

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
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

func ensureDockerImageExists() error {
	// Check if Ollama image exists
	cmd := exec.Command("docker", "images", "ollama/ollama", "--format", "{{.Repository}}")
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to check Docker images: %v", err)
	}

	if !strings.Contains(string(output), "ollama/ollama") {
		// Pull the Ollama image
		cmd = exec.Command("docker", "pull", "ollama/ollama")
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to pull Ollama image: %v", err)
		}
	}
	return nil
}

func ensureOllamaContainer() error {
	// First ensure the Docker image exists
	if err := ensureDockerImageExists(); err != nil {
		return err
	}

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

func pullModelIfNeeded(modelName string) error {
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

func (m *model) checkOllamaModel() tea.Cmd {
	return func() tea.Msg {
		// First ensure Ollama container is running
		if err := ensureOllamaContainer(); err != nil {
			return CheckModelMsg{Available: false, Err: fmt.Errorf("Failed to start Ollama container: %v", err)}
		}

		// Then check if the model is available and pull if needed
		if err := pullModelIfNeeded(m.getCurrentProfile().Model); err != nil {
			return CheckModelMsg{Available: false, Err: fmt.Errorf("Failed to ensure model availability: %v", err)}
		}

		return CheckModelMsg{Available: true, Err: nil}
	}
}

func sendChatMessage(userMsg string, profile ModelProfile) tea.Cmd {
	return func() tea.Msg {
		startTime := time.Now()
		client := &http.Client{Timeout: 180 * time.Second}

		requestBody := map[string]interface{}{
			"model":       profile.Model,
			"prompt":      userMsg,
			"system":      profile.SystemPrompt,
			"temperature": profile.Temperature,
			"stream":      false,
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
			Response        string `json:"response"`
			PromptEvalCount int    `json:"prompt_eval_count"`
			EvalCount       int    `json:"eval_count"`
		}

		if err := json.Unmarshal(body, &response); err != nil {
			return ResponseMsg{Err: fmt.Errorf("Failed to parse response: %v", err)}
		}
		duration := time.Since(startTime)
		return ResponseMsg{
			Content:      response.Response,
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
			// If word is too long, break it
			if len(word) > width {
				if currentLine != "" {
					lines = append(lines, currentLine)
					currentLine = ""
				}
				// Break the long word into chunks
				for len(word) > width {
					lines = append(lines, word[:width])
					word = word[width:]
				}
				if len(word) > 0 {
					currentLine = word
				}
				continue
			}

			// Check if adding this word would exceed width
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

	// Create chats directory in current directory
	chatsDir := filepath.Join(m.currentDir, "chats")
	if err := os.MkdirAll(chatsDir, 0755); err != nil {
		return err
	}

	// Format filename: 10_25_25_3_30_PM_model_name.txt
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
				content.WriteString(fmt.Sprintf(" (%.1fs, %d tokens)", msg.Duration.Seconds(), msg.TotalTokens))
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
		return 0, nil // No chats directory
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
