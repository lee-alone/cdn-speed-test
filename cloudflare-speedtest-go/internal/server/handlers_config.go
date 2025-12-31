package server

import (
	"cloudflare-speedtest/internal/yamlconfig"
	"fmt"
	"net/http"
)

// getConfig returns the current configuration
func (s *Server) getConfig(w http.ResponseWriter, r *http.Request) {
	s.writeJSON(w, http.StatusOK, s.config)
}

// updateConfig updates the configuration in memory
func (s *Server) updateConfig(w http.ResponseWriter, r *http.Request) {
	var cfg yamlconfig.Config
	if err := s.readJSON(r, &cfg); err != nil {
		fmt.Printf("JSON binding error: %v\n", err)
		s.writeError(w, http.StatusBadRequest, "Invalid JSON format: "+err.Error())
		return
	}

	fmt.Printf("Received config: %+v\n", cfg)

	if err := cfg.Validate(); err != nil {
		fmt.Printf("Validation error: %v\n", err)
		s.writeError(w, http.StatusBadRequest, "Configuration validation failed: "+err.Error())
		return
	}

	s.config = &cfg
	fmt.Println("Configuration updated successfully in memory")

	s.writeJSON(w, http.StatusOK, map[string]string{"message": "config updated in memory"})
}

// saveConfig saves the configuration to file
func (s *Server) saveConfig(w http.ResponseWriter, r *http.Request) {
	fmt.Printf("Saving config to file: %s\n", s.configPath)
	fmt.Printf("Config to save: %+v\n", s.config)

	if err := yamlconfig.SaveWithValidation(s.configPath, s.config); err != nil {
		fmt.Printf("Save error: %v\n", err)
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	fmt.Println("Configuration saved successfully to file")
	s.writeJSON(w, http.StatusOK, map[string]string{"message": "config saved successfully"})
}

// validateConfig validates a configuration without saving it
func (s *Server) validateConfig(w http.ResponseWriter, r *http.Request) {
	var cfg yamlconfig.Config
	if err := s.readJSON(r, &cfg); err != nil {
		s.writeError(w, http.StatusBadRequest, "Invalid JSON format: "+err.Error())
		return
	}

	if err := cfg.Validate(); err != nil {
		s.writeJSON(w, http.StatusBadRequest, map[string]any{
			"valid": false,
			"error": err.Error(),
		})
		return
	}

	s.writeJSON(w, http.StatusOK, map[string]any{
		"valid":   true,
		"message": "Configuration is valid",
	})
}

// getDataCenters returns all available data centers
func (s *Server) getDataCenters(w http.ResponseWriter, r *http.Request) {
	if !s.coloManager.HasColos() {
		if err := s.coloManager.LoadColos(); err != nil {
			s.writeError(w, http.StatusInternalServerError, "Failed to load data centers: "+err.Error())
			return
		}
	}

	datacenters := s.coloManager.GetAvailableDataCenters()
	s.writeJSON(w, http.StatusOK, map[string]any{
		"datacenters": datacenters,
		"count":       len(datacenters),
		"filter_mode": s.coloManager.GetFilterMode(),
		"selected":    s.coloManager.GetSelectedDataCenters(),
	})
}

// setDataCenterFilter sets the data center filtering options
func (s *Server) setDataCenterFilter(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Mode     string   `json:"mode"`
		Selected []string `json:"selected"`
	}

	if err := s.readJSON(r, &request); err != nil {
		s.writeError(w, http.StatusBadRequest, "Invalid JSON format: "+err.Error())
		return
	}

	if request.Mode != "all" && request.Mode != "selected" {
		s.writeError(w, http.StatusBadRequest, "Invalid mode. Must be 'all' or 'selected'")
		return
	}

	s.coloManager.SetFilterMode(request.Mode)
	if request.Mode == "selected" {
		s.coloManager.SetSelectedDataCenters(request.Selected)
	}

	s.writeJSON(w, http.StatusOK, map[string]any{
		"message":        "Data center filter updated",
		"mode":           s.coloManager.GetFilterMode(),
		"selected":       s.coloManager.GetSelectedDataCenters(),
		"selected_count": len(s.coloManager.GetSelectedDataCenters()),
	})
}
