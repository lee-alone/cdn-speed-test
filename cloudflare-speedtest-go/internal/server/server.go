package server

import (
	"cloudflare-speedtest/internal/colomanager"
	"cloudflare-speedtest/internal/downloader"
	"cloudflare-speedtest/internal/tester"
	"cloudflare-speedtest/internal/urlmanager"
	"cloudflare-speedtest/internal/yamlconfig"
	"cloudflare-speedtest/pkg/models"
	"embed"
	"fmt"
	"html/template"
	"net/http"
	"path/filepath"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
)

// Server represents the web server
type Server struct {
	router      *gin.Engine
	config      *yamlconfig.Config
	results     []*models.SpeedTestResult
	mu          sync.RWMutex
	testing     bool
	testMu      sync.RWMutex
	downloader  *downloader.Downloader
	urlManager  *urlmanager.URLManager
	coloManager *colomanager.ColoManager
	speedTester *tester.SpeedTester
	ipReader    *tester.IPReader
	dataDir     string
	configPath  string
	staticFS    embed.FS
	testStats   *models.TestStats
	statsMu     sync.RWMutex
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

	s := &Server{
		router:      router,
		config:      cfg,
		results:     make([]*models.SpeedTestResult, 0),
		downloader:  downloader.New(),
		urlManager:  urlmanager.New(dataDir),
		coloManager: colomanager.New(dataDir),
		speedTester: tester.New(10),
		ipReader:    tester.NewIPReader(dataDir),
		dataDir:     dataDir,
		configPath:  configPath,
		staticFS:    staticFS,
		testStats: &models.TestStats{
			Total:        0,
			Completed:    0,
			Qualified:    0,
			CurrentIP:    "",
			CurrentSpeed: "",
		},
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
		api.GET("/results", s.getResults)
		api.GET("/stats", s.getStats)
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
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	s.config = &cfg

	c.JSON(http.StatusOK, gin.H{"message": "config updated in memory"})
}

// saveConfig saves the configuration to file
func (s *Server) saveConfig(c *gin.Context) {
	if err := yamlconfig.Save(s.configPath, s.config); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "config saved successfully"})
}

// getResults returns all test results
func (s *Server) getResults(c *gin.Context) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	results := make([]*models.SpeedTestResult, len(s.results))
	copy(results, s.results)
	c.JSON(http.StatusOK, results)
}

// getStats returns test statistics
func (s *Server) getStats(c *gin.Context) {
	s.statsMu.RLock()
	defer s.statsMu.RUnlock()

	stats := &models.TestStats{
		Total:        s.testStats.Total,
		Completed:    s.testStats.Completed,
		Qualified:    s.testStats.Qualified,
		CurrentIP:    s.testStats.CurrentIP,
		CurrentSpeed: s.testStats.CurrentSpeed,
	}

	c.JSON(http.StatusOK, stats)
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

	// Clear previous results
	s.mu.Lock()
	s.results = make([]*models.SpeedTestResult, 0)
	s.mu.Unlock()

	// Start test in background
	go s.runTest()

	c.JSON(http.StatusOK, gin.H{"message": "test started"})
}

// stopTest stops the speed test
func (s *Server) stopTest(c *gin.Context) {
	s.testMu.Lock()
	s.testing = false
	s.testMu.Unlock()

	c.JSON(http.StatusOK, gin.H{"message": "test stopped"})
}

// clearResults clears all results
func (s *Server) clearResults(c *gin.Context) {
	s.mu.Lock()
	s.results = make([]*models.SpeedTestResult, 0)
	s.mu.Unlock()

	c.JSON(http.StatusOK, gin.H{"message": "results cleared"})
}

