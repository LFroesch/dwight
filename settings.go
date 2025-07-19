package main

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type AppSettings struct {
	MainPrompt      string `json:"main_prompt"`
	MemoryAllotment string `json:"memory_allotment"`
	UserName        string `json:"user_name"`
	ChatTimeout     int    `json:"chat_timeout"` // seconds
}

func defaultSettings() AppSettings {
	return AppSettings{
		MainPrompt:      "",
		MemoryAllotment: "4GB",
		UserName:        "User",
		ChatTimeout:     180,
	}
}

func (m *model) loadSettings() {
	settingsPath := filepath.Join(filepath.Dir(m.configFile), "settings.json")

	data, err := os.ReadFile(settingsPath)
	if err != nil {
		m.appSettings = defaultSettings()
		m.saveSettings()
		return
	}

	if err := json.Unmarshal(data, &m.appSettings); err != nil {
		m.appSettings = defaultSettings()
		m.saveSettings()
	}
}

func (m *model) saveSettings() {
	settingsPath := filepath.Join(filepath.Dir(m.configFile), "settings.json")

	data, err := json.MarshalIndent(m.appSettings, "", "  ")
	if err != nil {
		return
	}

	os.WriteFile(settingsPath, data, 0644)
}
