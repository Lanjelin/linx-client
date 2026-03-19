package main

import (
	"os"
	"path/filepath"
)

func getHomeDir() string {
	if homeDir, err := os.UserHomeDir(); err == nil && homeDir != "" {
		return homeDir
	}

	return getInput("Path to home directory", false)
}

func getConfigDir() string {
	return filepath.Join(getHomeDir(), ".config")
}
