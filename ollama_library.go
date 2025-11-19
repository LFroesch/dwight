package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// OllamaModel represents a model from Ollama API
type OllamaModel struct {
	Name       string `json:"name"`
	ModifiedAt string `json:"modified_at"`
	Size       int64  `json:"size"`
}

// OllamaLibraryModel represents a model from Ollama's library
type OllamaLibraryModel struct {
	Name        string
	Description string
	Tags        []string
	Size        string
	Installed   bool
}

// getInstalledModels fetches locally installed models
func getInstalledModels() ([]OllamaModel, error) {
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(ollamaURL + "/api/tags")
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Ollama: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Ollama API error: %d", resp.StatusCode)
	}

	var result struct {
		Models []OllamaModel `json:"models"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %v", err)
	}

	return result.Models, nil
}

// getPopularModels returns a curated list of popular models
func getPopularModels() []OllamaLibraryModel {
	return []OllamaLibraryModel{
		{
			Name:        "llama3.2:3b",
			Description: "Meta's Llama 3.2 - Fast, capable model (3B params)",
			Tags:        []string{"general", "chat", "code"},
			Size:        "2.0GB",
		},
		{
			Name:        "llama3.2:1b",
			Description: "Meta's Llama 3.2 - Ultra-fast lightweight (1B params)",
			Tags:        []string{"general", "chat"},
			Size:        "1.3GB",
		},
		{
			Name:        "qwen2.5-coder:7b",
			Description: "Alibaba's Qwen 2.5 Coder - Excellent for coding (7B params)",
			Tags:        []string{"code", "programming"},
			Size:        "4.7GB",
		},
		{
			Name:        "qwen2.5-coder:14b",
			Description: "Alibaba's Qwen 2.5 Coder - Advanced coding (14B params)",
			Tags:        []string{"code", "programming"},
			Size:        "9.0GB",
		},
		{
			Name:        "phi3:3.8b",
			Description: "Microsoft Phi-3 - Small but powerful (3.8B params)",
			Tags:        []string{"general", "chat"},
			Size:        "2.3GB",
		},
		{
			Name:        "gemma2:2b",
			Description: "Google Gemma 2 - Efficient and fast (2B params)",
			Tags:        []string{"general", "chat"},
			Size:        "1.6GB",
		},
		{
			Name:        "mistral:7b",
			Description: "Mistral AI - Balanced performance (7B params)",
			Tags:        []string{"general", "chat", "code"},
			Size:        "4.1GB",
		},
		{
			Name:        "llama3.1:8b",
			Description: "Meta's Llama 3.1 - Strong general model (8B params)",
			Tags:        []string{"general", "chat", "reasoning"},
			Size:        "4.7GB",
		},
		{
			Name:        "codellama:7b",
			Description: "Meta's Code Llama - Specialized for coding (7B params)",
			Tags:        []string{"code", "programming"},
			Size:        "3.8GB",
		},
		{
			Name:        "deepseek-coder:6.7b",
			Description: "DeepSeek Coder - Advanced code generation (6.7B params)",
			Tags:        []string{"code", "programming"},
			Size:        "3.8GB",
		},
		{
			Name:        "llava:7b",
			Description: "LLaVA - Vision + language model (7B params)",
			Tags:        []string{"vision", "multimodal"},
			Size:        "4.5GB",
		},
		{
			Name:        "neural-chat:7b",
			Description: "Intel's Neural Chat - Optimized for conversation (7B params)",
			Tags:        []string{"chat", "conversation"},
			Size:        "4.1GB",
		},
	}
}

// checkModelInstalled checks if a model is already installed
func checkModelInstalled(modelName string, installed []OllamaModel) bool {
	for _, model := range installed {
		if model.Name == modelName {
			return true
		}
	}
	return false
}
