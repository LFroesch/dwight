package storage

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Paths returns the base data directory (~/.local/share/dwight/).
func DataDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "share", "dwight")
}

// --- App Config ---

type Config struct {
	TemplatesDir string   `json:"templates_dir"`
	FileTypes    []string `json:"file_types"`
}

type configFile struct {
	App     string `json:"app"`
	Version string `json:"version"`
	Config  Config `json:"config"`
}

func LoadConfig() Config {
	path := filepath.Join(DataDir(), "config.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return defaultConfig()
	}

	var cf configFile
	if json.Unmarshal(data, &cf) == nil && cf.App == "dwight" {
		return cf.Config
	}

	// Try raw config format
	var c Config
	json.Unmarshal(data, &c)
	return c
}

func SaveConfig(c Config) {
	os.MkdirAll(DataDir(), 0755)
	cf := configFile{App: "dwight", Version: "1.0", Config: c}
	data, err := json.MarshalIndent(cf, "", "  ")
	if err != nil {
		return
	}
	os.WriteFile(filepath.Join(DataDir(), "config.json"), data, 0644)
}

func defaultConfig() Config {
	c := Config{
		TemplatesDir: filepath.Join(DataDir(), "templates"),
		FileTypes:    []string{".md", ".txt", ".json", ".yaml", ".yml", ".xml", ".csv", ".log"},
	}
	os.MkdirAll(c.TemplatesDir, 0755)
	SaveConfig(c)
	return c
}

// --- App Settings ---

type Settings struct {
	MainPrompt  string `json:"main_prompt"`
	UserName    string `json:"user_name"`
	ChatTimeout int    `json:"chat_timeout"` // seconds
}

func LoadSettings() Settings {
	path := filepath.Join(DataDir(), "settings.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return defaultSettings()
	}
	var s Settings
	if err := json.Unmarshal(data, &s); err != nil {
		return defaultSettings()
	}
	return s
}

func SaveSettings(s Settings) {
	os.MkdirAll(DataDir(), 0755)
	data, _ := json.MarshalIndent(s, "", "  ")
	os.WriteFile(filepath.Join(DataDir(), "settings.json"), data, 0644)
}

func defaultSettings() Settings {
	s := Settings{UserName: "User", ChatTimeout: 180}
	SaveSettings(s)
	return s
}

// --- Model Profiles ---

type ModelProfile struct {
	Name         string  `json:"name"`
	Model        string  `json:"model"`
	SystemPrompt string  `json:"system_prompt"`
	Temperature  float64 `json:"temperature"`
}

type ModelConfig struct {
	Profiles       []ModelProfile `json:"profiles"`
	CurrentProfile int            `json:"current_profile"`
}

func defaultModel() string {
	if m := os.Getenv("DWIGHT_MODEL"); m != "" {
		return m
	}
	return "qwen2.5:7b"
}

var DefaultProfiles = []ModelProfile{
	{Name: "General Assistant", Model: defaultModel(), SystemPrompt: "You are a helpful AI assistant. Answer questions clearly and concisely.", Temperature: 0.7},
	{Name: "Coder Assistant", Model: defaultModel(), SystemPrompt: "You are a helpful coding assistant. Provide clear, concise code examples.", Temperature: 0.5},
	{Name: "Creative Writer", Model: defaultModel(), SystemPrompt: "You are a creative writing assistant. Be imaginative and descriptive.", Temperature: 0.9},
}

func LoadModelConfig() ModelConfig {
	path := filepath.Join(DataDir(), ".dwight-models.json")
	data, err := os.ReadFile(path)
	if err != nil {
		mc := ModelConfig{Profiles: DefaultProfiles, CurrentProfile: 0}
		SaveModelConfig(mc)
		return mc
	}
	var mc ModelConfig
	json.Unmarshal(data, &mc)
	if len(mc.Profiles) == 0 {
		mc.Profiles = DefaultProfiles
	}
	return mc
}

func SaveModelConfig(mc ModelConfig) {
	os.MkdirAll(DataDir(), 0755)
	data, _ := json.MarshalIndent(mc, "", "  ")
	os.WriteFile(filepath.Join(DataDir(), ".dwight-models.json"), data, 0644)
}

func (mc *ModelConfig) Current() ModelProfile {
	if mc.CurrentProfile >= 0 && mc.CurrentProfile < len(mc.Profiles) {
		return mc.Profiles[mc.CurrentProfile]
	}
	return DefaultProfiles[0]
}

// --- Conversations ---

type Conversation struct {
	ID           string        `json:"id"`
	Title        string        `json:"title"`
	Model        string        `json:"model"`
	ProfileName  string        `json:"profile_name"`
	Created      time.Time     `json:"created"`
	LastModified time.Time     `json:"last_modified"`
	Messages     []ConvMessage `json:"messages"`
	TotalTokens  int           `json:"total_tokens"`
	PromptTokens int           `json:"prompt_tokens"`
	MessageCount int           `json:"message_count"`
	Tags         []string      `json:"tags"`
	// WorkContext: where this chat was started (global store; not tied to cwd on reload).
	WorkingDir string `json:"working_dir,omitempty"`
	GitRoot    string `json:"git_root,omitempty"`
	OriginHint string `json:"origin_hint,omitempty"`
}

