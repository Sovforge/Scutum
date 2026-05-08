package handlers

import (
	"encoding/json"
	"net/http"
)

// NodeHandler returns the API status
func GetNodeHandler(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"node_id": id, "status": "online"})
}
