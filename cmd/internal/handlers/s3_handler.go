package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"

	"scutum/cmd/internal/utils"
)

type S3Handler struct {
	validateEndpoint func(string) error
}

// WithEndpointValidator overrides the SSRF validator — use in tests only.
func WithEndpointValidator(fn func(string) error) func(*S3Handler) {
	return func(h *S3Handler) { h.validateEndpoint = fn }
}

func NewS3Handler(opts ...func(*S3Handler)) *S3Handler {
	h := &S3Handler{validateEndpoint: validateS3Endpoint}
	for _, opt := range opts {
		opt(h)
	}
	return h
}

// S3SyncRequest is the universal payload for all S3 operations
type S3SyncRequest struct {
	Endpoint   string `json:"endpoint"` // e.g., "s3.amazonaws.com" or "minio.local:9000"
	Bucket     string `json:"bucket"`
	Region     string `json:"region"` // e.g., "us-east-1"
	AccessKey  string `json:"access_key"`
	SecretKey  string `json:"secret_key"`
	UseSSL     bool   `json:"use_ssl"`
	ObjectName string `json:"object_name"` // Required for Upload/Download/Delete
	Payload    []byte `json:"payload"`     // Only used for Upload
}

// --- CORE METHODS ---

// HandleUpload (PUT) - Backs up a stack, config, or tarball to the provider.
func (h *S3Handler) HandleUpload(w http.ResponseWriter, r *http.Request) {
	req, ok := h.decodeRequest(w, r)
	if !ok {
		return
	}

	url := h.buildURL(req, true)
	httpReq, _ := http.NewRequest("PUT", url, bytes.NewReader(req.Payload))
	httpReq.Host = req.Endpoint

	utils.SignS3Request(httpReq, req.Payload, h.toConfig(req))
	h.execute(w, httpReq)
}

// HandleDownload (GET) - Retrieves a specific object (e.g., for a "Restore" operation).
func (h *S3Handler) HandleDownload(w http.ResponseWriter, r *http.Request) {
	req, ok := h.decodeRequest(w, r)
	if !ok {
		return
	}

	url := h.buildURL(req, true)
	httpReq, _ := http.NewRequest("GET", url, nil)
	httpReq.Host = req.Endpoint

	utils.SignS3Request(httpReq, nil, h.toConfig(req))
	h.execute(w, httpReq)
}

// HandleDelete (DELETE) - Removes a specific backup from the provider.
func (h *S3Handler) HandleDelete(w http.ResponseWriter, r *http.Request) {
	req, ok := h.decodeRequest(w, r)
	if !ok {
		return
	}

	url := h.buildURL(req, true)
	httpReq, _ := http.NewRequest("DELETE", url, nil)
	httpReq.Host = req.Endpoint

	utils.SignS3Request(httpReq, nil, h.toConfig(req))
	h.execute(w, httpReq)
}

// HandleList (GET Bucket) - Returns an XML list of all objects in the bucket.
func (h *S3Handler) HandleList(w http.ResponseWriter, r *http.Request) {
	req, ok := h.decodeRequest(w, r)
	if !ok {
		return
	}

	// For listing, we target the bucket root, not a specific object
	url := h.buildURL(req, false)
	httpReq, _ := http.NewRequest("GET", url, nil)
	httpReq.Host = req.Endpoint

	utils.SignS3Request(httpReq, nil, h.toConfig(req))

	w.Header().Set("Content-Type", "application/xml") // S3 lists are always XML
	h.execute(w, httpReq)
}

// --- INTERNAL HELPERS ---

// decodeRequest parses the incoming JSON and handles errors
func (h *S3Handler) decodeRequest(w http.ResponseWriter, r *http.Request) (S3SyncRequest, bool) {
	var req S3SyncRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid Request JSON: "+err.Error(), http.StatusBadRequest)
		return req, false
	}
	if err := h.validateEndpoint(req.Endpoint); err != nil {
		http.Error(w, "Invalid endpoint: "+err.Error(), http.StatusBadRequest)
		return req, false
	}
	return req, true
}

// validateS3Endpoint rejects endpoints that resolve to private/loopback addresses.
func validateS3Endpoint(endpoint string) error {
	host := endpoint
	if h, _, err := net.SplitHostPort(endpoint); err == nil {
		host = h
	}
	addrs, err := net.LookupHost(host)
	if err != nil {
		return fmt.Errorf("cannot resolve endpoint: %v", err)
	}
	for _, addr := range addrs {
		ip := net.ParseIP(addr)
		if ip == nil {
			continue
		}
		if ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
			return fmt.Errorf("endpoint resolves to a reserved address")
		}
		private := []string{"10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16", "fc00::/7"}
		for _, cidr := range private {
			_, network, _ := net.ParseCIDR(cidr)
			if network.Contains(ip) {
				return fmt.Errorf("endpoint resolves to a private address")
			}
		}
	}
	return nil
}

// buildURL constructs the Path-Style URL for maximum compatibility
func (h *S3Handler) buildURL(req S3SyncRequest, includeObject bool) string {
	scheme := "http"
	if req.UseSSL {
		scheme = "https"
	}

	if includeObject && req.ObjectName != "" {
		return fmt.Sprintf("%s://%s/%s/%s", scheme, req.Endpoint, req.Bucket, req.ObjectName)
	}
	return fmt.Sprintf("%s://%s/%s", scheme, req.Endpoint, req.Bucket)
}

// toConfig converts the request to the internal utility config format
func (h *S3Handler) toConfig(req S3SyncRequest) utils.S3Config {
	return utils.S3Config{
		Endpoint: req.Endpoint, Bucket: req.Bucket, Region: req.Region,
		AccessKey: req.AccessKey, SecretKey: req.SecretKey, UseSSL: req.UseSSL,
	}
}

// execute performs the HTTP call and pipes the result to the client
func (h *S3Handler) execute(w http.ResponseWriter, req *http.Request) {
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		http.Error(w, "S3 Request Failed: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	// Forward the status code and the body content
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}
