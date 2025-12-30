package colomanager

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// ColoInfo represents a data center information
type ColoInfo struct {
	Code     string `json:"code"`
	Location string `json:"location"`
	Region   string `json:"region"`
}

// DataCenterManager manages data center information with enhanced filtering
type DataCenterManager struct {
	dataDir     string
	colos       map[string]ColoInfo // code -> ColoInfo
	list        []ColoInfo          // for ordered access
	filterMode  string              // "all", "selected", or specific codes
	selectedDCs []string            // selected data center codes
	mu          sync.RWMutex        // for thread safety
	lastLoaded  time.Time           // cache timestamp
	cacheValid  bool                // cache validity flag
}

// ColoManager is an alias for backward compatibility
type ColoManager = DataCenterManager

// New creates a new data center manager
func New(dataDir string) *DataCenterManager {
	return &DataCenterManager{
		dataDir:     dataDir,
		colos:       make(map[string]ColoInfo),
		list:        make([]ColoInfo, 0),
		filterMode:  "all",
		selectedDCs: make([]string, 0),
		cacheValid:  false,
	}
}

// LoadColos loads data center information from colo.txt with caching
func (dcm *DataCenterManager) LoadColos() error {
	dcm.mu.Lock()
	defer dcm.mu.Unlock()

	// Check if cache is still valid (5 minutes)
	if dcm.cacheValid && time.Since(dcm.lastLoaded) < 5*time.Minute {
		return nil
	}

	filePath := filepath.Join(dcm.dataDir, "colo.txt")

	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open colo.txt: %w", err)
	}
	defer file.Close()

	dcm.colos = make(map[string]ColoInfo)
	dcm.list = make([]ColoInfo, 0)
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Parse format: location,code or location,code,region
		parts := strings.Split(line, ",")
		if len(parts) >= 2 {
			location := strings.TrimSpace(parts[0])
			code := strings.TrimSpace(parts[1])
			region := ""
			if len(parts) >= 3 {
				region = strings.TrimSpace(parts[2])
			} else {
				region = inferRegion(location)
			}

			// Extract code abbreviation (e.g., "LAX" from "LAX (Los Angeles)")
			codeAbbr := extractCodeAbbr(code)

			coloInfo := ColoInfo{
				Code:     codeAbbr,
				Location: location,
				Region:   region,
			}

			dcm.colos[codeAbbr] = coloInfo
			dcm.list = append(dcm.list, coloInfo)
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error reading colo.txt: %w", err)
	}

	dcm.lastLoaded = time.Now()
	dcm.cacheValid = true

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

// inferRegion infers the region from location name
func inferRegion(location string) string {
	location = strings.ToLower(location)

	// North America
	if strings.Contains(location, "united states") || strings.Contains(location, "canada") ||
		strings.Contains(location, "mexico") || strings.Contains(location, "usa") {
		return "North America"
	}

	// Europe
	if strings.Contains(location, "united kingdom") || strings.Contains(location, "germany") ||
		strings.Contains(location, "france") || strings.Contains(location, "netherlands") ||
		strings.Contains(location, "spain") || strings.Contains(location, "italy") ||
		strings.Contains(location, "poland") || strings.Contains(location, "sweden") ||
		strings.Contains(location, "finland") || strings.Contains(location, "norway") ||
		strings.Contains(location, "denmark") || strings.Contains(location, "belgium") ||
		strings.Contains(location, "switzerland") || strings.Contains(location, "austria") ||
		strings.Contains(location, "czech") || strings.Contains(location, "portugal") ||
		strings.Contains(location, "ireland") || strings.Contains(location, "romania") ||
		strings.Contains(location, "bulgaria") || strings.Contains(location, "greece") ||
		strings.Contains(location, "turkey") || strings.Contains(location, "russia") {
		return "Europe"
	}

	// Asia Pacific
	if strings.Contains(location, "china") || strings.Contains(location, "japan") ||
		strings.Contains(location, "korea") || strings.Contains(location, "singapore") ||
		strings.Contains(location, "hong kong") || strings.Contains(location, "taiwan") ||
		strings.Contains(location, "thailand") || strings.Contains(location, "malaysia") ||
		strings.Contains(location, "indonesia") || strings.Contains(location, "philippines") ||
		strings.Contains(location, "vietnam") || strings.Contains(location, "india") ||
		strings.Contains(location, "australia") || strings.Contains(location, "new zealand") {
		return "Asia Pacific"
	}

	// South America
	if strings.Contains(location, "brazil") || strings.Contains(location, "argentina") ||
		strings.Contains(location, "chile") || strings.Contains(location, "colombia") ||
		strings.Contains(location, "peru") || strings.Contains(location, "ecuador") {
		return "South America"
	}

	// Africa
	if strings.Contains(location, "south africa") || strings.Contains(location, "egypt") ||
		strings.Contains(location, "kenya") || strings.Contains(location, "nigeria") ||
		strings.Contains(location, "morocco") {
		return "Africa"
	}

	// Middle East
	if strings.Contains(location, "israel") || strings.Contains(location, "uae") ||
		strings.Contains(location, "saudi arabia") || strings.Contains(location, "qatar") ||
		strings.Contains(location, "bahrain") || strings.Contains(location, "kuwait") {
		return "Middle East"
	}

	return "Other"
}

