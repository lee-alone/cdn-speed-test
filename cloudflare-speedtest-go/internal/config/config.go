package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Config holds application configuration
type Config struct {
	ExpectedServers int
	UseTLS          bool
	IPType          string
	Bandwidth       float64
	Timeout         int
	DownloadTime    int
	FilePath        string
}

const configFile = "config.ini"

// Load reads configuration from config.ini
func Load() (*Config, error) {
	cfg := &Config{
		ExpectedServers: 3,
		UseTLS:          false,
		IPType:          "ipv4",
		Bandwidth:       100,
		Timeout:         5,
		DownloadTime:    10,
		FilePath:        "./",
	}

	content, err := os.ReadFile(configFile)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return nil, err
	}

	lines := strings.Split(string(content), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "[") || strings.HasPrefix(line, ";") {
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		switch key {
		case "expected_servers":
			if v, err := strconv.Atoi(value); err == nil {
				cfg.ExpectedServers = v
			}
		case "use_tls":
			cfg.UseTLS = strings.ToLower(value) == "true"
		case "ip_type":
			cfg.IPType = value
		case "bandwidth":
			if v, err := strconv.ParseFloat(value, 64); err == nil {
				cfg.Bandwidth = v
			}
		case "timeout":
			if v, err := strconv.Atoi(value); err == nil {
				cfg.Timeout = v
			}
		case "download_time":
			if v, err := strconv.Atoi(value); err == nil {
				cfg.DownloadTime = v
			}
		case "filepath":
			cfg.FilePath = value
		}
	}

	return cfg, nil
}

// Save writes configuration to config.ini
func Save(cfg *Config) error {
	content := fmt.Sprintf(`[DEFAULT]
expected_servers = %d
use_tls = %v
ip_type = %s
bandwidth = %.0f
timeout = %d
download_time = %d
filepath = %s
`, cfg.ExpectedServers, cfg.UseTLS, cfg.IPType, cfg.Bandwidth, cfg.Timeout, cfg.DownloadTime, cfg.FilePath)

	return os.WriteFile(configFile, []byte(content), 0644)
}

// GetDataDir returns the directory for data files
func GetDataDir() string {
	return "."
}

// GetFilePath returns the full path for a data file
func GetFilePath(filename string) string {
	return filepath.Join(GetDataDir(), filename)
}
