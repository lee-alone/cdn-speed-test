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
	// UI settings
	UI UIConfig `yaml:"ui" json:"ui"`
	// Advanced settings
	Advanced AdvancedConfig `yaml:"advanced" json:"advanced"`
}

// TestConfig represents test-related settings
type TestConfig struct {
	ExpectedServers   int     `yaml:"expected_servers" json:"expected_servers"`
	UseTLS            bool    `yaml:"use_tls" json:"use_tls"`
	IPType            string  `yaml:"ip_type" json:"ip_type"`
	Bandwidth         float64 `yaml:"bandwidth" json:"bandwidth"`
	Timeout           int     `yaml:"timeout" json:"timeout"`
	DownloadTime      int     `yaml:"download_time" json:"download_time"`
	FilePath          string  `yaml:"file_path" json:"file_path"`
	DataCenterFilter  string  `yaml:"datacenter_filter" json:"datacenter_filter"`
	ConcurrentWorkers int     `yaml:"concurrent_workers" json:"concurrent_workers"`
	SampleInterval    int     `yaml:"sample_interval" json:"sample_interval"`
}

// DownloadConfig represents download-related settings
type DownloadConfig struct {
	URLs map[string]string `yaml:"urls" json:"urls"`
}

// UIConfig represents UI-related settings
type UIConfig struct {
	DataCenterFilter string `yaml:"datacenter_filter" json:"datacenter_filter"`
	ResultFormat     string `yaml:"result_format" json:"result_format"`
	AutoRefresh      bool   `yaml:"auto_refresh" json:"auto_refresh"`
	Theme            string `yaml:"theme" json:"theme"`
}

// AdvancedConfig represents advanced system settings
type AdvancedConfig struct {
	ConcurrentWorkers int    `yaml:"concurrent_workers" json:"concurrent_workers"`
	RetryAttempts     int    `yaml:"retry_attempts" json:"retry_attempts"`
	LogLevel          string `yaml:"log_level" json:"log_level"`
	EnableMetrics     bool   `yaml:"enable_metrics" json:"enable_metrics"`
}

