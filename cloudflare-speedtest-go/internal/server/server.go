package server

import (
	"cloudflare-speedtest/internal/colomanager"
	"cloudflare-speedtest/internal/downloader"
	"cloudflare-speedtest/internal/errorhandler"
	"cloudflare-speedtest/internal/metrics"
	"cloudflare-speedtest/internal/resultmanager"
	"cloudflare-speedtest/internal/tester"
	"cloudflare-speedtest/internal/urlmanager"
	"cloudflare-speedtest/internal/workerpool"
	"cloudflare-speedtest/internal/yamlconfig"
	"cloudflare-speedtest/pkg/models"
	"embed"
	"fmt"
	"html/template"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// Server represents the web server
type Server struct {
	router        *gin.Engine
	config        *yamlconfig.Config
	resultManager *resultmanager.ResultManager
	errorHandler  *errorhandler.ErrorHandler
	metrics       *metrics.Metrics
	mu            sync.RWMutex
	testing       bool
	testMu        sync.RWMutex
	downloader    *downloader.Downloader
	urlManager    *urlmanager.URLManager
	coloManager   *colomanager.ColoManager
	speedTester   *tester.SpeedTester
	ipReader      *tester.IPReader
	workerPool    *workerpool.WorkerPool
	dataDir       string
	configPath    string
	staticFS      embed.FS
}

// New creates a new server instance
func New(cfg *yamlconfig.Config, dataDir string, configPath string, staticFS embed.FS) *Server {
	gin.SetMode(gin.ReleaseMode)
	router := gin.Default()

	// Load HTML templates from embedded filesystem
	tmpl, err := loadTemplatesFromEmbed(staticFS)
	if err != nil {
		fmt.Printf("Error loading templates from embed: %v\n", err)
		panic(fmt.Sprintf("Failed to load templates: %v", err))
	}
	router.SetHTMLTemplate(tmpl)

	coloManager := colomanager.New(dataDir)

	// Create enhanced downloader with cache
	downloader := downloader.New()
	cacheDir := filepath.Join(dataDir, "cache")
	if err := downloader.SetCacheDir(cacheDir); err != nil {
		fmt.Printf("Warning: Failed to set cache directory: %v\n", err)
	}

	s := &Server{
		router:        router,
		config:        cfg,
		resultManager: resultmanager.New(1000), // Keep max 1000 results in memory
		errorHandler:  errorhandler.New(),
		metrics:       metrics.New(),
		downloader:    downloader,
		urlManager:    urlmanager.New(dataDir),
		coloManager:   coloManager,
		speedTester:   tester.New(10),
		ipReader:      tester.NewIPReader(dataDir),
		workerPool:    workerpool.New(cfg.Advanced.ConcurrentWorkers, coloManager),
		dataDir:       dataDir,
		configPath:    configPath,
		staticFS:      staticFS,
	}

	s.setupRoutes()
	return s
}

// setupRoutes sets up all routes
func (s *Server) setupRoutes() {
	// API routes
	api := s.router.Group("/api")
	{
		api.GET("/config", s.getConfig)
		api.POST("/config", s.updateConfig)
		api.POST("/config/save", s.saveConfig)
		api.POST("/config/validate", s.validateConfig)
		api.GET("/datacenters", s.getDataCenters)
		api.POST("/datacenters/filter", s.setDataCenterFilter)
		api.GET("/results", s.getResults)
		api.GET("/results/sorted", s.getSortedResults)
		api.GET("/results/qualified", s.getQualifiedResults)
		api.GET("/results/export/:format", s.exportResults)
		api.GET("/stats", s.getStats)
		api.GET("/metrics", s.getMetrics)
		api.GET("/metrics/performance", s.getPerformanceStats)
		api.GET("/metrics/speed/smoothed", s.getSmoothedSpeed)
		api.GET("/metrics/speed/samples", s.getSpeedSamples)
		api.GET("/errors/stats", s.getErrorStats)
		api.POST("/start", s.startTest)
		api.POST("/stop", s.stopTest)
		api.DELETE("/results", s.clearResults)
		api.POST("/update", s.updateData)
		api.GET("/status", s.getStatus)
	}

	// HTML routes
	s.router.GET("/", s.indexHandler)
}

// indexHandler serves the main HTML page
func (s *Server) indexHandler(c *gin.Context) {
	c.HTML(http.StatusOK, "index.html", nil)
}

// getConfig returns the current configuration
func (s *Server) getConfig(c *gin.Context) {
	c.JSON(http.StatusOK, s.config)
}

// updateConfig updates the configuration in memory
func (s *Server) updateConfig(c *gin.Context) {
	var cfg yamlconfig.Config
	if err := c.ShouldBindJSON(&cfg); err != nil {
		fmt.Printf("JSON binding error: %v\n", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON format: " + err.Error()})
		return
	}

	fmt.Printf("Received config: %+v\n", cfg)

	// Validate the configuration
	if err := cfg.Validate(); err != nil {
		fmt.Printf("Validation error: %v\n", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Configuration validation failed: " + err.Error()})
		return
	}

	s.config = &cfg
	fmt.Println("Configuration updated successfully in memory")

	c.JSON(http.StatusOK, gin.H{"message": "config updated in memory"})
}

// saveConfig saves the configuration to file
func (s *Server) saveConfig(c *gin.Context) {
	fmt.Printf("Saving config to file: %s\n", s.configPath)
	fmt.Printf("Config to save: %+v\n", s.config)

	if err := yamlconfig.SaveWithValidation(s.configPath, s.config); err != nil {
		fmt.Printf("Save error: %v\n", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	fmt.Println("Configuration saved successfully to file")
	c.JSON(http.StatusOK, gin.H{"message": "config saved successfully"})
}

// validateConfig validates a configuration without saving it
func (s *Server) validateConfig(c *gin.Context) {
	var cfg yamlconfig.Config
	if err := c.ShouldBindJSON(&cfg); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON format: " + err.Error()})
		return
	}

	if err := cfg.Validate(); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"valid": false,
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"valid":   true,
		"message": "Configuration is valid",
	})
}

// getDataCenters returns all available data centers
func (s *Server) getDataCenters(c *gin.Context) {
	// Ensure data centers are loaded
	if !s.coloManager.HasColos() {
		if err := s.coloManager.LoadColos(); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load data centers: " + err.Error()})
			return
		}
	}

	datacenters := s.coloManager.GetAvailableDataCenters()
	c.JSON(http.StatusOK, gin.H{
		"datacenters": datacenters,
		"count":       len(datacenters),
		"filter_mode": s.coloManager.GetFilterMode(),
		"selected":    s.coloManager.GetSelectedDataCenters(),
	})
}

// getDataCentersByRegion returns data centers grouped by region
func (s *Server) getDataCentersByRegion(c *gin.Context) {
	// Ensure data centers are loaded
	if !s.coloManager.HasColos() {
		if err := s.coloManager.LoadColos(); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load data centers: " + err.Error()})
			return
		}
	}

	regions := s.coloManager.GetDataCentersByRegion()
	c.JSON(http.StatusOK, gin.H{
		"regions": regions,
		"total":   s.coloManager.ColoCount(),
	})
}

