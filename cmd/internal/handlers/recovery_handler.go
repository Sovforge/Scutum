package handlers

import (
	"encoding/json"
	"net/http"

	"scutum/cmd/internal/kms"
	"scutum/cmd/internal/store"
)

type RecoveryHandler struct {
	db  *store.Store
	kms kms.Provider
}

func NewRecoveryHandler(db *store.Store, provider kms.Provider) *RecoveryHandler {
	return &RecoveryHandler{db: db, kms: provider}
}

type GenerateSharesResponse struct {
	Shares []string `json:"shares"`
}

type GenerateSharesRequest struct {
	NShares   int `json:"n_shares"`
	Threshold int `json:"threshold"`
}

func (h *RecoveryHandler) HandleGenerateShares(w http.ResponseWriter, r *http.Request) {
	base := NewBaseHandler(nil)
	base.Audit("RECOVERY_SHARES_GENERATED", r)

	var req GenerateSharesRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	n, t := validatedRecoveryParams(req.NShares, req.Threshold)

	// Read master key from provider via digest check — we need the raw key.
	// Only local provider exposes this; for cloud KMS, share generation is not applicable.
	type keyExporter interface {
		ExportMasterKey() ([]byte, error)
	}
	exporter, ok := h.kms.(keyExporter)
	if !ok {
		http.Error(w, "share generation is only supported with the local KMS provider", http.StatusBadRequest)
		return
	}

	key, err := exporter.ExportMasterKey()
	if err != nil {
		handlerInternalErr(w, r, "export master key", err)
		return
	}
	defer func() {
		for i := range key {
			key[i] = 0
		}
	}()

	shares, err := kms.EmergencySetup(key, n, t)
	if err != nil {
		handlerInternalErr(w, r, "split key into shares", err)
		return
	}

	shareStrings := make([]string, len(shares))
	for i, s := range shares {
		shareStrings[i] = s.String()
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(GenerateSharesResponse{Shares: shareStrings})
}

type RecoverRequest struct {
	Shares    []string `json:"shares"`
	NShares   int      `json:"n_shares,omitempty"`
	Threshold int      `json:"threshold,omitempty"`
}

type RecoverResponse struct {
	Status    string   `json:"status"`
	NewShares []string `json:"new_shares,omitempty"`
}

func (h *RecoveryHandler) HandleRecover(w http.ResponseWriter, r *http.Request) {
	base := NewBaseHandler(nil)
	base.Audit("RECOVERY_STARTED", r)

	ctx := r.Context()

	var req RecoverRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	if len(req.Shares) < 2 {
		http.Error(w, "provide at least 2 recovery shares", http.StatusBadRequest)
		return
	}

	shares := make([]kms.Share, len(req.Shares))
	for i, s := range req.Shares {
		parsed, err := kms.ParseShare(s)
		if err != nil {
			http.Error(w, "one or more shares have an invalid format", http.StatusBadRequest)
			return
		}
		shares[i] = parsed
	}

	n, t := validatedRecoveryParams(req.NShares, req.Threshold)

	newShares, err := kms.EmergencyRecover(ctx, h.db, h.kms, shares, n, t)
	if err != nil {
		base.Audit("RECOVERY_FAILED", r)
		handlerInternalErr(w, r, "emergency recovery", err)
		return
	}

	base.Audit("RECOVERY_COMPLETED", r)

	shareStrings := make([]string, len(newShares))
	for i, s := range newShares {
		shareStrings[i] = s.String()
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(RecoverResponse{
		Status:    "success",
		NewShares: shareStrings,
	})
}

type ReissueRequest struct {
	Shares    []string `json:"shares"`
	NShares   int      `json:"n_shares,omitempty"`
	Threshold int      `json:"threshold,omitempty"`
}

type ReissueResponse struct {
	Status    string   `json:"status"`
	NewShares []string `json:"new_shares"`
}

func (h *RecoveryHandler) HandleReissueShares(w http.ResponseWriter, r *http.Request) {
	base := NewBaseHandler(nil)
	base.Audit("RECOVERY_REISSUE_STARTED", r)

	var req ReissueRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	if len(req.Shares) < 2 {
		http.Error(w, "provide at least 2 recovery shares", http.StatusBadRequest)
		return
	}

	shares := make([]kms.Share, len(req.Shares))
	for i, s := range req.Shares {
		parsed, err := kms.ParseShare(s)
		if err != nil {
			http.Error(w, "one or more shares have an invalid format", http.StatusBadRequest)
			return
		}
		shares[i] = parsed
	}

	n, t := validatedRecoveryParams(req.NShares, req.Threshold)

	newShares, err := kms.ReissueShares(h.kms, shares, n, t)
	if err != nil {
		base.Audit("RECOVERY_REISSUE_FAILED", r)
		handlerInternalErr(w, r, "reissue shares", err)
		return
	}

	base.Audit("RECOVERY_REISSUE_COMPLETED", r)

	shareStrings := make([]string, len(newShares))
	for i, s := range newShares {
		shareStrings[i] = s.String()
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(ReissueResponse{
		Status:    "success",
		NewShares: shareStrings,
	})
}
