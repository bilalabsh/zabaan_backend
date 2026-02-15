package health

import (
	"encoding/json"
	"net/http"

	"github.com/bilalabsh/zabaan_backend/internal/database"
)

// HealthResponse is the JSON shape of /health.
type HealthResponse struct {
	Status   string `json:"status"`
	Message  string `json:"message"`
	Database string `json:"database"`
}

// Check returns server health status.
func Check(w http.ResponseWriter, r *http.Request) {
	dbStatus := "not configured"
	if database.DB != nil {
		if err := database.DB.Ping(); err != nil {
			dbStatus = "disconnected"
		} else {
			dbStatus = "connected"
		}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(HealthResponse{
		Status:   "ok",
		Message:  "Server is running",
		Database: dbStatus,
	})
}

// Root returns API info and links.
func Root(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"message": "Zabaan API",
		"health":  "/health",
	})
}

// NotFound returns JSON 404 for unknown routes.
func NotFound(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotFound)
	json.NewEncoder(w).Encode(map[string]string{
		"error": "not found",
		"path":  r.URL.Path,
	})
}
