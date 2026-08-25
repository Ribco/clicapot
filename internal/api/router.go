package api

import (
	"database/sql"
	"encoding/json"
	"net/http"
)

func NewRouter(db *sql.DB) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/health", healthHandler)
	mux.HandleFunc("/api/v1/status", statusHandler)

	return mux
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"service": "clicapot",
		"status":  "ok",
		"version": "0.1.0",
	})
}

func statusHandler(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"service": "clicapot",
		"status":  "operational",
		"version": "0.1.0",
		"services": map[string]string{
			"api":          "operational",
			"database":     "operational",
			"dns":          "planned",
			"edge":         "planned",
			"shield":       "planned",
			"compute":      "planned",
			"pages":        "planned",
			"storage":      "planned",
			"kv":           "planned",
			"tunnel":       "planned",
			"access":       "planned",
			"loadbalancer": "planned",
			"analytics":    "planned",
			"queues":       "planned",
			"scheduler":    "planned",
		},
	})
}

func writeJSON(w http.ResponseWriter, status int, value interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