// setDataCenterFilter sets the data center filtering options
func (s *Server) setDataCenterFilter(c *gin.Context) {
	var request struct {
		Mode     string   `json:"mode"`     // "all" or "selected"
		Selected []string `json:"selected"` // list of selected data center codes
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON format: " + err.Error()})
		return
	}

	// Validate mode
	if request.Mode != "all" && request.Mode != "selected" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid mode. Must be 'all' or 'selected'"})
		return
	}

	// Set filter mode and selected data centers
	s.coloManager.SetFilterMode(request.Mode)
	if request.Mode == "selected" {
		s.coloManager.SetSelectedDataCenters(request.Selected)
	}

	c.JSON(http.StatusOK, gin.H{
		"message":        "Data center filter updated",
		"mode":           s.coloManager.GetFilterMode(),
		"selected":       s.coloManager.GetSelectedDataCenters(),
		"selected_count": len(s.coloManager.GetSelectedDataCenters()),
	})
}

// getWorkerPoolStats returns worker pool statistics
func (s *Server) getWorkerPoolStats(c *gin.Context) {
	stats := s.workerPool.GetStats()
	c.JSON(http.StatusOK, stats)
}

// getStats returns test statistics
func (s *Server) getStats(c *gin.Context) {
	stats := s.resultManager.GetStats()
	c.JSON(http.StatusOK, stats)
}

// getMetrics returns all metrics
func (s *Server) getMetrics(c *gin.Context) {
	allMetrics := s.metrics.GetAllMetrics()
	c.JSON(http.StatusOK, gin.H{
		"metrics":   allMetrics,
		"timestamp": time.Now(),
	})
}

