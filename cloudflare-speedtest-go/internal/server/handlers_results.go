package server

import (
	"cloudflare-speedtest/internal/resultmanager"
	"fmt"
	"net/http"
	"time"
)

// getResults returns all test results
func (s *Server) getResults(w http.ResponseWriter, r *http.Request) {
	results := s.resultManager.GetResults()
	s.writeJSON(w, http.StatusOK, results)
}

// getSortedResults returns sorted test results
func (s *Server) getSortedResults(w http.ResponseWriter, r *http.Request) {
	sortBy := s.getQueryParam(r, "sort", "speed")
	ascending := s.getQueryParam(r, "order", "desc") == "asc"

	results := s.resultManager.GetSortedResults(sortBy, ascending)
	s.writeJSON(w, http.StatusOK, map[string]any{
		"results":   results,
		"sort_by":   sortBy,
		"ascending": ascending,
		"count":     len(results),
	})
}

// getQualifiedResults returns only qualified/completed results
func (s *Server) getQualifiedResults(w http.ResponseWriter, r *http.Request) {
	results := s.resultManager.GetQualifiedResults()
	s.writeJSON(w, http.StatusOK, map[string]any{
		"results": results,
		"count":   len(results),
	})
}

// exportResults exports results in specified format
func (s *Server) exportResults(w http.ResponseWriter, r *http.Request) {
	format := r.PathValue("format")
	sortBy := s.getQueryParam(r, "sort", "speed")
	ascending := s.getQueryParam(r, "order", "desc") == "asc"

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
		s.writeError(w, http.StatusBadRequest, "Unsupported format. Use csv, json, or txt")
		return
	}

	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))

	if err := s.resultManager.Export(w, exportFormat, sortBy, ascending); err != nil {
		s.writeError(w, http.StatusInternalServerError, "Failed to export results: "+err.Error())
		return
	}
}

// getStats returns test statistics
func (s *Server) getStats(w http.ResponseWriter, r *http.Request) {
	stats := s.resultManager.GetStats()
	s.writeJSON(w, http.StatusOK, stats)
}

// clearResults clears all results
func (s *Server) clearResults(w http.ResponseWriter, r *http.Request) {
	s.resultManager.Clear()
	s.writeJSON(w, http.StatusOK, map[string]string{"message": "results cleared"})
}