type ConvMessage struct {
	Role         string        `json:"role"`
	Content      string        `json:"content"`
	Timestamp    time.Time     `json:"timestamp"`
	Duration     time.Duration `json:"duration"`
	PromptTokens int           `json:"prompt_tokens"`
	TotalTokens  int           `json:"total_tokens"`
}

type ConversationMeta struct {
	ID           string    `json:"id"`
	Title        string    `json:"title"`
	Model        string    `json:"model"`
	ProfileName  string    `json:"profile_name"`
	Created      time.Time `json:"created"`
	LastModified time.Time `json:"last_modified"`
	MessageCount int       `json:"message_count"`
	TotalTokens  int       `json:"total_tokens"`
	WorkingDir   string    `json:"working_dir,omitempty"`
	GitRoot      string    `json:"git_root,omitempty"`
	OriginHint   string    `json:"origin_hint,omitempty"`
}

// Where returns a short label for lists (origin, repo name, or working directory).
func (m ConversationMeta) Where() string {
	return ContextLabel(WorkContext{WorkingDir: m.WorkingDir, GitRoot: m.GitRoot, OriginHint: m.OriginHint})
}

// ConversationsDir is ~/.local/share/dwight/conversations (all chats in one place).
func ConversationsDir() string {
	return filepath.Join(DataDir(), "conversations")
}

func SaveConversation(conv *Conversation) error {
	dir := ConversationsDir()
	os.MkdirAll(dir, 0755)

	conv.LastModified = time.Now()
	conv.MessageCount = len(conv.Messages)

	totalTokens, promptTokens := 0, 0
	for _, msg := range conv.Messages {
		totalTokens += msg.TotalTokens
		promptTokens += msg.PromptTokens
	}
	conv.TotalTokens = totalTokens
	conv.PromptTokens = promptTokens

	data, err := json.MarshalIndent(conv, "", "  ")
	if err != nil {
		return err
	}

	path := filepath.Join(dir, conv.ID+".json")
	if err := os.WriteFile(path, data, 0644); err != nil {
		return err
	}
	return nil
}

func LoadConversation(id string) (*Conversation, error) {
	path := filepath.Join(ConversationsDir(), id+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var conv Conversation
	if err := json.Unmarshal(data, &conv); err != nil {
		return nil, err
	}
	return &conv, nil
}

func DeleteConversation(id string) error {
	return os.Remove(filepath.Join(ConversationsDir(), id+".json"))
}

func ListConversations() ([]ConversationMeta, error) {
	dir := ConversationsDir()
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return nil, nil
	}

	files, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var convs []ConversationMeta
	for _, file := range files {
		if file.IsDir() || !strings.HasSuffix(file.Name(), ".json") {
			continue
		}
		id := strings.TrimSuffix(file.Name(), ".json")
		conv, err := LoadConversation(id)
		if err != nil {
			continue
		}
		convs = append(convs, ConversationMeta{
			ID: conv.ID, Title: conv.Title, Model: conv.Model,
			ProfileName: conv.ProfileName, Created: conv.Created,
			LastModified: conv.LastModified, MessageCount: conv.MessageCount,
			TotalTokens: conv.TotalTokens,
			WorkingDir: conv.WorkingDir, GitRoot: conv.GitRoot, OriginHint: conv.OriginHint,
		})
	}

	sort.Slice(convs, func(i, j int) bool {
		return convs[i].LastModified.After(convs[j].LastModified)
	})
	return convs, nil
}

func GenerateTitle(messages []ConvMessage) string {
	for _, msg := range messages {
		if msg.Role == "user" {
			title := strings.ReplaceAll(msg.Content, "\n", " ")
			if len(title) > 50 {
				title = title[:50] + "..."
			}
			return title
		}
	}
	return "Untitled Conversation"
}

// ExportMarkdown exports a conversation to markdown format.
func ExportMarkdown(conv *Conversation) string {
	var md strings.Builder
	md.WriteString(fmt.Sprintf("# %s\n\n", conv.Title))
	md.WriteString(fmt.Sprintf("**Model:** %s (%s)  \n", conv.Model, conv.ProfileName))
	md.WriteString(fmt.Sprintf("**Created:** %s  \n", conv.Created.Format("January 2, 2006 3:04 PM")))
	if label := ContextLabel(WorkContext{WorkingDir: conv.WorkingDir, GitRoot: conv.GitRoot, OriginHint: conv.OriginHint}); label != "" {
		md.WriteString(fmt.Sprintf("**Where:** %s  \n", label))
	}
	md.WriteString(fmt.Sprintf("**Messages:** %d | **Tokens:** %d  \n\n---\n\n", conv.MessageCount, conv.TotalTokens))

	for _, msg := range conv.Messages {
		if msg.Role == "user" {
			md.WriteString("## User\n\n")
		} else {
			md.WriteString("## Assistant\n\n")
			if msg.Duration > 0 {
				md.WriteString(fmt.Sprintf("*%.1fs | %d tokens*\n\n", msg.Duration.Seconds(), msg.TotalTokens))
			}
		}
		md.WriteString(msg.Content + "\n\n---\n\n")
	}
	return md.String()
}

// ExportJSON exports a conversation to pretty JSON.
func ExportJSON(conv *Conversation) (string, error) {
	data, err := json.MarshalIndent(conv, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}
