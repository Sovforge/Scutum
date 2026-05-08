package handlers

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"scutum/cmd/internal/store"
	"scutum/cmd/internal/utils"

	"github.com/google/uuid"
)

type StorageHandler struct {
	store *store.Store
}

func NewStorageHandler(st *store.Store) *StorageHandler {
	return &StorageHandler{store: st}
}

// ── List backends ────────────────────────────────────────────────────────────

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func (h *StorageHandler) HandleListBackends(w http.ResponseWriter, r *http.Request) {
	backends, err := h.store.ListStorageBackends(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if backends == nil {
		backends = []store.StorageBackend{}
	}
	writeJSON(w, http.StatusOK, backends)
}

// ── Create backend ───────────────────────────────────────────────────────────

type createBackendReq struct {
	Name      string `json:"name"`
	Provider  string `json:"provider"`
	Endpoint  string `json:"endpoint"`
	Region    string `json:"region"`
	AccessKey string `json:"access_key"`
	SecretKey string `json:"secret_key"`
	PathStyle bool   `json:"path_style"`
	UseSSL    bool   `json:"use_ssl"`
}

func (h *StorageHandler) HandleCreateBackend(w http.ResponseWriter, r *http.Request) {
	var req createBackendReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.Name == "" || req.Endpoint == "" {
		http.Error(w, "name and endpoint are required", http.StatusBadRequest)
		return
	}
	if req.Region == "" {
		req.Region = "us-east-1"
	}
	b := store.StorageBackend{
		ID:        uuid.New().String(),
		Name:      req.Name,
		Provider:  req.Provider,
		Endpoint:  req.Endpoint,
		Region:    req.Region,
		AccessKey: req.AccessKey,
		PathStyle: req.PathStyle,
		UseSSL:    req.UseSSL,
	}
	if err := h.store.CreateStorageBackend(r.Context(), b, req.SecretKey); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusCreated, b)
}

// ── Delete backend ───────────────────────────────────────────────────────────

func (h *StorageHandler) HandleDeleteBackend(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := h.store.DeleteStorageBackend(r.Context(), id); err != nil {
		code := http.StatusInternalServerError
		if strings.Contains(err.Error(), "not found") {
			code = http.StatusNotFound
		}
		http.Error(w, err.Error(), code)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ── Test connection ──────────────────────────────────────────────────────────

func (h *StorageHandler) HandleTestBackend(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	b, secret, err := h.store.GetStorageBackendWithSecret(r.Context(), id)
	if err != nil {
		http.Error(w, "backend not found", http.StatusNotFound)
		return
	}
	buckets, err := listS3Buckets(b, secret)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "buckets": len(buckets)})
}

// ── List buckets ─────────────────────────────────────────────────────────────

type BucketInfo struct {
	Name      string `json:"name"`
	CreatedAt string `json:"created_at,omitempty"`
}

func (h *StorageHandler) HandleListBuckets(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	b, secret, err := h.store.GetStorageBackendWithSecret(r.Context(), id)
	if err != nil {
		http.Error(w, "backend not found", http.StatusNotFound)
		return
	}
	buckets, err := listS3Buckets(b, secret)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	if buckets == nil {
		buckets = []BucketInfo{}
	}
	writeJSON(w, http.StatusOK, buckets)
}

// ── S3 bucket listing ────────────────────────────────────────────────────────

type xmlListBucketsResult struct {
	XMLName xml.Name `xml:"ListAllMyBucketsResult"`
	Buckets struct {
		Buckets []struct {
			Name         string `xml:"Name"`
			CreationDate string `xml:"CreationDate"`
		} `xml:"Bucket"`
	} `xml:"Buckets"`
}

func listS3Buckets(b store.StorageBackend, secret string) ([]BucketInfo, error) {
	scheme := "http"
	if b.UseSSL {
		scheme = "https"
	}
	url := fmt.Sprintf("%s://%s/", scheme, b.Endpoint)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Host = b.Endpoint

	utils.SignS3Request(req, nil, utils.S3Config{
		Endpoint:  b.Endpoint,
		Bucket:    "",
		Region:    b.Region,
		AccessKey: b.AccessKey,
		SecretKey: secret,
		UseSSL:    b.UseSSL,
	})

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("connection failed: %s", err.Error())
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("S3 error %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var result xmlListBucketsResult
	if err := xml.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("parse response: %s", err.Error())
	}

	var buckets []BucketInfo
	for _, bkt := range result.Buckets.Buckets {
		buckets = append(buckets, BucketInfo{Name: bkt.Name, CreatedAt: bkt.CreationDate})
	}
	return buckets, nil
}
