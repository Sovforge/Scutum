package handlers

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strings"

	"scutum/cmd/internal/utils"
)

type operatorStore interface {
	GetSecret(ctx context.Context, key string) ([]byte, error)
}

// OperatorHandler exposes bootstrap data for the Kubernetes operator.
type OperatorHandler struct {
	store operatorStore
}

// NewOperatorHandler creates a new OperatorHandler.
func NewOperatorHandler(s operatorStore) *OperatorHandler {
	return &OperatorHandler{store: s}
}

// bootstrapResponse is the JSON returned by GET /api/operator/bootstrap.
type bootstrapResponse struct {
	HubWGPublicKey string `json:"hub_wg_public_key"`
	HubWGPort      int    `json:"hub_wg_port"`
	HubHMACKey     string `json:"hub_hmac_key"`
	HubMeshCIDR    string `json:"hub_mesh_cidr"`
}

// HandleBootstrap returns the hub's WireGuard public key, port, HMAC key, and
// mesh CIDR. These are used by the Kubernetes operator to configure edge nodes
// without requiring operator access to the hub's secrets volume.
func (h *OperatorHandler) HandleBootstrap(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// --- hub_hmac_key ---
	hmacKeyBytes, err := h.store.GetSecret(ctx, "hub_hmac_key")
	if err != nil {
		http.Error(w, "hub_hmac_key not available", http.StatusInternalServerError)
		return
	}
	hmacKeyB64 := base64.StdEncoding.EncodeToString(hmacKeyBytes)

	// --- wg0_config → port + mesh CIDR ---
	wg0ConfigBytes, err := h.store.GetSecret(ctx, "wg0_config")
	if err != nil {
		http.Error(w, "wg0_config not available", http.StatusInternalServerError)
		return
	}
	var wg0Cfg utils.InterfaceConfig
	if err := json.Unmarshal(wg0ConfigBytes, &wg0Cfg); err != nil {
		http.Error(w, "failed to parse wg0_config", http.StatusInternalServerError)
		return
	}

	port := wg0Cfg.Port
	if port == 0 {
		port = 51820
	}

	meshCIDR := wg0Cfg.Address
	// Normalise: wg0_config.Address may be "10.x.x.x/24" — use as-is.
	meshCIDR = strings.TrimSpace(meshCIDR)

	// --- WireGuard public key from running interface ---
	pubKeyBytes, err := utils.DefaultCommandRunner.Output("wg", "show", "wg0", "public-key")
	if err != nil {
		http.Error(w, "failed to read wg0 public key", http.StatusInternalServerError)
		return
	}
	pubKey := strings.TrimSpace(string(pubKeyBytes))
	if pubKey == "" {
		http.Error(w, "empty wg0 public key", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(bootstrapResponse{
		HubWGPublicKey: pubKey,
		HubWGPort:      port,
		HubHMACKey:     hmacKeyB64,
		HubMeshCIDR:    meshCIDR,
	})
}
