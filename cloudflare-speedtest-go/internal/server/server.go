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
	"embed"
	"fmt"
	"html/template"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
)

// Server represents the web server
type Server struct {
	mux           *http.ServeMux
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
	templates     *template.Template
}

// New creates a new server instance
func New(cfg *yamlconfig.Config, dataDir string, configPath string, staticFS embed.FS) *Server {
	// Load HTML templates from embedded filesystem
	tmpl, err := loadTemplatesFromEmbed(staticFS)
	if err != nil {
		fmt.Printf("Error loading templates from embed: %v\n", err)
		panic(fmt.Sprintf("Failed to load templates: %v", err))
	}

	coloManager := colomanager.New(dataDir)

	// Create enhanced downloader with cache
	downloader := downloader.New()
	cacheDir := filepath.Join(dataDir, "cache")
	if err := downloader.SetCacheDir(cacheDir); err != nil {
		fmt.Printf("Warning: Failed to set cache directory: %v\n", err)
	}

	s := &Server{
		mux:           http.NewServeMux(),
		config:        cfg,
		resultManager: resultmanager.New(1000),
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
		templates:     tmpl,
	}

	s.setupRoutes()
	return s
}

// loadTemplatesFromEmbed loads HTML templates from embedded filesystem
func loadTemplatesFromEmbed(staticFS embed.FS) (*template.Template, error) {
	tmpl := template.New("")

	entries, err := staticFS.ReadDir("static")
	if err != nil {
		fmt.Printf("Error reading static dir: %v\n", err)
		return nil, err
	}

	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".html") {
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

// setupRoutes sets up all HTTP routes
func (s *Server) setupRoutes() {
	// API routes
	s.mux.HandleFunc("GET /api/config", s.getConfig)
	s.mux.HandleFunc("POST /api/config", s.updateConfig)
	s.mux.HandleFunc("POST /api/config/save", s.saveConfig)
	s.mux.HandleFunc("POST /api/config/validate", s.validateConfig)
	s.mux.HandleFunc("GET /api/datacenters", s.getDataCenters)
	s.mux.HandleFunc("POST /api/datacenters/filter", s.setDataCenterFilter)
	s.mux.HandleFunc("GET /api/results", s.getResults)
	s.mux.HandleFunc("GET /api/results/sorted", s.getSortedResults)
	s.mux.HandleFunc("GET /api/results/qualified", s.getQualifiedResults)
	s.mux.HandleFunc("GET /api/results/export/{format}", s.exportResults)
	s.mux.HandleFunc("GET /api/stats", s.getStats)
	s.mux.HandleFunc("GET /api/metrics", s.getMetrics)
	s.mux.HandleFunc("GET /api/metrics/performance", s.getPerformanceStats)
	s.mux.HandleFunc("GET /api/metrics/speed/smoothed", s.getSmoothedSpeed)
	s.mux.HandleFunc("GET /api/metrics/speed/samples", s.getSpeedSamples)
	s.mux.HandleFunc("GET /api/errors/stats", s.getErrorStats)
	s.mux.HandleFunc("POST /api/start", s.startTest)
	s.mux.HandleFunc("POST /api/stop", s.stopTest)
	s.mux.HandleFunc("DELETE /api/results", s.clearResults)
	s.mux.HandleFunc("POST /api/update", s.updateData)
	s.mux.HandleFunc("GET /api/status", s.getStatus)

	// HTML routes
	s.mux.HandleFunc("GET /", s.indexHandler)
}

// IsTesting returns whether a test is running
func (s *Server) IsTesting() bool {
	s.testMu.RLock()
	defer s.testMu.RUnlock()
	return s.testing
}

// Run starts the HTTP server
func (s *Server) Run(addr string) error {
	return http.ListenAndServe(addr, s.mux)
}
