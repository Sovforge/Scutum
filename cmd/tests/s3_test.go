package tests

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"scutum/cmd/internal/handlers"
	"scutum/cmd/internal/utils"
)

func TestSignS3RequestSetsRequiredHeaders(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "http://s3.amazonaws.com/my-bucket/my-key", nil)
	req.Host = "s3.amazonaws.com"

	cfg := utils.S3Config{
		Endpoint:  "s3.amazonaws.com",
		Region:    "us-east-1",
		Bucket:    "my-bucket",
		AccessKey: "AKIAIOSFODNN7EXAMPLE",
		SecretKey: "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
		UseSSL:    true,
	}

	utils.SignS3Request(req, nil, cfg)

	if req.Header.Get("x-amz-date") == "" {
		t.Error("SignS3Request() missing x-amz-date header")
	}
	if req.Header.Get("x-amz-content-sha256") == "" {
		t.Error("SignS3Request() missing x-amz-content-sha256 header")
	}
	if req.Header.Get("Authorization") == "" {
		t.Error("SignS3Request() missing Authorization header")
	}
}

func TestSignS3RequestAuthorizationFormat(t *testing.T) {
	req := httptest.NewRequest(http.MethodPut, "http://s3.amazonaws.com/my-bucket/my-key", nil)
	req.Host = "s3.amazonaws.com"

	cfg := utils.S3Config{
		Endpoint:  "s3.amazonaws.com",
		Region:    "eu-west-1",
		Bucket:    "my-bucket",
		AccessKey: "AKIAIOSFODNN7EXAMPLE",
		SecretKey: "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
	}

	utils.SignS3Request(req, []byte("hello world"), cfg)

	auth := req.Header.Get("Authorization")
	if auth == "" {
		t.Fatal("Authorization header is empty")
	}

	for _, part := range []string{"AWS4-HMAC-SHA256", "Credential=", "SignedHeaders=", "Signature="} {
		if !contains(auth, part) {
			t.Errorf("Authorization missing %q, got: %s", part, auth)
		}
	}
}

func TestSignS3RequestRegionInCredential(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "http://minio.local:9000/bucket/key", nil)
	req.Host = "minio.local:9000"

	cfg := utils.S3Config{
		Endpoint:  "minio.local:9000",
		Region:    "main",
		Bucket:    "bucket",
		AccessKey: "minioadmin",
		SecretKey: "minioadmin",
	}

	utils.SignS3Request(req, nil, cfg)

	auth := req.Header.Get("Authorization")
	if !contains(auth, "/main/") {
		t.Errorf("Authorization credential missing region, got: %s", auth)
	}
}

func TestSignS3RequestDifferentMethods(t *testing.T) {
	cfg := utils.S3Config{
		Endpoint:  "s3.amazonaws.com",
		Region:    "us-east-1",
		Bucket:    "bucket",
		AccessKey: "key",
		SecretKey: "secret",
	}

	for _, method := range []string{http.MethodGet, http.MethodPut, http.MethodDelete} {
		req := httptest.NewRequest(method, "http://s3.amazonaws.com/bucket/key", nil)
		req.Host = "s3.amazonaws.com"

		utils.SignS3Request(req, nil, cfg)

		if req.Header.Get("Authorization") == "" {
			t.Errorf("method %s: missing Authorization header", method)
		}
	}
}

func TestSignS3RequestUniqueSignatures(t *testing.T) {
	cfg := utils.S3Config{
		Endpoint:  "s3.amazonaws.com",
		Region:    "us-east-1",
		Bucket:    "bucket",
		AccessKey: "key",
		SecretKey: "secret",
	}

	req1 := httptest.NewRequest(http.MethodGet, "http://s3.amazonaws.com/bucket/key1", nil)
	req1.Host = "s3.amazonaws.com"
	utils.SignS3Request(req1, nil, cfg)

	req2 := httptest.NewRequest(http.MethodGet, "http://s3.amazonaws.com/bucket/key2", nil)
	req2.Host = "s3.amazonaws.com"
	utils.SignS3Request(req2, nil, cfg)

	if req1.Header.Get("Authorization") == req2.Header.Get("Authorization") {
		t.Error("different paths produced identical signatures")
	}
}

