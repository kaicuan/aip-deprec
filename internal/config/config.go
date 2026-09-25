package config

import (
	"os"
	"path/filepath"
)

const (
	DefaultPort = 9100
	UpstreamBaseURL = "https://opencode.ai/zen/v1"
)

type Config struct {
	DataDir      string
	DBPath       string
	PIDPath      string
	LogPath      string
	OpenCodeJSON string
	Port         int
}

// Load initializes default runtime directories and loads configuration paths.
func Load() (*Config, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	dataDir := filepath.Join(home, ".aip")
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, err
	}

	return &Config{
		DataDir:      dataDir,
		DBPath:       filepath.Join(dataDir, "aip.db"),
		PIDPath:      filepath.Join(dataDir, "aip.pid"),
		LogPath:      filepath.Join(dataDir, "aip.log"),
		OpenCodeJSON: filepath.Join(home, ".config", "opencode", "opencode.json"),
		Port:         DefaultPort,
	}, nil
}

// GetConfig aliases Load for backward compatibility.
func GetConfig() (*Config, error) {
	return Load()
}
