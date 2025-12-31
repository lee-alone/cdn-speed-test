package server

import (
	"cloudflare-speedtest/internal/downloader"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"time"
)

// startTest starts the speed test
func (s *Server) startTest(w http.ResponseWriter, r *http.Request) {
	s.testMu.Lock()
	if s.testing {
		s.testMu.Unlock()
		s.writeError(w, http.StatusBadRequest, "test already running")
		return
	}
	s.testing = true
	s.testMu.Unlock()

	s.resultManager.Clear()
	s.metrics.Reset()
	s.metrics.RecordTestStart()

	go s.runTest()

	s.writeJSON(w, http.StatusOK, map[string]string{"message": "test started"})
}

// stopTest stops the speed test
func (s *Server) stopTest(w http.ResponseWriter, r *http.Request) {
	s.testMu.Lock()
	s.testing = false
	s.testMu.Unlock()

	if s.workerPool.IsRunning() {
		if err := s.workerPool.Stop(); err != nil {
			fmt.Printf("Error stopping worker pool: %v\n", err)
		}
	}

	s.writeJSON(w, http.StatusOK, map[string]string{"message": "test stopped"})
}

// getStatus returns the current server status
func (s *Server) getStatus(w http.ResponseWriter, r *http.Request) {
	s.testMu.RLock()
	testing := s.testing
	s.testMu.RUnlock()

	s.writeJSON(w, http.StatusOK, map[string]any{
		"testing":   testing,
		"timestamp": time.Now(),
	})
}

// updateData updates data files from upstream
func (s *Server) updateData(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Force bool `json:"force"`
	}

	body, _ := io.ReadAll(r.Body)
	fmt.Printf("Raw request body: %s\n", string(body))

	if err := json.Unmarshal(body, &req); err != nil {
		fmt.Printf("Warning: Failed to parse JSON body: %v\n", err)
	}

	fmt.Printf("Update request received - Force: %v\n", req.Force)

	files := downloader.GetFilesFromConfig(s.config.GetAllDownloadURLs())
	fmt.Printf("Total files to check: %d\n", len(files))

	var toDownload []downloader.FileInfo

	if req.Force {
		toDownload = files
		fmt.Printf("Force update requested: will download and overwrite %d files\n", len(files))
	} else {
		for _, file := range files {
			filePath := filepath.Join(s.dataDir, file.Name)
			exists := downloader.FileExists(filePath)
			fmt.Printf("Checking file %s: exists=%v\n", file.Name, exists)
			if !exists {
				toDownload = append(toDownload, file)
			}
		}
	}

	fmt.Printf("Files to download: %d\n", len(toDownload))

	if len(toDownload) == 0 {
		s.writeJSON(w, http.StatusOK, map[string]any{
			"message": "all files are up to date",
			"files":   0,
		})
		return
	}

	go func() {
		fmt.Printf("Starting download of %d files...\n", len(toDownload))
		if err := s.downloader.DownloadFiles(toDownload, s.dataDir); err != nil {
			fmt.Printf("Download error: %v\n", err)
		} else {
			fmt.Println("Download completed successfully")
		}
	}()

	s.writeJSON(w, http.StatusOK, map[string]any{
		"message": "download started",
		"files":   len(toDownload),
	})
}

