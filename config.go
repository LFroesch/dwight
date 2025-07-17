package main

import (
	"encoding/json"
	"os"
)

func loadConfig(configFile string) Config {
	var config Config
	data, err := os.ReadFile(configFile)
	if err != nil {
		return config
	}

	var configData ConfigFile
	if json.Unmarshal(data, &configData) == nil && configData.App == "dwight" {
		return configData.Config
	}

	json.Unmarshal(data, &config)
	return config
}

func saveConfig(configFile string, config Config) {
	configData := ConfigFile{
		App:     "dwight",
		Version: "1.0",
		Config:  config,
	}

	data, err := json.MarshalIndent(configData, "", "  ")
	if err != nil {
		return
	}
	os.WriteFile(configFile, data, 0644)
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return !os.IsNotExist(err)
}