// getPerformanceStats returns performance statistics
func (s *Server) getPerformanceStats(c *gin.Context) {
	stats := s.metrics.GetPerformanceStats()
	c.JSON(http.StatusOK, stats)
}

// getSmoothedSpeed returns smoothed speed using sliding window
func (s *Server) getSmoothedSpeed(c *gin.Context) {
	smoothedSpeed := s.metrics.GetSmoothedSpeed()
	c.JSON(http.StatusOK, gin.H{
		"smoothed_speed": smoothedSpeed,
		"timestamp":      time.Now(),
	})
}

// getSpeedSamples returns recent speed samples
func (s *Server) getSpeedSamples(c *gin.Context) {
	countStr := c.DefaultQuery("count", "20")
	count, err := strconv.Atoi(countStr)
	if err != nil || count <= 0 {
		count = 20
	}

	samples := s.metrics.GetRecentSamples(count)
	c.JSON(http.StatusOK, gin.H{
		"samples":         samples,
		"count":           len(samples),
		"requested_count": count,
	})
}

// resetMetrics resets all metrics
func (s *Server) resetMetrics(c *gin.Context) {
	s.metrics.Reset()
	c.JSON(http.StatusOK, gin.H{
		"message":   "Metrics reset successfully",
		"timestamp": time.Now(),
	})
}

// getDownloadProgress returns download progress information
func (s *Server) getDownloadProgress(c *gin.Context) {
	// This would typically be implemented with WebSocket for real-time updates
	// For now, we'll return a simple status
	c.JSON(http.StatusOK, gin.H{
		"message":  "Download progress tracking available via WebSocket",
		"endpoint": "/ws/download-progress",
		"note":     "Use the progress channel from downloader for real-time updates",
	})
}

// getResults returns all test results
func (s *Server) getResults(c *gin.Context) {
	results := s.resultManager.GetResults()
	c.JSON(http.StatusOK, results)
}

// getSortedResults returns sorted test results
func (s *Server) getSortedResults(c *gin.Context) {
	sortBy := c.DefaultQuery("sort", "speed")
	ascending := c.DefaultQuery("order", "desc") == "asc"

	results := s.resultManager.GetSortedResults(sortBy, ascending)
	c.JSON(http.StatusOK, gin.H{
		"results":   results,
		"sort_by":   sortBy,
		"ascending": ascending,
		"count":     len(results),
	})
}

// getQualifiedResults returns only qualified/completed results
func (s *Server) getQualifiedResults(c *gin.Context) {
	results := s.resultManager.GetQualifiedResults()
	c.JSON(http.StatusOK, gin.H{
		"results": results,
		"count":   len(results),
	})
}

// exportResults exports results in specified format
func (s *Server) exportResults(c *gin.Context) {
	format := c.Param("format")
	sortBy := c.DefaultQuery("sort", "speed")
	ascending := c.DefaultQuery("order", "desc") == "asc"

	var exportFormat resultmanager.ExportFormat
	var contentType string
	var filename string

	switch format {
	case "csv":
		exportFormat = resultmanager.FormatCSV
		contentType = "text/csv"
		filename = fmt.Sprintf("cloudflare-speedtest-%s.csv", time.Now().Format("20060102-150405"))
	case "json":
		exportFormat = resultmanager.FormatJSON
		contentType = "application/json"
		filename = fmt.Sprintf("cloudflare-speedtest-%s.json", time.Now().Format("20060102-150405"))
	case "txt":
		exportFormat = resultmanager.FormatTXT
		contentType = "text/plain"
		filename = fmt.Sprintf("cloudflare-speedtest-%s.txt", time.Now().Format("20060102-150405"))
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "Unsupported format. Use csv, json, or txt"})
		return
	}

	c.Header("Content-Type", contentType)
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))

	if err := s.resultManager.Export(c.Writer, exportFormat, sortBy, ascending); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to export results: " + err.Error()})
		return
	}
}

// getMemoryUsage returns memory usage information
func (s *Server) getMemoryUsage(c *gin.Context) {
	usage := s.resultManager.GetMemoryUsage()
	c.JSON(http.StatusOK, usage)
}

// getErrorStats returns error statistics
func (s *Server) getErrorStats(c *gin.Context) {
	stats := s.errorHandler.GetErrorStats()
	c.JSON(http.StatusOK, gin.H{
		"error_stats":   stats,
		"degraded_mode": s.errorHandler.IsInDegradedMode(),
	})
}

