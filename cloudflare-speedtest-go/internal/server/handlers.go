package server

import (
	"encoding/json"
	"net/http"
)

// Helper functions for HTTP response handling

func (s *Server) writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func (s *Server) writeError(w http.ResponseWriter, status int, message string) {
	s.writeJSON(w, status, map[string]string{"error": message})
}

func (s *Server) readJSON(r *http.Request, v any) error {
	defer r.Body.Close()
	return json.NewDecoder(r.Body).Decode(v)
}

func (s *Server) getQueryParam(r *http.Request, key, defaultValue string) string {
	if val := r.URL.Query().Get(key); val != "" {
		return val
	}
	return defaultValue
}

// indexHandler serves the main HTML page
func (s *Server) indexHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	s.templates.ExecuteTemplate(w, "index.html", nil)
}
