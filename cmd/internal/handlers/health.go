package handlers

import (
	"encoding/json"
	"net/http"
)

// HealthHandler returns the API status
func HealthHandler(w http.ResponseWriter, r *http.Request) {
	data := map[string]string{"status": "up"}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}
