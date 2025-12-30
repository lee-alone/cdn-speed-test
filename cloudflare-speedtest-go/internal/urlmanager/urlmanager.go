package urlmanager

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// URLManager manages test URLs
type URLManager struct {
	dataDir string
	urls    []string
}

// New creates a new URL manager
func New(dataDir string) *URLManager {
	return &URLManager{
		dataDir: dataDir,
		urls:    make([]string, 0),
	}
}

// LoadURLs loads URLs from url.txt file
func (um *URLManager) LoadURLs() error {
	filePath := filepath.Join(um.dataDir, "url.txt")

	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open url.txt: %w", err)
	}
	defer file.Close()

	um.urls = make([]string, 0)
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		// Skip empty lines and comments
		if line != "" && !strings.HasPrefix(line, "#") {
			um.urls = append(um.urls, line)
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error reading url.txt: %w", err)
	}

	return nil
}

// GetURLs returns all loaded URLs
func (um *URLManager) GetURLs() []string {
	urls := make([]string, len(um.urls))
	copy(urls, um.urls)
	return urls
}

// GetRandomURL returns a random URL from the list
func (um *URLManager) GetRandomURL() (string, error) {
	if len(um.urls) == 0 {
		return "", fmt.Errorf("no URLs available")
	}

	// For now, return the first URL
	// TODO: Implement random selection
	return um.urls[0], nil
}

// URLCount returns the number of loaded URLs
func (um *URLManager) URLCount() int {
	return len(um.urls)
}

// HasURLs checks if URLs are loaded
func (um *URLManager) HasURLs() bool {
	return len(um.urls) > 0
}