// getRetryPolicies returns retry policies for all error types
func (s *Server) getRetryPolicies(c *gin.Context) {
	policies := make(map[string]*errorhandler.RetryPolicy)

	errorTypes := []errorhandler.ErrorType{
		errorhandler.ErrorTypeNetwork,
		errorhandler.ErrorTypeTimeout,
		errorhandler.ErrorTypeValidation,
		errorhandler.ErrorTypeSystem,
		errorhandler.ErrorTypeDataCenter,
		errorhandler.ErrorTypeSpeedTest,
		errorhandler.ErrorTypeConfig,
		errorhandler.ErrorTypeFileIO,
	}

	for _, errorType := range errorTypes {
		if policy := s.errorHandler.GetRetryPolicy(errorType); policy != nil {
			policies[string(errorType)] = policy
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"retry_policies": policies,
	})
}

// enableDegradedMode enables degraded mode
func (s *Server) enableDegradedMode(c *gin.Context) {
	var request struct {
		DurationMinutes int `json:"duration_minutes"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON format: " + err.Error()})
		return
	}

	if request.DurationMinutes <= 0 {
		request.DurationMinutes = 30 // Default to 30 minutes
	}

	duration := time.Duration(request.DurationMinutes) * time.Minute
	s.errorHandler.EnableDegradedMode(duration)

	c.JSON(http.StatusOK, gin.H{
		"message":          "Degraded mode enabled",
		"duration_minutes": request.DurationMinutes,
	})
}

// disableDegradedMode disables degraded mode
func (s *Server) disableDegradedMode(c *gin.Context) {
	s.errorHandler.DisableDegradedMode()
	c.JSON(http.StatusOK, gin.H{
		"message": "Degraded mode disabled",
	})
}

// startTest starts the speed test
func (s *Server) startTest(c *gin.Context) {
	s.testMu.Lock()
	if s.testing {
		s.testMu.Unlock()
		c.JSON(http.StatusBadRequest, gin.H{"error": "test already running"})
		return
	}
	s.testing = true
	s.testMu.Unlock()

	// Clear previous results and reset metrics
	s.resultManager.Clear()
	s.metrics.Reset()

	// Record test start
	s.metrics.RecordTestStart()

	// Start test in background
	go s.runTest()

	c.JSON(http.StatusOK, gin.H{"message": "test started"})
}

// stopTest stops the speed test
func (s *Server) stopTest(c *gin.Context) {
	s.testMu.Lock()
	s.testing = false
	s.testMu.Unlock()

	// Stop worker pool if it's running
	if s.workerPool.IsRunning() {
		if err := s.workerPool.Stop(); err != nil {
			fmt.Printf("Error stopping worker pool: %v\n", err)
		}
	}

	c.JSON(http.StatusOK, gin.H{"message": "test stopped"})
}

// clearResults clears all results
func (s *Server) clearResults(c *gin.Context) {
	s.resultManager.Clear()
	c.JSON(http.StatusOK, gin.H{"message": "results cleared"})
}

// runTest runs the two-phase speed test: concurrent datacenter detection + serial speed testing
func (s *Server) runTest() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("Test panic: %v\n", r)
		}
		s.testMu.Lock()
		s.testing = false
		s.testMu.Unlock()

		// Stop worker pool if it's running
		if s.workerPool.IsRunning() {
			s.workerPool.Stop()
		}

		fmt.Println("Test execution completed, testing flag set to false")
	}()

	fmt.Println("Starting runTest function")

	// Load URL configuration
	if !s.urlManager.HasURLs() {
		fmt.Println("Loading URLs...")
		if err := s.urlManager.LoadURLs(); err != nil {
			fmt.Printf("Failed to load URLs: %v\n", err)
			return
		}
	}

	// Load data center information
	if !s.coloManager.HasColos() {
		fmt.Println("Loading data centers...")
		if err := s.coloManager.LoadColos(); err != nil {
			fmt.Printf("Failed to load data centers: %v\n", err)
			return
		}
	}

	urls := s.urlManager.GetURLs()
	if len(urls) == 0 {
		fmt.Println("No URLs available for testing")
		return
	}

	// Parse URL to get domain and file path
	url := urls[0]
	fmt.Printf("Using URL: %s\n", url)

	// Remove protocol if present
	url, _ = strings.CutPrefix(url, "http://")
	url, _ = strings.CutPrefix(url, "https://")

	parts := strings.SplitN(url, "/", 2)
	domain := parts[0]
	filePath := ""
	if len(parts) > 1 {
		filePath = parts[1]
	}

	fmt.Printf("Domain: %s, FilePath: %s\n", domain, filePath)

	// Read IPs based on IP type
	fmt.Printf("Reading IPs from %s...\n", s.config.Test.IPType)
	ips, err := s.ipReader.ReadIPs(s.config.Test.IPType, 100) // Read more IPs for two-phase testing
	if err != nil {
		fmt.Printf("Failed to read IPs: %v\n", err)
		return
	}

	if len(ips) == 0 {
		fmt.Println("No IPs available for testing")
		return
	}

	fmt.Printf("Starting two-phase speed test with %d IPs\n", len(ips))
	fmt.Printf("Phase 1: Concurrent datacenter detection using %d workers\n", s.config.Advanced.ConcurrentWorkers)
	fmt.Printf("Phase 2: Serial speed testing (to avoid bandwidth interference)\n")
	fmt.Printf("Data center filter mode: %s\n", s.coloManager.GetFilterMode())
	if s.coloManager.GetFilterMode() == "selected" {
		fmt.Printf("Selected data centers: %v\n", s.coloManager.GetSelectedDataCenters())
	}

	// Initialize stats
	s.resultManager.SetTotal(len(ips))

	// PHASE 1: Concurrent datacenter detection
	fmt.Println("\n=== PHASE 1: Concurrent Datacenter Detection ===")
	validIPs := s.runDataCenterPhase(ips, domain, filePath)

	// If no valid IPs found, try to read more IPs and retry
	if len(validIPs) == 0 {
		fmt.Println("No valid IPs found in first batch, attempting to read more IPs...")

		// Determine max attempts: -1 means unlimited, otherwise use configured value
		maxAttempts := s.config.Advanced.RetryAttempts
		if maxAttempts < 0 {
			maxAttempts = 1000 // Use a large number to represent "unlimited"
		}

		// Try to read more IPs
		for attempt := 1; attempt <= maxAttempts && len(validIPs) == 0; attempt++ {
			// Check if testing should stop
			s.testMu.RLock()
			if !s.testing {
				s.testMu.RUnlock()
				fmt.Println("Testing stopped by user")
				return
			}
			s.testMu.RUnlock()

			// Show warning message after 3 attempts
			if attempt >= 3 {
				fmt.Printf("Retry attempt %d: 该数据中心IP较少，需要较长时间，请耐心等待...\n", attempt)
			} else {
				fmt.Printf("Retry attempt %d: Reading more IPs...\n", attempt)
			}

			moreIPs, err := s.ipReader.ReadIPs(s.config.Test.IPType, 100)
			if err != nil {
				fmt.Printf("Failed to read more IPs: %v\n", err)
				break
			}

			if len(moreIPs) == 0 {
				fmt.Println("No more IPs available")
				break
			}

			fmt.Printf("Testing %d more IPs\n", len(moreIPs))
			validIPs = s.runDataCenterPhase(moreIPs, domain, filePath)
		}
	}

	if len(validIPs) == 0 {
		fmt.Println("No valid IPs found after all attempts")
		fmt.Println("This might be due to:")
		fmt.Println("1. Network connectivity issues")
		fmt.Println("2. Strict datacenter filtering")
		fmt.Println("3. All IPs failed datacenter detection")
		return
	}

	fmt.Printf("Phase 1 completed: %d valid IPs found\n", len(validIPs))

	// PHASE 2: Serial speed testing
	fmt.Println("\n=== PHASE 2: Serial Speed Testing ===")
	s.runSpeedTestPhase(validIPs, domain, filePath)

	fmt.Println("Two-phase speed test completed")
}

// runDataCenterPhase runs the concurrent datacenter detection phase
func (s *Server) runDataCenterPhase(ips []string, domain, filePath string) []string {
	fmt.Printf("Starting datacenter detection for %d IPs using %d workers\n", len(ips), s.config.Advanced.ConcurrentWorkers)

	// Create enhanced speed tester for datacenter detection
	enhancedTester := tester.NewEnhanced(s.config.Test.Timeout)
	enhancedTester.SetConfig(domain, filePath, float64(s.config.Test.DownloadTime))

	// Channel to collect datacenter results
	type DataCenterResult struct {
		IP         string
		DataCenter string
		Latency    float64
		Error      error
	}

	resultChan := make(chan DataCenterResult, len(ips))
	semaphore := make(chan struct{}, s.config.Advanced.ConcurrentWorkers)

	// Start concurrent datacenter detection
	var wg sync.WaitGroup
	for _, ip := range ips {
		// Check if testing should stop
		s.testMu.RLock()
		if !s.testing {
			s.testMu.RUnlock()
			break
		}
		s.testMu.RUnlock()

		wg.Add(1)
		go func(testIP string) {
			defer wg.Done()

			// Acquire semaphore
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			fmt.Printf("Testing datacenter for IP: %s\n", testIP)
			datacenter, latency, err := enhancedTester.TestDataCenterOnly(testIP, s.config.Test.UseTLS, s.config.Test.Timeout)

			resultChan <- DataCenterResult{
				IP:         testIP,
				DataCenter: datacenter,
				Latency:    latency,
				Error:      err,
			}
		}(ip)
	}

	// Wait for all datacenter tests to complete
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	// Collect results and filter valid IPs
	validIPs := make([]string, 0)
	testedCount := 0
	filteredCount := 0

	for result := range resultChan {
		testedCount++

		if result.Error != nil {
			fmt.Printf("Datacenter test failed for %s: %v\n", result.IP, result.Error)
			continue
		}

		if result.DataCenter == "" {
			fmt.Printf("No datacenter info found for %s\n", result.IP)
			continue
		}

		// Apply datacenter filtering
		if !s.coloManager.FilterByDataCenter(result.DataCenter) {
			fmt.Printf("IP %s filtered out (datacenter: %s not in selected list)\n", result.IP, result.DataCenter)
			filteredCount++
			continue
		}

		fmt.Printf("Valid IP found: %s (datacenter: %s, latency: %.2f ms)\n", result.IP, result.DataCenter, result.Latency)
		validIPs = append(validIPs, result.IP)
	}

	fmt.Printf("Datacenter phase summary: Tested=%d, Filtered=%d, Valid=%d\n", testedCount, filteredCount, len(validIPs))

	// If no valid IPs found due to filtering, log warning
	if len(validIPs) == 0 && filteredCount > 0 {
		fmt.Printf("WARNING: All %d IPs were filtered out due to datacenter selection. No IPs match the selected datacenters.\n", filteredCount)
	}

	return validIPs
}

// runSpeedTestPhase runs the serial speed testing phase
func (s *Server) runSpeedTestPhase(validIPs []string, domain, filePath string) {
	fmt.Printf("Starting serial speed testing for %d valid IPs\n", len(validIPs))

	// Create enhanced speed tester for speed testing
	enhancedTester := tester.NewEnhanced(s.config.Test.Timeout)
	enhancedTester.SetConfig(domain, filePath, float64(s.config.Test.DownloadTime))

	// Test each IP serially to avoid bandwidth interference
	for i, ip := range validIPs {
		// Check if testing should stop
		s.testMu.RLock()
		if !s.testing {
			s.testMu.RUnlock()
			fmt.Println("Speed testing stopped by user")
			break
		}
		s.testMu.RUnlock()

		fmt.Printf("Speed testing IP %d/%d: %s\n", i+1, len(validIPs), ip)

		// Update current IP in stats
		s.resultManager.UpdateCurrentTest(ip, "")

		// First get datacenter info again (for the final result)
		datacenter, latency, err := enhancedTester.TestDataCenterOnly(ip, s.config.Test.UseTLS, s.config.Test.Timeout)
		if err != nil {
			fmt.Printf("Failed to get datacenter info for %s: %v\n", ip, err)
			continue
		}

		// Then test speed
		speedResult, err := enhancedTester.TestSpeedOnly(ip, s.config.Test.UseTLS, s.config.Test.Timeout, float64(s.config.Test.DownloadTime))
		if err != nil {
			fmt.Printf("Speed test failed for %s: %v\n", ip, err)

			// Create failed result
			result := &models.SpeedTestResult{
				IP:         ip,
				Status:     "无效",
				Latency:    fmt.Sprintf("%.2f", latency),
				Speed:      "timeout",
				DataCenter: s.coloManager.GetFriendlyName(datacenter),
				PeakSpeed:  0,
			}

			s.storeResult(result)
			continue
		}

		// Create successful result
		result := &models.SpeedTestResult{
			IP:         ip,
			Status:     speedResult.Status,
			Latency:    fmt.Sprintf("%.2f", latency),
			Speed:      speedResult.Speed,
			DataCenter: s.coloManager.GetFriendlyName(datacenter),
			PeakSpeed:  speedResult.PeakSpeed,
		}

		s.storeResult(result)

		// Update stats
		s.resultManager.UpdateCurrentTest(result.IP, result.Speed)

		fmt.Printf("Speed test completed for %s: Status=%s, Speed=%s Mbps, Latency=%s ms, DataCenter=%s\n",
			ip, result.Status, result.Speed, result.Latency, result.DataCenter)

		// Small delay between tests to avoid overwhelming the server
		time.Sleep(100 * time.Millisecond)

		// Check if we found enough qualified servers (with bandwidth requirement)
		qualifiedResults := s.resultManager.GetQualifiedResults()
		qualifiedCount := 0
		expectedBandwidth := s.config.Test.Bandwidth

		for _, r := range qualifiedResults {
			// Speed is string like "123.45", ignore errors as they should be valid floats
			speedVal, err := strconv.ParseFloat(r.Speed, 64)
			if err == nil && speedVal >= expectedBandwidth {
				qualifiedCount++
			}
		}

		fmt.Printf("Current progress: %d servers with speed >= %.2f Mbps (need %d)\n",
			qualifiedCount, expectedBandwidth, s.config.Test.ExpectedServers)

		if qualifiedCount >= s.config.Test.ExpectedServers {
			fmt.Printf("\nFound %d qualified servers (speed >= %.2f Mbps). Expected: %d. Stopping test.\n",
				qualifiedCount, expectedBandwidth, s.config.Test.ExpectedServers)

			// Update total to match completed so frontend knows we are done
			stats := s.resultManager.GetStats()
			s.resultManager.SetTotal(stats.Completed)

			break
		}
	}

	// Clear current IP status
	s.resultManager.UpdateCurrentTest("", "")
}

// storeResult stores a test result using ResultManager and updates metrics
func (s *Server) storeResult(result *models.SpeedTestResult) {
	// Try to add result, allowing duplicates for updates
	s.resultManager.AddResultAllowDuplicate(result)

	// Update current speed if this is a successful result
	if result.Status == "已完成" {
		s.resultManager.UpdateCurrentTest(result.IP, result.Speed)

		// Record metrics
		speed, _ := strconv.ParseFloat(result.Speed, 64)
		latency, _ := strconv.ParseFloat(result.Latency, 64)

		s.metrics.RecordSpeedSample(speed, 0, 0) // We don't have bytes/duration here
		s.metrics.RecordLatencySample(latency)
		s.metrics.RecordTestComplete(true)

		// Record counters
		s.metrics.RecordCounter("tests.successful", 1, map[string]string{
			"datacenter": result.DataCenter,
		})
	} else {
		s.metrics.RecordTestComplete(false)
		s.metrics.RecordCounter("tests.failed", 1, map[string]string{
			"status": result.Status,
		})
	}
}

// IsTesting returns whether a test is running
func (s *Server) IsTesting() bool {
	s.testMu.RLock()
	defer s.testMu.RUnlock()
	return s.testing
}

// Run starts the server
func (s *Server) Run(addr string) error {
	return s.router.Run(addr)
}

// updateData updates data files from upstream
func (s *Server) updateData(c *gin.Context) {
	// Parse request body to check for force flag
	var req struct {
		Force bool `json:"force"`
	}
	c.ShouldBindJSON(&req)

	// Get files from configuration
	files := downloader.GetFilesFromConfig(s.config.GetAllDownloadURLs())

	var toDownload []downloader.FileInfo

	if req.Force {
		// Force update: delete existing files and download all files
		for _, file := range files {
			filePath := filepath.Join(s.dataDir, file.Name)
			// Delete existing file to force re-download
			if downloader.FileExists(filePath) {
				if err := os.Remove(filePath); err != nil {
					fmt.Printf("Warning: failed to delete %s: %v\n", filePath, err)
				}
			}
		}
		toDownload = files
	} else {
		// Normal update: only download missing files
		for _, file := range files {
			filePath := filepath.Join(s.dataDir, file.Name)
			if !downloader.FileExists(filePath) {
				toDownload = append(toDownload, file)
			}
		}
	}

	if len(toDownload) == 0 {
		c.JSON(http.StatusOK, gin.H{
			"message": "all files are up to date",
			"files":   0,
		})
		return
	}

	// Download files
	go func() {
		fmt.Printf("Starting download of %d files...\n", len(toDownload))
		if err := s.downloader.DownloadFiles(toDownload, s.dataDir); err != nil {
			fmt.Printf("Download error: %v\n", err)
		} else {
			fmt.Println("Download completed successfully")
		}
	}()

	c.JSON(http.StatusOK, gin.H{
		"message": "download started",
		"files":   len(toDownload),
	})
}

// getStatus returns the current status
func (s *Server) getStatus(c *gin.Context) {
	s.testMu.RLock()
	testing := s.testing
	s.testMu.RUnlock()

	resultCount := s.resultManager.GetResultCount()

	// Check for data files
	files := downloader.GetFilesFromConfig(s.config.GetAllDownloadURLs())
	var missingFiles []string
	for _, file := range files {
		filePath := filepath.Join(s.dataDir, file.Name)
		if !downloader.FileExists(filePath) {
			missingFiles = append(missingFiles, file.Name)
		}
	}

	// Load managers if not already loaded
	if !s.urlManager.HasURLs() {
		s.urlManager.LoadURLs()
	}
	if !s.coloManager.HasColos() {
		s.coloManager.LoadColos()
	}

	c.JSON(http.StatusOK, gin.H{
		"testing":        testing,
		"resultCount":    resultCount,
		"qualifiedCount": s.resultManager.GetQualifiedCount(),
		"missingFiles":   missingFiles,
		"dataDir":        s.dataDir,
		"urlCount":       s.urlManager.URLCount(),
		"coloCount":      s.coloManager.ColoCount(),
		"hasURLs":        s.urlManager.HasURLs(),
		"hasColos":       s.coloManager.HasColos(),
		"memoryUsage":    s.resultManager.GetMemoryUsage(),
	})
}

// getDebugInfo returns detailed debug information
func (s *Server) getDebugInfo(c *gin.Context) {
	s.testMu.RLock()
	testing := s.testing
	s.testMu.RUnlock()

	// Get current stats
	stats := s.resultManager.GetStats()

	// Get worker pool stats
	workerStats := s.workerPool.GetStats()

	// Get recent results (last 10)
	allResults := s.resultManager.GetResults()
	recentResults := allResults
	if len(allResults) > 10 {
		recentResults = allResults[len(allResults)-10:]
	}

	c.JSON(http.StatusOK, gin.H{
		"timestamp":      time.Now(),
		"testing":        testing,
		"stats":          stats,
		"worker_stats":   workerStats,
		"recent_results": recentResults,
		"config": gin.H{
			"ip_type":            s.config.Test.IPType,
			"concurrent_workers": s.config.Advanced.ConcurrentWorkers,
			"timeout":            s.config.Test.Timeout,
			"download_time":      s.config.Test.DownloadTime,
			"use_tls":            s.config.Test.UseTLS,
		},
		"managers": gin.H{
			"has_urls":       s.urlManager.HasURLs(),
			"url_count":      s.urlManager.URLCount(),
			"has_colos":      s.coloManager.HasColos(),
			"colo_count":     s.coloManager.ColoCount(),
			"filter_mode":    s.coloManager.GetFilterMode(),
			"selected_colos": s.coloManager.GetSelectedDataCenters(),
		},
	})
}

// loadTemplatesFromEmbed loads templates from embedded filesystem
func loadTemplatesFromEmbed(staticFS embed.FS) (*template.Template, error) {
	tmpl := template.New("")

	// Read static directory
	entries, err := staticFS.ReadDir("static")
	if err != nil {
		fmt.Printf("Error reading static dir: %v\n", err)
		return nil, err
	}

	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".html") {
			// Use forward slash for embed.FS (always uses Unix-style paths)
			filePath := "static/" + entry.Name()
			data, err := staticFS.ReadFile(filePath)
			if err != nil {
				fmt.Printf("Error reading file %s: %v\n", filePath, err)
				return nil, err
			}

			_, err = tmpl.New(entry.Name()).Parse(string(data))
			if err != nil {
				fmt.Printf("Error parsing template %s: %v\n", entry.Name(), err)
				return nil, err
			}
		}
	}

	return tmpl, nil
}
