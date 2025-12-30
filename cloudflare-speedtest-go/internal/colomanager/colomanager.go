package colomanager

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ColoInfo represents a data center information
type ColoInfo struct {
	Code     string
	Location string
}

// ColoManager manages data center information
type ColoManager struct {
	dataDir string
	colos   map[string]string // code -> location
	list    []ColoInfo        // for ordered access
}

// New creates a new colo manager
func New(dataDir string) *ColoManager {
	return &ColoManager{
		dataDir: dataDir,
		colos:   make(map[string]string),
		list:    make([]ColoInfo, 0),
	}
}

// LoadColos loads data center information from colo.txt
func (cm *ColoManager) LoadColos() error {
	filePath := filepath.Join(cm.dataDir, "colo.txt")

	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open colo.txt: %w", err)
	}
	defer file.Close()

	cm.colos = make(map[string]string)
	cm.list = make([]ColoInfo, 0)
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Parse format: location,code
		parts := strings.Split(line, ",")
		if len(parts) >= 2 {
			location := strings.TrimSpace(parts[0])
			code := strings.TrimSpace(parts[1])

			// Extract code abbreviation (e.g., "LAX" from "LAX (Los Angeles)")
			codeAbbr := extractCodeAbbr(code)

			cm.colos[codeAbbr] = location
			cm.list = append(cm.list, ColoInfo{
				Code:     codeAbbr,
				Location: location,
			})
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error reading colo.txt: %w", err)
	}

	return nil
}

// extractCodeAbbr extracts the abbreviation from code string
func extractCodeAbbr(code string) string {
	// Remove parentheses and extra info
	if idx := strings.Index(code, "("); idx != -1 {
		code = code[:idx]
	}
	return strings.TrimSpace(code)
}

// GetLocation returns the location for a given code
func (cm *ColoManager) GetLocation(code string) string {
	return cm.colos[code]
}

// GetColos returns all data centers
func (cm *ColoManager) GetColos() []ColoInfo {
	colos := make([]ColoInfo, len(cm.list))
	copy(colos, cm.list)
	return colos
}

// GetCodes returns all data center codes
func (cm *ColoManager) GetCodes() []string {
	codes := make([]string, 0, len(cm.colos))
	for code := range cm.colos {
		codes = append(codes, code)
	}
	return codes
}

// ColoCount returns the number of data centers
func (cm *ColoManager) ColoCount() int {
	return len(cm.colos)
}

// HasColos checks if data centers are loaded
func (cm *ColoManager) HasColos() bool {
	return len(cm.colos) > 0
}
