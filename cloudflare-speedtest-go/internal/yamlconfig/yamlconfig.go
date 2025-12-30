package yamlconfig

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config represents the application configuration
type Config struct {
	// Test settings
	Test TestConfig `yaml:"test" json:"test"`
	// Download settings
	Download DownloadConfig `yaml:"download" json:"download"`
}

// TestConfig represents test-related settings
type TestConfig struct {
	ExpectedServers int     `yaml:"expected_servers" json:"expected_servers"`
	UseTLS          bool    `yaml:"use_tls" json:"use_tls"`
	IPType          string  `yaml:"ip_type" json:"ip_type"`
	Bandwidth       float64 `yaml:"bandwidth" json:"bandwidth"`
	Timeout         int     `yaml:"timeout" json:"timeout"`
	DownloadTime    int     `yaml:"download_time" json:"download_time"`
	FilePath        string  `yaml:"file_path" json:"file_path"`
}

// DownloadConfig represents download-related settings
type DownloadConfig struct {
	URLs map[string]string `yaml:"urls" json:"urls"`
}

// DefaultConfig returns the default configuration
func DefaultConfig() *Config {
	return &Config{
		Test: TestConfig{
			ExpectedServers: 3,
			UseTLS:          false,
			IPType:          "ipv6",
			Bandwidth:       100,
			Timeout:         5,
			DownloadTime:    10,
			FilePath:        "./",
		},
		Download: DownloadConfig{
			URLs: map[string]string{
				"ips-v4.txt": "https://www.baipiao.eu.org/cloudflare/ips-v4",
				"ips-v6.txt": "https://www.baipiao.eu.org/cloudflare/ips-v6",
				"colo.txt":   "https://www.baipiao.eu.org/cloudflare/colo",
				"url.txt":    "https://www.baipiao.eu.org/cloudflare/url",
			},
		},
	}
}

// Load loads configuration from file, creates default if not exists
func Load(configPath string) (*Config, error) {
	// Check if file exists
	if _, err := os.Stat(configPath); err == nil {
		// File exists, read it
		return loadFromFile(configPath)
	} else if os.IsNotExist(err) {
		// File doesn't exist, create default
		cfg := DefaultConfig()
		if err := Save(configPath, cfg); err != nil {
			return nil, fmt.Errorf("failed to create default config: %w", err)
		}
		return cfg, nil
	} else {
		return nil, fmt.Errorf("failed to check config file: %w", err)
	}
}

// loadFromFile loads configuration from YAML file
func loadFromFile(configPath string) (*Config, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	cfg := DefaultConfig()
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Merge with defaults for missing values
	mergeWithDefaults(cfg)

	return cfg, nil
}

// mergeWithDefaults fills in missing values with defaults
func mergeWithDefaults(cfg *Config) {
	defaults := DefaultConfig()

	// Merge test config
	if cfg.Test.ExpectedServers == 0 {
		cfg.Test.ExpectedServers = defaults.Test.ExpectedServers
	}
	if cfg.Test.IPType == "" {
		cfg.Test.IPType = defaults.Test.IPType
	}
	if cfg.Test.Bandwidth == 0 {
		cfg.Test.Bandwidth = defaults.Test.Bandwidth
	}
	if cfg.Test.Timeout == 0 {
		cfg.Test.Timeout = defaults.Test.Timeout
	}
	if cfg.Test.DownloadTime == 0 {
		cfg.Test.DownloadTime = defaults.Test.DownloadTime
	}
	if cfg.Test.FilePath == "" {
		cfg.Test.FilePath = defaults.Test.FilePath
	}

	// Merge download config
	if cfg.Download.URLs == nil {
		cfg.Download.URLs = defaults.Download.URLs
	} else {
		// Merge individual URLs
		for key, value := range defaults.Download.URLs {
			if _, exists := cfg.Download.URLs[key]; !exists {
				cfg.Download.URLs[key] = value
			}
		}
	}
}

// Save saves configuration to YAML file
func Save(configPath string, cfg *Config) error {
	// Create directory if not exists
	dir := filepath.Dir(configPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Marshal to YAML
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	// Write to file
	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// GetDownloadURL returns the download URL for a file
func (cfg *Config) GetDownloadURL(filename string) string {
	if url, exists := cfg.Download.URLs[filename]; exists {
		return url
	}
	return ""
}

// GetAllDownloadURLs returns all download URLs
func (cfg *Config) GetAllDownloadURLs() map[string]string {
	urls := make(map[string]string)
	for key, value := range cfg.Download.URLs {
		urls[key] = value
	}
	return urls
}