// GetLocation returns the location for a given code
func (dcm *DataCenterManager) GetLocation(code string) string {
	dcm.mu.RLock()
	defer dcm.mu.RUnlock()

	if colo, exists := dcm.colos[code]; exists {
		return colo.Location
	}
	return ""
}

// GetDataCenter returns the data center info for a given code
func (dcm *DataCenterManager) GetDataCenter(code string) (ColoInfo, bool) {
	dcm.mu.RLock()
	defer dcm.mu.RUnlock()

	colo, exists := dcm.colos[code]
	return colo, exists
}

// GetColos returns all data centers
func (dcm *DataCenterManager) GetColos() []ColoInfo {
	dcm.mu.RLock()
	defer dcm.mu.RUnlock()

	colos := make([]ColoInfo, len(dcm.list))
	copy(colos, dcm.list)
	return colos
}

// GetCodes returns all data center codes
func (dcm *DataCenterManager) GetCodes() []string {
	dcm.mu.RLock()
	defer dcm.mu.RUnlock()

	codes := make([]string, 0, len(dcm.colos))
	for code := range dcm.colos {
		codes = append(codes, code)
	}
	return codes
}

// ColoCount returns the number of data centers
func (dcm *DataCenterManager) ColoCount() int {
	dcm.mu.RLock()
	defer dcm.mu.RUnlock()

	return len(dcm.colos)
}

// HasColos checks if data centers are loaded
func (dcm *DataCenterManager) HasColos() bool {
	dcm.mu.RLock()
	defer dcm.mu.RUnlock()

	return len(dcm.colos) > 0
}

// SetFilterMode sets the data center filtering mode
func (dcm *DataCenterManager) SetFilterMode(mode string) {
	dcm.mu.Lock()
	defer dcm.mu.Unlock()

	dcm.filterMode = mode
}

// GetFilterMode returns the current filtering mode
func (dcm *DataCenterManager) GetFilterMode() string {
	dcm.mu.RLock()
	defer dcm.mu.RUnlock()

	return dcm.filterMode
}

// SetSelectedDataCenters sets the list of selected data centers for filtering
func (dcm *DataCenterManager) SetSelectedDataCenters(codes []string) {
	dcm.mu.Lock()
	defer dcm.mu.Unlock()

	dcm.selectedDCs = make([]string, len(codes))
	copy(dcm.selectedDCs, codes)

	if len(codes) > 0 {
		dcm.filterMode = "selected"
	} else {
		dcm.filterMode = "all"
	}
}

// GetSelectedDataCenters returns the list of selected data centers
func (dcm *DataCenterManager) GetSelectedDataCenters() []string {
	dcm.mu.RLock()
	defer dcm.mu.RUnlock()

	selected := make([]string, len(dcm.selectedDCs))
	copy(selected, dcm.selectedDCs)
	return selected
}

// FilterByDataCenter checks if an IP should be tested based on its data center
func (dcm *DataCenterManager) FilterByDataCenter(ipDataCenter string) bool {
	dcm.mu.RLock()
	defer dcm.mu.RUnlock()

	switch dcm.filterMode {
	case "all":
		return true
	case "selected":
		for _, selected := range dcm.selectedDCs {
			if selected == ipDataCenter {
				return true
			}
		}
		return false
	default:
		// Treat unknown modes as "all"
		return true
	}
}

// GetAvailableDataCenters returns a list of available data centers for selection
func (dcm *DataCenterManager) GetAvailableDataCenters() []ColoInfo {
	return dcm.GetColos()
}

// GetDataCentersByRegion returns data centers grouped by region
func (dcm *DataCenterManager) GetDataCentersByRegion() map[string][]ColoInfo {
	dcm.mu.RLock()
	defer dcm.mu.RUnlock()

	regions := make(map[string][]ColoInfo)

	for _, colo := range dcm.list {
		region := colo.Region
		if region == "" {
			region = "Other"
		}
		regions[region] = append(regions[region], colo)
	}

	return regions
}

// InvalidateCache forces a reload of data center information on next access
func (dcm *DataCenterManager) InvalidateCache() {
	dcm.mu.Lock()
	defer dcm.mu.Unlock()

	dcm.cacheValid = false
}

// GetFriendlyName returns a friendly display name for a data center code
func (dcm *DataCenterManager) GetFriendlyName(code string) string {
	dcm.mu.RLock()
	defer dcm.mu.RUnlock()

	if colo, exists := dcm.colos[code]; exists {
		return fmt.Sprintf("%s (%s)", colo.Location, code)
	}
	return code // Return code if not found
}