// runTest runs the speed test
func (s *Server) runTest() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("Test panic: %v\n", r)
		}
		s.testMu.Lock()
		s.testing = false
		s.testMu.Unlock()
	}()

	// Load URL configuration
	if !s.urlManager.HasURLs() {
		if err := s.urlManager.LoadURLs(); err != nil {
			fmt.Printf("Failed to load URLs: %v\n", err)
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
	if strings.HasPrefix(url, "http://") {
		url = strings.TrimPrefix(url, "http://")
	} else if strings.HasPrefix(url, "https://") {
		url = strings.TrimPrefix(url, "https://")
	}

	parts := strings.SplitN(url, "/", 2)
	domain := parts[0]
	filePath := ""
	if len(parts) > 1 {
		filePath = parts[1]
	}

	fmt.Printf("Domain: %s, FilePath: %s\n", domain, filePath)

	// Configure speed tester
	s.speedTester.SetConfig(domain, filePath, float64(s.config.Test.DownloadTime))

	// Read IPs based on IP type
	fmt.Printf("Reading IPs from %s...\n", s.config.Test.IPType)
	ips, err := s.ipReader.ReadIPs(s.config.Test.IPType, 10)
	if err != nil {
		fmt.Printf("Failed to read IPs: %v\n", err)
		return
	}

	if len(ips) == 0 {
		fmt.Println("No IPs available for testing")
		return
	}

	fmt.Printf("Starting speed test with %d IPs\n", len(ips))

	// Initialize stats
	s.statsMu.Lock()
	s.testStats.Total = len(ips)
	s.testStats.Completed = 0
	s.testStats.Qualified = 0
	s.statsMu.Unlock()

	// Test each IP
	for idx, ip := range ips {
		// Check if testing should stop
		s.testMu.RLock()
		if !s.testing {
			s.testMu.RUnlock()
			fmt.Println("Test stopped by user")
			break
		}
		s.testMu.RUnlock()

		// Update current IP in stats
		s.statsMu.Lock()
		s.testStats.CurrentIP = ip
		s.statsMu.Unlock()

		fmt.Printf("[%d/%d] Testing IP: %s\n", idx+1, len(ips), ip)

		// Test the IP
		result := s.speedTester.TestSpeed(ip, s.config.Test.UseTLS, s.config.Test.Timeout, float64(s.config.Test.DownloadTime))

		// Store result
		s.mu.Lock()
		s.results = append(s.results, result)
		s.mu.Unlock()

		// Update stats
		s.statsMu.Lock()
		s.testStats.Completed++
		if result.Status == "已完成" {
			s.testStats.Qualified++
			s.testStats.CurrentSpeed = result.Speed
		}
		s.statsMu.Unlock()

		fmt.Printf("Tested IP: %s, Status: %s, Speed: %s Mbps, Latency: %s ms, DataCenter: %s\n",
			result.IP, result.Status, result.Speed, result.Latency, result.DataCenter)
	}

	fmt.Println("Speed test completed")
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
	// Get files from configuration
	files := downloader.GetFilesFromConfig(s.config.GetAllDownloadURLs())

	// Check for missing files
	var missing []downloader.FileInfo
	for _, file := range files {
		filePath := filepath.Join(s.dataDir, file.Name)
		if !downloader.FileExists(filePath) {
			missing = append(missing, file)
		}
	}

	if len(missing) == 0 {
		c.JSON(http.StatusOK, gin.H{
			"message": "all files are up to date",
			"files":   []string{},
		})
		return
	}

	// Download missing files
	go func() {
		fmt.Printf("Starting download of %d files...\n", len(missing))
		if err := s.downloader.DownloadFiles(missing, s.dataDir); err != nil {
			fmt.Printf("Download error: %v\n", err)
		} else {
			fmt.Println("Download completed successfully")
		}
	}()

	c.JSON(http.StatusOK, gin.H{
		"message": "download started",
		"files":   len(missing),
	})
}

// getStatus returns the current status
func (s *Server) getStatus(c *gin.Context) {
	s.testMu.RLock()
	testing := s.testing
	s.testMu.RUnlock()

	s.mu.RLock()
	resultCount := len(s.results)
	s.mu.RUnlock()

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
		"testing":      testing,
		"resultCount":  resultCount,
		"missingFiles": missingFiles,
		"dataDir":      s.dataDir,
		"urlCount":     s.urlManager.URLCount(),
		"coloCount":    s.coloManager.ColoCount(),
		"hasURLs":      s.urlManager.HasURLs(),
		"hasColos":     s.coloManager.HasColos(),
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
