package gemini

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

const defaultBaseURL = "https://generativelanguage.googleapis.com/v1beta"

type ChatMessage struct {
	Role    string
	Content string
}

type ChatRequest struct {
	Model       string
	Messages    []ChatMessage
	System      string
	Temperature float64
	Timeout     time.Duration
}

type StreamChunk struct {
	Content      string
	Done         bool
	Duration     time.Duration
	PromptTokens int
	TotalTokens  int
	Err          error
}

func APIKey() string {
	if key := strings.TrimSpace(os.Getenv("GEMINI_API_KEY")); key != "" {
		return key
	}
	return strings.TrimSpace(os.Getenv("GOOGLE_API_KEY"))
}

func CheckModel(modelName string) error {
	if strings.TrimSpace(modelName) == "" {
		return fmt.Errorf("Gemini profile is missing a model name")
	}
	if APIKey() == "" {
		return fmt.Errorf("Gemini API key missing. Set GEMINI_API_KEY or GOOGLE_API_KEY, then retry")
	}
	return nil
}

func ChatStream(ctx context.Context, req ChatRequest) (<-chan StreamChunk, error) {
	if err := CheckModel(req.Model); err != nil {
		return nil, err
	}

	timeout := req.Timeout
	if timeout == 0 {
		timeout = 180 * time.Second
	}
	client := &http.Client{Timeout: timeout}

	payload := buildPayload(req)
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to encode request: %v", err)
	}

	url := fmt.Sprintf("%s/models/%s:streamGenerateContent?alt=sse", defaultBaseURL, req.Model)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(body))
	if err != nil {
		return nil, fmt.Errorf("request failed: %v", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-goog-api-key", APIKey())

	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request failed: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Gemini API error: %d %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}

	ch := make(chan StreamChunk)
	go func() {
		defer resp.Body.Close()
		defer close(ch)

		startTime := time.Now()
		var lastPromptTokens int
		var lastTotalTokens int
		scanner := bufio.NewScanner(resp.Body)
		const maxScanToken = 1024 * 1024
		buf := make([]byte, 0, 64*1024)
		scanner.Buffer(buf, maxScanToken)

		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" || strings.HasPrefix(line, ":") {
				continue
			}
			if !strings.HasPrefix(line, "data:") {
				continue
			}

			data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
			if data == "" {
				continue
			}

			var respChunk generateContentResponse
			if err := json.Unmarshal([]byte(data), &respChunk); err != nil {
				ch <- StreamChunk{Err: fmt.Errorf("failed to parse Gemini stream: %v", err)}
				return
			}

			lastPromptTokens = respChunk.UsageMetadata.PromptTokenCount
			lastTotalTokens = respChunk.UsageMetadata.TotalTokenCount
			text := respChunk.text()
			if text == "" {
				continue
			}
			ch <- StreamChunk{
				Content:      text,
				Duration:     time.Since(startTime),
				PromptTokens: lastPromptTokens,
				TotalTokens:  lastTotalTokens,
			}
		}

		if err := scanner.Err(); err != nil {
			if ctx.Err() != nil {
				return
			}
			ch <- StreamChunk{Err: fmt.Errorf("Gemini stream failed: %v", err)}
			return
		}

		ch <- StreamChunk{
			Done:         true,
			Duration:     time.Since(startTime),
			PromptTokens: lastPromptTokens,
			TotalTokens:  lastTotalTokens,
		}
	}()

	return ch, nil
}

func buildPayload(req ChatRequest) map[string]interface{} {
	contents := make([]map[string]interface{}, 0, len(req.Messages))
	for _, msg := range req.Messages {
		role := "user"
		if strings.EqualFold(msg.Role, "assistant") || strings.EqualFold(msg.Role, "model") {
			role = "model"
		}
		contents = append(contents, map[string]interface{}{
			"role": role,
			"parts": []map[string]string{
				{"text": msg.Content},
			},
		})
	}

	payload := map[string]interface{}{
		"contents": contents,
		"generationConfig": map[string]interface{}{
			"temperature": req.Temperature,
		},
	}

	if strings.TrimSpace(req.System) != "" {
		payload["systemInstruction"] = map[string]interface{}{
			"parts": []map[string]string{
				{"text": req.System},
			},
		}
	}

	return payload
}

type generateContentResponse struct {
	Candidates []struct {
		Content struct {
			Parts []struct {
				Text string `json:"text"`
			} `json:"parts"`
		} `json:"content"`
	} `json:"candidates"`
	UsageMetadata struct {
		PromptTokenCount int `json:"promptTokenCount"`
		TotalTokenCount  int `json:"totalTokenCount"`
	} `json:"usageMetadata"`
}

func (r generateContentResponse) text() string {
	var b strings.Builder
	for _, candidate := range r.Candidates {
		for _, part := range candidate.Content.Parts {
			b.WriteString(part.Text)
		}
	}
	return b.String()
}
