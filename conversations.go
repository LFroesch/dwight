package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Conversation represents a saved chat session
type Conversation struct {
	ID              string        `json:"id"`
	Title           string        `json:"title"`
	Model           string        `json:"model"`
	ProfileName     string        `json:"profile_name"`
	Created         time.Time     `json:"created"`
	LastModified    time.Time     `json:"last_modified"`
	Messages        []ChatMessage `json:"messages"`
	TotalTokens     int           `json:"total_tokens"`
	PromptTokens    int           `json:"prompt_tokens"`
	MessageCount    int           `json:"message_count"`
	AttachedResources []string    `json:"attached_resources"` // Paths to resources used as context
	Tags            []string      `json:"tags"`
}

// ConversationMetadata is a lightweight version for listing
type ConversationMetadata struct {
	ID           string    `json:"id"`
	Title        string    `json:"title"`
	Model        string    `json:"model"`
	ProfileName  string    `json:"profile_name"`
	Created      time.Time `json:"created"`
	LastModified time.Time `json:"last_modified"`
	MessageCount int       `json:"message_count"`
	TotalTokens  int       `json:"total_tokens"`
	Tags         []string  `json:"tags"`
}

// getConversationsDir returns the directory where conversations are stored
func (m *model) getConversationsDir() string {
	return filepath.Join(m.currentDir, "conversations")
}

// ensureConversationsDir creates the conversations directory if it doesn't exist
func (m *model) ensureConversationsDir() error {
	dir := m.getConversationsDir()
	return os.MkdirAll(dir, 0755)
}

// generateConversationID creates a unique ID based on timestamp
func generateConversationID() string {
	return fmt.Sprintf("conv_%d", time.Now().UnixNano())
}

// generateConversationTitle creates a title from the first user message
func generateConversationTitle(messages []ChatMessage) string {
	for _, msg := range messages {
		if msg.Role == "user" {
			// Take first 50 chars of first user message
			title := msg.Content
			if len(title) > 50 {
				title = title[:50] + "..."
			}
			// Remove newlines
			title = strings.ReplaceAll(title, "\n", " ")
			return title
		}
	}
	return "Untitled Conversation"
}

// saveConversation saves a conversation to disk
func (m *model) saveConversation(conv *Conversation) error {
	if err := m.ensureConversationsDir(); err != nil {
		return err
	}

	// Update metadata
	conv.LastModified = time.Now()
	conv.MessageCount = len(conv.Messages)

	// Calculate total tokens
	totalTokens := 0
	promptTokens := 0
	for _, msg := range conv.Messages {
		totalTokens += msg.TotalTokens
		promptTokens += msg.PromptTokens
	}
	conv.TotalTokens = totalTokens
	conv.PromptTokens = promptTokens

	// Save full conversation
	filename := filepath.Join(m.getConversationsDir(), conv.ID+".json")
	data, err := json.MarshalIndent(conv, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(filename, data, 0644)
}

// loadConversation loads a conversation by ID
func (m *model) loadConversation(id string) (*Conversation, error) {
	filename := filepath.Join(m.getConversationsDir(), id+".json")
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	var conv Conversation
	if err := json.Unmarshal(data, &conv); err != nil {
		return nil, err
	}

	return &conv, nil
}

// deleteConversation removes a conversation from disk
func (m *model) deleteConversation(id string) error {
	filename := filepath.Join(m.getConversationsDir(), id+".json")
	return os.Remove(filename)
}

// listConversations returns all conversation metadata, sorted by last modified
func (m *model) listConversations() ([]ConversationMetadata, error) {
	dir := m.getConversationsDir()

	// Check if directory exists
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return []ConversationMetadata{}, nil
	}

	files, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var conversations []ConversationMetadata
	for _, file := range files {
		if file.IsDir() || !strings.HasSuffix(file.Name(), ".json") {
			continue
		}

		// Load full conversation (we could optimize this by storing metadata separately)
		id := strings.TrimSuffix(file.Name(), ".json")
		conv, err := m.loadConversation(id)
		if err != nil {
			continue // Skip corrupted files
		}

		conversations = append(conversations, ConversationMetadata{
			ID:           conv.ID,
			Title:        conv.Title,
			Model:        conv.Model,
			ProfileName:  conv.ProfileName,
			Created:      conv.Created,
			LastModified: conv.LastModified,
			MessageCount: conv.MessageCount,
			TotalTokens:  conv.TotalTokens,
			Tags:         conv.Tags,
		})
	}

	// Sort by last modified (newest first)
	sort.Slice(conversations, func(i, j int) bool {
		return conversations[i].LastModified.After(conversations[j].LastModified)
	})

	return conversations, nil
}

// saveCurrentChat saves the current chat session as a conversation
func (m *model) saveCurrentChat() error {
	if len(m.chatMessages) == 0 {
		return fmt.Errorf("no messages to save")
	}

	// Check if this is an existing conversation or a new one
	var conv *Conversation
	if m.currentConversation != nil {
		// Update existing conversation
		conv = m.currentConversation
		conv.Messages = m.chatMessages
	} else {
		// Create new conversation
		profile := m.getCurrentProfile()
		conv = &Conversation{
			ID:          generateConversationID(),
			Title:       generateConversationTitle(m.chatMessages),
			Model:       profile.Model,
			ProfileName: profile.Name,
			Created:     time.Now(),
			Messages:    m.chatMessages,
			AttachedResources: m.attachedResources,
			Tags:        []string{},
		}
		m.currentConversation = conv
	}

	return m.saveConversation(conv)
}

