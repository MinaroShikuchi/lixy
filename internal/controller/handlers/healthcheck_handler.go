package handlers

import (
	"encoding/json"
	"net/http"
)

type HealthCheckHandler struct {
	Version string
}

func NewHealthCheckHandler(version string) *HealthCheckHandler {
	return &HealthCheckHandler{
		Version: version,
	}
}

func (hh *HealthCheckHandler) HealthCheckHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	status := struct {
		Healthy bool   `json:"healthy"`
		Version string `json:"version"`
		Message string `json:"message,omitempty"`
	}{
		Healthy: true,
		Version: hh.Version,
	}

	w.Header().Set("Content-Type", "application/json")

	json.NewEncoder(w).Encode(status)
}