// DefaultConfig returns the default configuration
func DefaultConfig() *Config {
	return &Config{
		Test: TestConfig{
			ExpectedServers:   3,
			UseTLS:            false,
			IPType:            "ipv6",
			Bandwidth:         100,
			Timeout:           5,
			DownloadTime:      10,
			FilePath:          "./",
			DataCenterFilter:  "all",
			ConcurrentWorkers: 10,
			SampleInterval:    1,
		},
		Download: DownloadConfig{
			URLs: map[string]string{
				"ips-v4.txt": "https://www.baipiao.eu.org/cloudflare/ips-v4",
				"ips-v6.txt": "https://www.baipiao.eu.org/cloudflare/ips-v6",
				"colo.txt":   "https://www.baipiao.eu.org/cloudflare/colo",
				"url.txt":    "https://www.baipiao.eu.org/cloudflare/url",
			},
		},
		UI: UIConfig{
			DataCenterFilter: "all",
			ResultFormat:     "table",
			AutoRefresh:      true,
			Theme:            "light",
		},
		Advanced: AdvancedConfig{
			ConcurrentWorkers: 10,
			RetryAttempts:     3,
			LogLevel:          "info",
			EnableMetrics:     true,
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
	if cfg.Test.DataCenterFilter == "" {
		cfg.Test.DataCenterFilter = defaults.Test.DataCenterFilter
	}
	if cfg.Test.ConcurrentWorkers == 0 {
		cfg.Test.ConcurrentWorkers = defaults.Test.ConcurrentWorkers
	}
	if cfg.Test.SampleInterval == 0 {
		cfg.Test.SampleInterval = defaults.Test.SampleInterval
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

	// Merge UI config
	if cfg.UI.DataCenterFilter == "" {
		cfg.UI.DataCenterFilter = defaults.UI.DataCenterFilter
	}
	if cfg.UI.ResultFormat == "" {
		cfg.UI.ResultFormat = defaults.UI.ResultFormat
	}
	if cfg.UI.Theme == "" {
		cfg.UI.Theme = defaults.UI.Theme
	}

	// Merge advanced config
	if cfg.Advanced.ConcurrentWorkers == 0 {
		cfg.Advanced.ConcurrentWorkers = defaults.Advanced.ConcurrentWorkers
	}
	if cfg.Advanced.RetryAttempts == 0 {
		cfg.Advanced.RetryAttempts = defaults.Advanced.RetryAttempts
	}
	if cfg.Advanced.LogLevel == "" {
		cfg.Advanced.LogLevel = defaults.Advanced.LogLevel
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

// ValidationError represents a configuration validation error
type ValidationError struct {
	Field   string
	Value   interface{}
	Message string
}

func (e ValidationError) Error() string {
	return fmt.Sprintf("validation error for field '%s' with value '%v': %s", e.Field, e.Value, e.Message)
}

// ValidationErrors represents multiple validation errors
type ValidationErrors []ValidationError

func (e ValidationErrors) Error() string {
	if len(e) == 0 {
		return "no validation errors"
	}
	if len(e) == 1 {
		return e[0].Error()
	}
	return fmt.Sprintf("%s (and %d more errors)", e[0].Error(), len(e)-1)
}

// Validate validates the configuration and returns any validation errors
func (cfg *Config) Validate() error {
	var errors ValidationErrors

	// Validate test config
	if cfg.Test.ExpectedServers < 1 || cfg.Test.ExpectedServers > 100 {
		errors = append(errors, ValidationError{
			Field:   "test.expected_servers",
			Value:   cfg.Test.ExpectedServers,
			Message: "must be between 1 and 100",
		})
	}

	if cfg.Test.IPType != "ipv4" && cfg.Test.IPType != "ipv6" {
		errors = append(errors, ValidationError{
			Field:   "test.ip_type",
			Value:   cfg.Test.IPType,
			Message: "must be 'ipv4' or 'ipv6'",
		})
	}

	if cfg.Test.Bandwidth < 0.1 || cfg.Test.Bandwidth > 10000 {
		errors = append(errors, ValidationError{
			Field:   "test.bandwidth",
			Value:   cfg.Test.Bandwidth,
			Message: "must be between 0.1 and 10000 Mbps",
		})
	}

	if cfg.Test.Timeout < 1 || cfg.Test.Timeout > 300 {
		errors = append(errors, ValidationError{
			Field:   "test.timeout",
			Value:   cfg.Test.Timeout,
			Message: "must be between 1 and 300 seconds",
		})
	}

	if cfg.Test.DownloadTime < 1 || cfg.Test.DownloadTime > 300 {
		errors = append(errors, ValidationError{
			Field:   "test.download_time",
			Value:   cfg.Test.DownloadTime,
			Message: "must be between 1 and 300 seconds",
		})
	}

	if cfg.Test.ConcurrentWorkers < 1 || cfg.Test.ConcurrentWorkers > 100 {
		errors = append(errors, ValidationError{
			Field:   "test.concurrent_workers",
			Value:   cfg.Test.ConcurrentWorkers,
			Message: "must be between 1 and 100",
		})
	}

	if cfg.Test.SampleInterval < 1 || cfg.Test.SampleInterval > 60 {
		errors = append(errors, ValidationError{
			Field:   "test.sample_interval",
			Value:   cfg.Test.SampleInterval,
			Message: "must be between 1 and 60 seconds",
		})
	}

	// Validate UI config
	validResultFormats := []string{"table", "json", "csv"}
	validFormat := false
	for _, format := range validResultFormats {
		if cfg.UI.ResultFormat == format {
			validFormat = true
			break
		}
	}
	if !validFormat {
		errors = append(errors, ValidationError{
			Field:   "ui.result_format",
			Value:   cfg.UI.ResultFormat,
			Message: "must be one of: table, json, csv",
		})
	}

	validThemes := []string{"light", "dark", "auto"}
	validTheme := false
	for _, theme := range validThemes {
		if cfg.UI.Theme == theme {
			validTheme = true
			break
		}
	}
	if !validTheme {
		errors = append(errors, ValidationError{
			Field:   "ui.theme",
			Value:   cfg.UI.Theme,
			Message: "must be one of: light, dark, auto",
		})
	}

	// Validate advanced config
	if cfg.Advanced.ConcurrentWorkers < 1 || cfg.Advanced.ConcurrentWorkers > 100 {
		errors = append(errors, ValidationError{
			Field:   "advanced.concurrent_workers",
			Value:   cfg.Advanced.ConcurrentWorkers,
			Message: "must be between 1 and 100",
		})
	}

	if cfg.Advanced.RetryAttempts < 0 || cfg.Advanced.RetryAttempts > 10 {
		errors = append(errors, ValidationError{
			Field:   "advanced.retry_attempts",
			Value:   cfg.Advanced.RetryAttempts,
			Message: "must be between 0 and 10",
		})
	}

	validLogLevels := []string{"debug", "info", "warn", "error"}
	validLogLevel := false
	for _, level := range validLogLevels {
		if cfg.Advanced.LogLevel == level {
			validLogLevel = true
			break
		}
	}
	if !validLogLevel {
		errors = append(errors, ValidationError{
			Field:   "advanced.log_level",
			Value:   cfg.Advanced.LogLevel,
			Message: "must be one of: debug, info, warn, error",
		})
	}

	// Validate download URLs
	if len(cfg.Download.URLs) == 0 {
		errors = append(errors, ValidationError{
			Field:   "download.urls",
			Value:   len(cfg.Download.URLs),
			Message: "at least one download URL must be configured",
		})
	}

	if len(errors) > 0 {
		return errors
	}
	return nil
}

// LoadAndValidate loads configuration from file and validates it
func LoadAndValidate(configPath string) (*Config, error) {
	cfg, err := Load(configPath)
	if err != nil {
		return nil, err
	}

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("configuration validation failed: %w", err)
	}

	return cfg, nil
}

// SaveWithValidation validates configuration before saving
func SaveWithValidation(configPath string, cfg *Config) error {
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("configuration validation failed: %w", err)
	}

	return Save(configPath, cfg)
}
