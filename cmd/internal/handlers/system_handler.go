package handlers

import (
	"encoding/json"
	"net/http"
	"os"
)

type SystemHandler struct{}

func NewSystemHandler() *SystemHandler { return &SystemHandler{} }

type tlsModeResponse struct {
	Mode     string `json:"mode"`              // "acme" | "manual" | "none"
	Domain   string `json:"domain,omitempty"`  // ACME only
	Email    string `json:"email,omitempty"`   // ACME only
	Staging  bool   `json:"staging,omitempty"` // ACME only
	CertFile string `json:"cert_file,omitempty"` // manual only
}

func (h *SystemHandler) HandleTLSMode(w http.ResponseWriter, r *http.Request) {
	resp := tlsModeResponse{Mode: "none"}

	if domain := os.Getenv("ACME_DOMAIN"); domain != "" && os.Getenv("ACME_EMAIL") != "" {
		resp.Mode = "acme"
		resp.Domain = domain
		resp.Email = os.Getenv("ACME_EMAIL")
		resp.Staging = os.Getenv("ACME_STAGING") == "true"
	} else if certFile := os.Getenv("CERT_FILE"); certFile != "" {
		if _, err := os.Stat(certFile); err == nil {
			resp.Mode = "manual"
			resp.CertFile = certFile
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}
