package main

import (
	"os"
	"path/filepath"
)

func getHomeDir() string {
	if homeDir, err := os.UserHomeDir(); err == nil && homeDir != "" {
		return homeDir
	}

	if homeDir := os.Getenv("HOMEPATH"); homeDir != "" {
		return homeDir
	}

	return getInput("Path to home directory", false)
}

func getConfigDir() string {
	if configDir := os.Getenv("APPDATA"); configDir != "" {
		return configDir
	}
	return filepath.Join(getHomeDir(), "AppData", "Roaming")
}