// loadConversationIntoChat loads a conversation into the current chat
func (m *model) loadConversationIntoChat(id string) error {
	conv, err := m.loadConversation(id)
	if err != nil {
		return err
	}

	m.currentConversation = conv
	m.chatMessages = conv.Messages
	m.attachedResources = conv.AttachedResources

	// Update chat display
	m.updateChatLines()

	return nil
}

// exportConversationMarkdown exports a conversation to markdown format
func (m *model) exportConversationMarkdown(id string) (string, error) {
	conv, err := m.loadConversation(id)
	if err != nil {
		return "", err
	}

	var md strings.Builder
	md.WriteString(fmt.Sprintf("# %s\n\n", conv.Title))
	md.WriteString(fmt.Sprintf("**Model:** %s (%s)  \n", conv.Model, conv.ProfileName))
	md.WriteString(fmt.Sprintf("**Created:** %s  \n", conv.Created.Format("January 2, 2006 3:04 PM")))
	md.WriteString(fmt.Sprintf("**Messages:** %d  \n", conv.MessageCount))
	md.WriteString(fmt.Sprintf("**Tokens:** %d  \n\n", conv.TotalTokens))

	if len(conv.AttachedResources) > 0 {
		md.WriteString("**Attached Resources:**\n")
		for _, res := range conv.AttachedResources {
			md.WriteString(fmt.Sprintf("- %s\n", res))
		}
		md.WriteString("\n")
	}

	md.WriteString("---\n\n")

	for _, msg := range conv.Messages {
		if msg.Role == "user" {
			md.WriteString("## 👤 User\n\n")
		} else {
			md.WriteString("## 🤖 Assistant\n\n")
			if msg.Duration > 0 {
				md.WriteString(fmt.Sprintf("*Response time: %.1fs | Tokens: %d*\n\n",
					msg.Duration.Seconds(), msg.TotalTokens))
			}
		}
		md.WriteString(msg.Content)
		md.WriteString("\n\n---\n\n")
	}

	return md.String(), nil
}

// exportConversationJSON exports a conversation to JSON format
func (m *model) exportConversationJSON(id string) (string, error) {
	conv, err := m.loadConversation(id)
	if err != nil {
		return "", err
	}

	data, err := json.MarshalIndent(conv, "", "  ")
	if err != nil {
		return "", err
	}

	return string(data), nil
}

// searchConversations searches conversations by title or content
func (m *model) searchConversations(query string) ([]ConversationMetadata, error) {
	allConvs, err := m.listConversations()
	if err != nil {
		return nil, err
	}

	if query == "" {
		return allConvs, nil
	}

	query = strings.ToLower(query)
	var results []ConversationMetadata

	for _, meta := range allConvs {
		// Search in title
		if strings.Contains(strings.ToLower(meta.Title), query) {
			results = append(results, meta)
			continue
		}

		// Search in model name
		if strings.Contains(strings.ToLower(meta.Model), query) {
			results = append(results, meta)
			continue
		}

		// Search in tags
		for _, tag := range meta.Tags {
			if strings.Contains(strings.ToLower(tag), query) {
				results = append(results, meta)
				break
			}
		}
	}

	return results, nil
}

// getContextWindowSize returns the estimated context window size for a model
func getContextWindowSize(modelName string) int {
	// Common model context windows
	// These are estimates - adjust based on actual models
	switch {
	case strings.Contains(modelName, "llama3.2"):
		return 128000 // 128k context
	case strings.Contains(modelName, "qwen2.5"):
		return 32768 // 32k context
	case strings.Contains(modelName, "llama3.1"):
		return 128000 // 128k context
	case strings.Contains(modelName, "mistral"):
		return 32768 // 32k context
	default:
		return 8192 // Default 8k context for unknown models
	}
}

// estimateTokenCount provides a rough estimate of token count
// This is a simple heuristic: ~4 characters per token for English text
func estimateTokenCount(text string) int {
	return len(text) / 4
}

// trimConversationToContext trims old messages to fit within context window
func (m *model) trimConversationToContext() {
	if len(m.chatMessages) == 0 {
		return
	}

	profile := m.getCurrentProfile()
	maxTokens := getContextWindowSize(profile.Model)

	// Reserve 20% for the response
	maxContextTokens := int(float64(maxTokens) * 0.8)

	// Calculate current token usage
	totalTokens := 0
	for _, msg := range m.chatMessages {
		if msg.TotalTokens > 0 {
			totalTokens += msg.TotalTokens
		} else {
			// Estimate if no token count
			totalTokens += estimateTokenCount(msg.Content)
		}
	}

	// If we're under the limit, no trimming needed
	if totalTokens <= maxContextTokens {
		return
	}

	// Keep the most recent messages that fit in the context
	// Always keep at least the last 2 exchanges (4 messages)
	minMessages := 4
	if len(m.chatMessages) <= minMessages {
		return
	}

	// Trim from the beginning, keeping system messages
	var trimmed []ChatMessage
	currentTokens := 0

	// Start from the end and work backwards
	for i := len(m.chatMessages) - 1; i >= 0; i-- {
		msg := m.chatMessages[i]
		msgTokens := msg.TotalTokens
		if msgTokens == 0 {
			msgTokens = estimateTokenCount(msg.Content)
		}

		if currentTokens+msgTokens <= maxContextTokens {
			trimmed = append([]ChatMessage{msg}, trimmed...)
			currentTokens += msgTokens
		} else {
			break
		}
	}

	m.chatMessages = trimmed
	m.statusMsg = fmt.Sprintf("⚠️  Trimmed conversation to %d messages (%d tokens)", len(trimmed), currentTokens)
	m.statusExpiry = time.Now().Add(5 * time.Second)
}
