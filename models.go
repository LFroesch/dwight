package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

var defaultProfiles = []ModelProfile{
	{
		Name:         "Coder Assistant",
		Model:        "qwen2.5-coder:7b",
		SystemPrompt: "You are a helpful coding assistant. Provide clear, concise code examples.",
		Temperature:  0.7,
	},
	{
		Name:         "General Assistant",
		Model:        "llama3.2:3b",
		SystemPrompt: "You are a helpful AI assistant.",
		Temperature:  0.8,
	},
	{
		Name:         "Creative Writer",
		Model:        "llama3.2:3b",
		SystemPrompt: "You are a creative writing assistant. Be imaginative and descriptive.",
		Temperature:  0.9,
	},
}

func pullOllamaModel(modelName string) tea.Cmd {
	return func() tea.Msg {
		// Ensure container is running
		if err := ensureOllamaContainer(); err != nil {
			return ModelPullMsg{Err: err}
		}

		client := &http.Client{Timeout: 10 * time.Minute} // Long timeout for downloads

		pullReq := map[string]string{"name": modelName}
		jsonData, _ := json.Marshal(pullReq)

		resp, err := client.Post(ollamaURL+"/api/pull", "application/json", bytes.NewBuffer(jsonData))
		if err != nil {
			return ModelPullMsg{Err: err}
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return ModelPullMsg{Err: fmt.Errorf("pull failed with status: %d", resp.StatusCode)}
		}

		// Read the streaming response
		decoder := json.NewDecoder(resp.Body)
		var lastStatus string

		for {
			var pullResp struct {
				Status    string `json:"status"`
				Total     int64  `json:"total"`
				Completed int64  `json:"completed"`
			}

			if err := decoder.Decode(&pullResp); err != nil {
				if err == io.EOF {
					break
				}
				return ModelPullMsg{Err: err}
			}

			lastStatus = pullResp.Status
		}

		return ModelPullMsg{Success: true, Status: lastStatus}
	}
}

// Add message type
type ModelPullMsg struct {
	Success bool
	Status  string
	Err     error
}

func (m *model) loadModelConfig() {
	configPath := filepath.Join(filepath.Dir(m.configFile), ".dwight-models.json")

	data, err := os.ReadFile(configPath)
	if err != nil {
		// Create default config
		m.modelConfig = ModelConfig{
			Profiles:       defaultProfiles,
			CurrentProfile: 0,
		}
		m.saveModelConfig()
		return
	}

	json.Unmarshal(data, &m.modelConfig)
	if len(m.modelConfig.Profiles) == 0 {
		m.modelConfig.Profiles = defaultProfiles
	}
}

func (m *model) saveModelConfig() {
	configPath := filepath.Join(filepath.Dir(m.configFile), ".dwight-models.json")
	data, _ := json.MarshalIndent(m.modelConfig, "", "  ")
	os.WriteFile(configPath, data, 0644)
}

func (m *model) getCurrentProfile() ModelProfile {
	if m.modelConfig.CurrentProfile >= 0 && m.modelConfig.CurrentProfile < len(m.modelConfig.Profiles) {
		return m.modelConfig.Profiles[m.modelConfig.CurrentProfile]
	}
	return defaultProfiles[0]
}

// Fetch available models from Ollama API
func fetchOllamaModels() ([]OllamaModel, error) {
	// Ensure container is running
	if err := ensureOllamaContainer(); err != nil {
		return nil, err
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(ollamaURL + "/api/tags")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch models: status %d", resp.StatusCode)
	}

	var result struct {
		Models []OllamaModel `json:"models"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return result.Models, nil
}

// Get popular Ollama models (curated list)
func getPopularOllamaModels() []string {
	return []string{
		"llama3.2:1b",
		"llama3.2:3b",
		"llama3.1:8b",
		"qwen2.5-coder:7b",
		"qwen2.5:7b",
		"mistral:7b",
		"phi3:3.8b",
		"gemma2:2b",
		"codellama:7b",
		"deepseek-coder:6.7b",
		"neural-chat:7b",
		"starling-lm:7b",
	}
}

type FetchModelsMsg struct {
	Models []OllamaModel
	Err    error
}

func fetchModelsCmd() tea.Cmd {
	return func() tea.Msg {
		models, err := fetchOllamaModels()
		return FetchModelsMsg{Models: models, Err: err}
	}
}
