package server

import (
	"net/http"
	"strconv"
	"time"
)

// getMetrics returns all metrics
func (s *Server) getMetrics(w http.ResponseWriter, r *http.Request) {
	allMetrics := s.metrics.GetAllMetrics()
	s.writeJSON(w, http.StatusOK, map[string]any{
		"metrics":   allMetrics,
		"timestamp": time.Now(),
	})
}

// getPerformanceStats returns performance statistics
func (s *Server) getPerformanceStats(w http.ResponseWriter, r *http.Request) {
	stats := s.metrics.GetPerformanceStats()
	s.writeJSON(w, http.StatusOK, stats)
}

// getSmoothedSpeed returns smoothed speed using sliding window
func (s *Server) getSmoothedSpeed(w http.ResponseWriter, r *http.Request) {
	smoothedSpeed := s.metrics.GetSmoothedSpeed()
	s.writeJSON(w, http.StatusOK, map[string]any{
		"smoothed_speed": smoothedSpeed,
		"timestamp":      time.Now(),
	})
}

// getSpeedSamples returns recent speed samples
func (s *Server) getSpeedSamples(w http.ResponseWriter, r *http.Request) {
	countStr := s.getQueryParam(r, "count", "20")
	count, err := strconv.Atoi(countStr)
	if err != nil || count <= 0 {
		count = 20
	}

	samples := s.metrics.GetRecentSamples(count)
	s.writeJSON(w, http.StatusOK, map[string]any{
		"samples":         samples,
		"count":           len(samples),
		"requested_count": count,
	})
}

// getErrorStats returns error statistics
func (s *Server) getErrorStats(w http.ResponseWriter, r *http.Request) {
	stats := s.errorHandler.GetErrorStats()
	s.writeJSON(w, http.StatusOK, map[string]any{
		"error_stats":   stats,
		"degraded_mode": s.errorHandler.IsInDegradedMode(),
	})
}
