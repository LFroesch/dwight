package main

import (
	"encoding/json"
	"os"
	"path/filepath"
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

func (m *model) loadModelConfig() {
	configPath := filepath.Join(m.currentDir, ".dwight-models.json")

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
	configPath := filepath.Join(m.currentDir, ".dwight-models.json")
	data, _ := json.MarshalIndent(m.modelConfig, "", "  ")
	os.WriteFile(configPath, data, 0644)
}

func (m *model) getCurrentProfile() ModelProfile {
	if m.modelConfig.CurrentProfile >= 0 && m.modelConfig.CurrentProfile < len(m.modelConfig.Profiles) {
		return m.modelConfig.Profiles[m.modelConfig.CurrentProfile]
	}
	return defaultProfiles[0]
}
