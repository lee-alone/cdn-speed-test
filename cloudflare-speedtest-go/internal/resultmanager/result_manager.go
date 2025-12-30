package resultmanager

import (
	"cloudflare-speedtest/pkg/models"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// ResultManager manages test results with storage, export, and memory management
type ResultManager struct {
	results    []*models.SpeedTestResult
	mu         sync.RWMutex
	maxResults int               // Maximum number of results to keep in memory
	ipSet      map[string]bool   // Track unique IPs to prevent duplicates
	stats      *models.TestStats // Real-time statistics
	statsMu    sync.RWMutex
}

// ExportFormat represents different export formats
type ExportFormat string

const (
	FormatCSV  ExportFormat = "csv"
	FormatJSON ExportFormat = "json"
	FormatTXT  ExportFormat = "txt"
)

// New creates a new result manager
func New(maxResults int) *ResultManager {
	return &ResultManager{
		results:    make([]*models.SpeedTestResult, 0),
		maxResults: maxResults,
		ipSet:      make(map[string]bool),
		stats: &models.TestStats{
			Total:        0,
			Completed:    0,
			Qualified:    0,
			CurrentIP:    "",
			CurrentSpeed: "",
		},
	}
}

// AddResult adds a new test result with duplicate IP detection
func (rm *ResultManager) AddResult(result *models.SpeedTestResult) error {
	if result == nil {
		return fmt.Errorf("result cannot be nil")
	}

	rm.mu.Lock()
	defer rm.mu.Unlock()

	// Check for duplicate IP
	if rm.ipSet[result.IP] {
		return fmt.Errorf("duplicate IP: %s already exists", result.IP)
	}

	// Add result
	rm.results = append(rm.results, result)
	rm.ipSet[result.IP] = true

	// Update statistics
	rm.updateStats(result)

	// Manage memory by removing oldest results if limit exceeded
	if len(rm.results) > rm.maxResults {
		rm.removeOldestResult()
	}

	return nil
}

// AddResultAllowDuplicate adds a result without duplicate checking (for updates)
func (rm *ResultManager) AddResultAllowDuplicate(result *models.SpeedTestResult) {
	if result == nil {
		return
	}

	rm.mu.Lock()
	defer rm.mu.Unlock()

	rm.results = append(rm.results, result)
	rm.ipSet[result.IP] = true
	rm.updateStats(result)

	// Manage memory
	if len(rm.results) > rm.maxResults {
		rm.removeOldestResult()
	}
}

// GetResults returns a copy of all results
func (rm *ResultManager) GetResults() []*models.SpeedTestResult {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	results := make([]*models.SpeedTestResult, len(rm.results))
	copy(results, rm.results)
	return results
}

// GetSortedResults returns results sorted by specified criteria
func (rm *ResultManager) GetSortedResults(sortBy string, ascending bool) []*models.SpeedTestResult {
	results := rm.GetResults()

	sort.Slice(results, func(i, j int) bool {
		var less bool
		switch sortBy {
		case "speed":
			speedI, _ := strconv.ParseFloat(results[i].Speed, 64)
			speedJ, _ := strconv.ParseFloat(results[j].Speed, 64)
			less = speedI < speedJ
		case "latency":
			latencyI, _ := strconv.ParseFloat(results[i].Latency, 64)
			latencyJ, _ := strconv.ParseFloat(results[j].Latency, 64)
			less = latencyI < latencyJ
		case "datacenter":
			less = results[i].DataCenter < results[j].DataCenter
		case "ip":
			less = results[i].IP < results[j].IP
		default: // Default sort by speed
			speedI, _ := strconv.ParseFloat(results[i].Speed, 64)
			speedJ, _ := strconv.ParseFloat(results[j].Speed, 64)
			less = speedI < speedJ
		}

		if ascending {
			return less
		}
		return !less
	})

	return results
}

// GetQualifiedResults returns only completed/qualified results
func (rm *ResultManager) GetQualifiedResults() []*models.SpeedTestResult {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	qualified := make([]*models.SpeedTestResult, 0)
	for _, result := range rm.results {
		if result.Status == "已完成" {
			qualified = append(qualified, result)
		}
	}

	return qualified
}

// GetStats returns current statistics
func (rm *ResultManager) GetStats() *models.TestStats {
	rm.statsMu.RLock()
	defer rm.statsMu.RUnlock()

	return &models.TestStats{
		Total:        rm.stats.Total,
		Completed:    rm.stats.Completed,
		Qualified:    rm.stats.Qualified,
		CurrentIP:    rm.stats.CurrentIP,
		CurrentSpeed: rm.stats.CurrentSpeed,
	}
}

// UpdateCurrentTest updates the current testing information
func (rm *ResultManager) UpdateCurrentTest(ip, speed string) {
	rm.statsMu.Lock()
	defer rm.statsMu.Unlock()

	rm.stats.CurrentIP = ip
	rm.stats.CurrentSpeed = speed
}

// SetTotal sets the total number of tests
func (rm *ResultManager) SetTotal(total int) {
	rm.statsMu.Lock()
	defer rm.statsMu.Unlock()

	rm.stats.Total = total
}

// Clear removes all results and resets statistics
func (rm *ResultManager) Clear() {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	rm.results = make([]*models.SpeedTestResult, 0)
	rm.ipSet = make(map[string]bool)

	rm.statsMu.Lock()
	rm.stats = &models.TestStats{
		Total:        0,
		Completed:    0,
		Qualified:    0,
		CurrentIP:    "",
		CurrentSpeed: "",
	}
	rm.statsMu.Unlock()
}

// ExportToCSV exports results to CSV format
func (rm *ResultManager) ExportToCSV(writer io.Writer, sortBy string, ascending bool) error {
	results := rm.GetSortedResults(sortBy, ascending)

	csvWriter := csv.NewWriter(writer)
	defer csvWriter.Flush()

	// Write header
	header := []string{"IP", "Status", "Latency(ms)", "Speed(Mbps)", "PeakSpeed(Mbps)", "DataCenter"}
	if err := csvWriter.Write(header); err != nil {
		return fmt.Errorf("failed to write CSV header: %w", err)
	}

	// Write data
	for _, result := range results {
		record := []string{
			result.IP,
			result.Status,
			result.Latency,
			result.Speed,
			fmt.Sprintf("%.2f", result.PeakSpeed),
			result.DataCenter,
		}
		if err := csvWriter.Write(record); err != nil {
			return fmt.Errorf("failed to write CSV record: %w", err)
		}
	}

	return nil
}

// ExportToJSON exports results to JSON format
func (rm *ResultManager) ExportToJSON(writer io.Writer, sortBy string, ascending bool) error {
	results := rm.GetSortedResults(sortBy, ascending)

	encoder := json.NewEncoder(writer)
	encoder.SetIndent("", "  ")

	exportData := map[string]interface{}{
		"timestamp":       time.Now().Format(time.RFC3339),
		"total_count":     len(results),
		"qualified_count": len(rm.GetQualifiedResults()),
		"results":         results,
		"statistics":      rm.GetStats(),
	}

	if err := encoder.Encode(exportData); err != nil {
		return fmt.Errorf("failed to encode JSON: %w", err)
	}

	return nil
}

// ExportToTXT exports results to human-readable text format
func (rm *ResultManager) ExportToTXT(writer io.Writer, sortBy string, ascending bool) error {
	results := rm.GetSortedResults(sortBy, ascending)
	stats := rm.GetStats()

	// Write header
	fmt.Fprintf(writer, "Cloudflare IP Speed Test Results\n")
	fmt.Fprintf(writer, "Generated: %s\n", time.Now().Format("2006-01-02 15:04:05"))
	fmt.Fprintf(writer, "Total Results: %d\n", len(results))
	fmt.Fprintf(writer, "Qualified Results: %d\n", len(rm.GetQualifiedResults()))
	fmt.Fprintf(writer, "Completion Rate: %.1f%%\n", float64(stats.Qualified)/float64(stats.Total)*100)
	fmt.Fprintf(writer, "\n")

	// Write table header
	fmt.Fprintf(writer, "%-15s %-8s %-12s %-12s %-12s %-20s\n",
		"IP", "Status", "Latency(ms)", "Speed(Mbps)", "Peak(Mbps)", "DataCenter")
	fmt.Fprintf(writer, "%s\n", strings.Repeat("-", 85))

	// Write results
	for _, result := range results {
		fmt.Fprintf(writer, "%-15s %-8s %-12s %-12s %-12.2f %-20s\n",
			result.IP,
			result.Status,
			result.Latency,
			result.Speed,
			result.PeakSpeed,
			result.DataCenter)
	}

	return nil
}

// Export exports results in the specified format
func (rm *ResultManager) Export(writer io.Writer, format ExportFormat, sortBy string, ascending bool) error {
	switch format {
	case FormatCSV:
		return rm.ExportToCSV(writer, sortBy, ascending)
	case FormatJSON:
		return rm.ExportToJSON(writer, sortBy, ascending)
	case FormatTXT:
		return rm.ExportToTXT(writer, sortBy, ascending)
	default:
		return fmt.Errorf("unsupported export format: %s", format)
	}
}

// GetMemoryUsage returns current memory usage information
func (rm *ResultManager) GetMemoryUsage() map[string]interface{} {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	return map[string]interface{}{
		"current_results": len(rm.results),
		"max_results":     rm.maxResults,
		"memory_usage":    fmt.Sprintf("%.1f%%", float64(len(rm.results))/float64(rm.maxResults)*100),
		"unique_ips":      len(rm.ipSet),
	}
}

// updateStats updates internal statistics (must be called with lock held)
func (rm *ResultManager) updateStats(result *models.SpeedTestResult) {
	rm.statsMu.Lock()
	defer rm.statsMu.Unlock()

	rm.stats.Completed++
	if result.Status == "已完成" {
		rm.stats.Qualified++
	}
}

// removeOldestResult removes the oldest result to manage memory (must be called with lock held)
func (rm *ResultManager) removeOldestResult() {
	if len(rm.results) == 0 {
		return
	}

	// Remove the first (oldest) result
	oldestResult := rm.results[0]
	rm.results = rm.results[1:]

	// Remove from IP set
	delete(rm.ipSet, oldestResult.IP)
}

// HasIP checks if an IP already exists in results
func (rm *ResultManager) HasIP(ip string) bool {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	return rm.ipSet[ip]
}

// GetResultCount returns the current number of results
func (rm *ResultManager) GetResultCount() int {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	return len(rm.results)
}

// GetQualifiedCount returns the number of qualified results
func (rm *ResultManager) GetQualifiedCount() int {
	return len(rm.GetQualifiedResults())
}