func TestS3HandlerInitialization(t *testing.T) {
	handler := handlers.NewS3Handler()
	if handler == nil {
		t.Error("NewS3Handler() returned nil")
	}
}

func TestS3HandlerMethods(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPut:
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("uploaded"))
		case http.MethodGet:
			if strings.HasSuffix(r.URL.Path, "/bucket") {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("<ListBucketResult/>"))
				return
			}
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("downloaded"))
		case http.MethodDelete:
			w.WriteHeader(http.StatusNoContent)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	}))
	defer server.Close()

	endpoint := strings.TrimPrefix(server.URL, "http://")
	h := handlers.NewS3Handler(handlers.WithEndpointValidator(func(string) error { return nil }))

	tests := []struct {
		name       string
		handler    func(http.ResponseWriter, *http.Request)
		request    handlers.S3SyncRequest
		statusCode int
	}{
		{
			name:       "upload",
			handler:    h.HandleUpload,
			request:    handlers.S3SyncRequest{Endpoint: endpoint, Bucket: "bucket", Region: "us-east-1", AccessKey: "key", SecretKey: "secret", UseSSL: false, ObjectName: "file.txt", Payload: []byte("hello")},
			statusCode: http.StatusOK,
		},
		{
			name:       "download",
			handler:    h.HandleDownload,
			request:    handlers.S3SyncRequest{Endpoint: endpoint, Bucket: "bucket", Region: "us-east-1", AccessKey: "key", SecretKey: "secret", UseSSL: false, ObjectName: "file.txt"},
			statusCode: http.StatusOK,
		},
		{
			name:       "delete",
			handler:    h.HandleDelete,
			request:    handlers.S3SyncRequest{Endpoint: endpoint, Bucket: "bucket", Region: "us-east-1", AccessKey: "key", SecretKey: "secret", UseSSL: false, ObjectName: "file.txt"},
			statusCode: http.StatusNoContent,
		},
		{
			name:       "list",
			handler:    h.HandleList,
			request:    handlers.S3SyncRequest{Endpoint: endpoint, Bucket: "bucket", Region: "us-east-1", AccessKey: "key", SecretKey: "secret", UseSSL: false},
			statusCode: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, err := json.Marshal(tt.request)
			if err != nil {
				t.Fatalf("marshal request: %v", err)
			}
			req := httptest.NewRequest(http.MethodPost, "/s3", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			tt.handler(w, req)
			if w.Code != tt.statusCode {
				t.Fatalf("expected status %d, got %d", tt.statusCode, w.Code)
			}
		})
	}
}

func TestS3HandlerErrors(t *testing.T) {
	h := handlers.NewS3Handler()

	t.Run("invalid-json", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/s3", strings.NewReader(`{invalid`))
		w := httptest.NewRecorder()
		h.HandleUpload(w, req)
		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400, got %d", w.Code)
		}
	})

	t.Run("empty-body-upload", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/s3", strings.NewReader(`{}`))
		w := httptest.NewRecorder()
		h.HandleUpload(w, req)
		if w.Code != http.StatusInternalServerError && w.Code != http.StatusBadRequest {
			t.Errorf("expected 400 or 500, got %d", w.Code)
		}
	})

	t.Run("empty-body-download", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/s3", strings.NewReader(`{}`))
		w := httptest.NewRecorder()
		h.HandleDownload(w, req)
		if w.Code != http.StatusInternalServerError && w.Code != http.StatusBadRequest {
			t.Errorf("expected 400 or 500, got %d", w.Code)
		}
	})

	t.Run("network-error", func(t *testing.T) {
		h := handlers.NewS3Handler()
		req := handlers.S3SyncRequest{
			Endpoint:  "nonexistent.invalid:9999",
			Bucket:    "bucket",
			Region:    "us-east-1",
			AccessKey: "key",
			SecretKey: "secret",
		}
		body, _ := json.Marshal(req)
		httpReq := httptest.NewRequest(http.MethodPost, "/s3", bytes.NewReader(body))
		w := httptest.NewRecorder()
		h.HandleUpload(w, httpReq)
		if w.Code != http.StatusInternalServerError {
			t.Logf("expected 500 for network error, got %d", w.Code)
		}
	})
}
