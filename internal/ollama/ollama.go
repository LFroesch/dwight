package ollama

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// GetURL reads OLLAMA_HOST env var, falls back to localhost:11434.
func GetURL() string {
	host := os.Getenv("OLLAMA_HOST")
	if host == "" {
		return "http://localhost:11434"
	}
	if !strings.HasPrefix(host, "http://") && !strings.HasPrefix(host, "https://") {
		host = "http://" + host
	}
	return host
}

// Model represents a locally installed Ollama model.
type Model struct {
	Name       string `json:"name"`
	ModifiedAt string `json:"modified_at"`
	Size       int64  `json:"size"`
}

// ChatMessage is a single message in a conversation.
type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ChatRequest configures a chat API call.
type ChatRequest struct {
	Model       string
	Messages    []ChatMessage
	Temperature float64
	Stream      bool
	Timeout     time.Duration
}

// ChatResponse holds the result of a non-streaming chat call.
type ChatResponse struct {
	Content      string
	Duration     time.Duration
	PromptTokens int
	TotalTokens  int
}

// StreamChunk holds one chunk from a streaming response.
type StreamChunk struct {
	Content      string
	Done         bool
	Duration     time.Duration
	PromptTokens int
	TotalTokens  int
	Err          error
}

// CheckModel returns true if the named model is locally available.
func CheckModel(modelName string) (bool, error) {
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(GetURL() + "/api/tags")
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

	for _, m := range tags.Models {
		if strings.HasPrefix(m.Name, modelName) {
			return true, nil
		}
	}
	return false, nil
}

// ListModels returns all locally installed models.
func ListModels() ([]Model, error) {
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(GetURL() + "/api/tags")
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Ollama: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Ollama API error: %d", resp.StatusCode)
	}

	var result struct {
		Models []Model `json:"models"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %v", err)
	}
	return result.Models, nil
}

// PullModel downloads a model. Blocks until complete.
func PullModel(modelName string) error {
	client := &http.Client{Timeout: 10 * time.Minute}

	pullReq := map[string]string{"name": modelName}
	jsonData, _ := json.Marshal(pullReq)

	resp, err := client.Post(GetURL()+"/api/pull", "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("pull failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("pull failed with status: %d", resp.StatusCode)
	}

	// Drain the streaming response to completion
	decoder := json.NewDecoder(resp.Body)
	for {
		var pullResp struct {
			Status string `json:"status"`
		}
		if err := decoder.Decode(&pullResp); err != nil {
			if err == io.EOF {
				break
			}
			return err
		}
	}
	return nil
}

// Chat sends a non-streaming chat request and returns the full response.
func Chat(req ChatRequest) (*ChatResponse, error) {
	startTime := time.Now()
	timeout := req.Timeout
	if timeout == 0 {
		timeout = 180 * time.Second
	}
	client := &http.Client{Timeout: timeout}

	messages := make([]map[string]string, len(req.Messages))
	for i, m := range req.Messages {
		messages[i] = map[string]string{"role": m.Role, "content": m.Content}
	}

	body := map[string]interface{}{
		"model":       req.Model,
		"messages":    messages,
		"temperature": req.Temperature,
		"stream":      false,
	}
	jsonData, _ := json.Marshal(body)

	resp, err := client.Post(GetURL()+"/api/chat", "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API error: %d", resp.StatusCode)
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %v", err)
	}

	var result struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
		PromptEvalCount int `json:"prompt_eval_count"`
		EvalCount       int `json:"eval_count"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %v", err)
	}

	return &ChatResponse{
		Content:      result.Message.Content,
		Duration:     time.Since(startTime),
		PromptTokens: result.PromptEvalCount,
		TotalTokens:  result.PromptEvalCount + result.EvalCount,
	}, nil
}

