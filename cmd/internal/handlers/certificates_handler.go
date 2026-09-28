package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"scutum/cmd/internal/certs"
)

// CertificatesHandler exposes the hub's TLS certificate inventory and
// supports in-place rotation. store is nil when TLS is off or ACME-managed
// (ACME renews its own certs automatically — see cmd/internal/acme).
type CertificatesHandler struct {
	store    *certs.Store
	mode     string // "acme" | "manual" | "none"
	warnDays int
}

func NewCertificatesHandler(store *certs.Store, mode string, warnDays int) *CertificatesHandler {
	return &CertificatesHandler{store: store, mode: mode, warnDays: warnDays}
}

// HandleList — GET /admin/certificates.
func (h *CertificatesHandler) HandleList(w http.ResponseWriter, r *http.Request) {
	list := []certs.Info{}
	if h.store != nil {
		list = h.store.Info(h.warnDays)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"mode":         h.mode,
		"warn_days":    h.warnDays,
		"certificates": list,
	})
}

type rotateCertReq struct {
	Role    string `json:"role"` // "server" | "ca"
	CertPEM string `json:"cert_pem"`
	KeyPEM  string `json:"key_pem,omitempty"`
}

// HandleRotate — POST /admin/certificates/rotate.
func (h *CertificatesHandler) HandleRotate(w http.ResponseWriter, r *http.Request) {
	if h.mode != "manual" || h.store == nil {
		http.Error(w, "certificate rotation is only available when TLS is configured manually via CERT_FILE/KEY_FILE — ACME-managed certificates renew automatically, and there is nothing to rotate when TLS is disabled", http.StatusConflict)
		return
	}

	var req rotateCertReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.CertPEM == "" {
		http.Error(w, "cert_pem is required", http.StatusBadRequest)
		return
	}

	var info certs.Info
	var err error
	switch req.Role {
	case "server":
		if req.KeyPEM == "" {
			http.Error(w, "key_pem is required when rotating the server certificate", http.StatusBadRequest)
			return
		}
		info, err = h.store.RotateServerCert([]byte(req.CertPEM), []byte(req.KeyPEM))
	case "ca":
		info, err = h.store.RotateCA([]byte(req.CertPEM))
	default:
		http.Error(w, `role must be "server" or "ca"`, http.StatusBadRequest)
		return
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	audit("CERTIFICATE_ROTATED", r, "role", req.Role, "subject", info.Subject, "not_after", info.NotAfter.Format(time.RFC3339))
	writeJSON(w, http.StatusOK, info)
}