// ChatStream sends a streaming chat request and returns chunks via channel.
func ChatStream(req ChatRequest) (<-chan StreamChunk, error) {
	timeout := req.Timeout
	if timeout == 0 {
		timeout = 180 * time.Second
	}
	client := &http.Client{Timeout: timeout}

	messages := make([]map[string]string, len(req.Messages))
	for i, m := range req.Messages {
		messages[i] = map[string]string{"role": m.Role, "content": m.Content}
	}

	body := map[string]interface{}{
		"model":       req.Model,
		"messages":    messages,
		"temperature": req.Temperature,
		"stream":      true,
	}
	jsonData, _ := json.Marshal(body)

	resp, err := client.Post(GetURL()+"/api/chat", "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("request failed: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("API error: %d", resp.StatusCode)
	}

	ch := make(chan StreamChunk)
	go func() {
		defer resp.Body.Close()
		defer close(ch)

		startTime := time.Now()
		scanner := bufio.NewScanner(resp.Body)
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

			chunk := StreamChunk{
				Content: streamResp.Message.Content,
				Done:    streamResp.Done,
			}
			if streamResp.Done {
				chunk.Duration = time.Since(startTime)
				chunk.PromptTokens = streamResp.PromptEvalCount
				chunk.TotalTokens = streamResp.PromptEvalCount + streamResp.EvalCount
			}
			ch <- chunk
		}

		if err := scanner.Err(); err != nil {
			ch <- StreamChunk{Err: fmt.Errorf("stream error: %v", err)}
		}
	}()

	return ch, nil
}

// PopularModels returns a curated list of popular models for the library browser.
type LibraryModel struct {
	Name        string
	Description string
	Tags        []string
	Size        string
	Installed   bool
}

func PopularModels() []LibraryModel {
	return []LibraryModel{
		{Name: "llama3.2:3b", Description: "Meta's Llama 3.2 - Fast, capable (3B)", Tags: []string{"general", "chat", "code"}, Size: "2.0GB"},
		{Name: "llama3.2:1b", Description: "Meta's Llama 3.2 - Ultra-fast (1B)", Tags: []string{"general", "chat"}, Size: "1.3GB"},
		{Name: "qwen2.5-coder:7b", Description: "Qwen 2.5 Coder - Excellent for coding (7B)", Tags: []string{"code"}, Size: "4.7GB"},
		{Name: "qwen2.5-coder:14b", Description: "Qwen 2.5 Coder - Advanced coding (14B)", Tags: []string{"code"}, Size: "9.0GB"},
		{Name: "phi3:3.8b", Description: "Microsoft Phi-3 - Small but powerful (3.8B)", Tags: []string{"general", "chat"}, Size: "2.3GB"},
		{Name: "gemma2:2b", Description: "Google Gemma 2 - Efficient (2B)", Tags: []string{"general", "chat"}, Size: "1.6GB"},
		{Name: "mistral:7b", Description: "Mistral - Balanced performance (7B)", Tags: []string{"general", "chat", "code"}, Size: "4.1GB"},
		{Name: "llama3.1:8b", Description: "Meta's Llama 3.1 - Strong general (8B)", Tags: []string{"general", "reasoning"}, Size: "4.7GB"},
		{Name: "codellama:7b", Description: "Code Llama - Specialized coding (7B)", Tags: []string{"code"}, Size: "3.8GB"},
		{Name: "deepseek-coder:6.7b", Description: "DeepSeek Coder - Code generation (6.7B)", Tags: []string{"code"}, Size: "3.8GB"},
	}
}

// IsModelInstalled checks if a model name matches any installed model.
func IsModelInstalled(name string, installed []Model) bool {
	for _, m := range installed {
		if m.Name == name {
			return true
		}
	}
	return false
}

// ContextWindowSize returns estimated context window for a model.
func ContextWindowSize(modelName string) int {
	switch {
	case strings.Contains(modelName, "llama3.2"), strings.Contains(modelName, "llama3.1"):
		return 128000
	case strings.Contains(modelName, "qwen2.5"), strings.Contains(modelName, "mistral"):
		return 32768
	default:
		return 8192
	}
}
