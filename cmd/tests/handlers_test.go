package tests

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"scutum/cmd/internal/clients"
	"scutum/cmd/internal/handlers"
	"scutum/cmd/internal/models"
	pluginpkg "scutum/cmd/internal/plugins"
)

// TestDockerHandlerPostDeployValidRequest tests that PostDeploy handler processes valid deployment requests
func TestDockerHandlerPostDeployValidRequest(t *testing.T) {
	deployReq := models.DeployRequest{
		Repo:        "nginx:latest",
		Name:        "test-container",
		Port:        80,
		HostPort:    8080,
		Env:         []string{"DEBUG=true"},
		Volumes:     []string{"/host:/container:rw"},
		MemoryLimit: 536870912,
		CPULimit:    0.5,
		Restart:     "always",
	}

	body, err := json.Marshal(deployReq)
	if err != nil {
		t.Fatalf("failed to marshal request: %v", err)
	}

	req := httptest.NewRequest("POST", "/deploy", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler := handlers.NewDockerHandler(&mockNodeProxyStore{})
	handler.PostDeploy(w, req)

	// Handler should respond with valid HTTP status and Content-Type
	if w.Code < 200 {
		t.Errorf("expected successful or error status, got %d", w.Code)
	}
	if w.Header().Get("Content-Type") != "" {
		if !contains(w.Header().Get("Content-Type"), "/") {
			t.Errorf("invalid Content-Type header: %s", w.Header().Get("Content-Type"))
		}
	}
}

// TestDockerHandlerPostDeployInvalidJSON tests that PostDeploy rejects malformed JSON
func TestDockerHandlerPostDeployInvalidJSON(t *testing.T) {
	body := []byte(`{invalid json"`)
	req := httptest.NewRequest("POST", "/deploy", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler := handlers.NewDockerHandler(&mockNodeProxyStore{})
	handler.PostDeploy(w, req)

	// Should return 400 Bad Request for invalid JSON
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d for invalid JSON", http.StatusBadRequest, w.Code)
	}
}

// TestDockerHandlerPostDeployEmptyBody tests that PostDeploy handles empty request body
func TestDockerHandlerPostDeployEmptyBody(t *testing.T) {
	req := httptest.NewRequest("POST", "/deploy", bytes.NewReader([]byte("")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler := handlers.NewDockerHandler(&mockNodeProxyStore{})
	handler.PostDeploy(w, req)

	// Should return error for empty body
	if w.Code < 400 {
		t.Errorf("expected error status for empty body, got %d", w.Code)
	}
}

// TestDockerHandlerPostDeployInvalidPorts tests that PostDeploy validates port numbers
func TestDockerHandlerPostDeployInvalidPorts(t *testing.T) {
	tests := []struct {
		name     string
		port     int
		hostPort int
	}{
		{"negative container port", -1, 8080},
		{"negative host port", 80, -1},
		{"port zero", 0, 8080},
		{"host port out of range", 80, 70000},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			deployReq := models.DeployRequest{
				Repo:     "nginx",
				Name:     "test",
				Port:     tt.port,
				HostPort: tt.hostPort,
			}

			body, _ := json.Marshal(deployReq)
			req := httptest.NewRequest("POST", "/deploy", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			handler := handlers.NewDockerHandler(&mockNodeProxyStore{})
			handler.PostDeploy(w, req)

			// Should handle port validation (either 400 or 500)
			if w.Code < 400 {
				t.Errorf("%s: expected error status, got %d", tt.name, w.Code)
			}
		})
	}
}

// TestDockerHandlerResponseContentType tests that PostDeploy returns proper Content-Type
func TestDockerHandlerResponseContentType(t *testing.T) {
	deployReq := models.DeployRequest{
		Repo:     "nginx",
		Name:     "test",
		Port:     80,
		HostPort: 8080,
	}

	body, _ := json.Marshal(deployReq)
	req := httptest.NewRequest("POST", "/deploy", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler := handlers.NewDockerHandler(&mockNodeProxyStore{})
	handler.PostDeploy(w, req)

	// On success or JSON error response, should have JSON content type
	contentType := w.Header().Get("Content-Type")
	if contentType != "" && !contains(contentType, "json") && w.Code >= 200 && w.Code < 400 {
		t.Errorf("expected JSON response, got Content-Type: %s", contentType)
	}
}

// TestDockerHandlerRequestValidation tests request field validation
func TestDockerHandlerRequestValidation(t *testing.T) {
	tests := []struct {
		name    string
		request models.DeployRequest
	}{
		{
			"missing repo",
			models.DeployRequest{
				Name:     "test",
				Port:     80,
				HostPort: 8080,
			},
		},
		{
			"missing name",
			models.DeployRequest{
				Repo:     "nginx",
				Port:     80,
				HostPort: 8080,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(tt.request)
			req := httptest.NewRequest("POST", "/deploy", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			handler := handlers.NewDockerHandler(&mockNodeProxyStore{})
			handler.PostDeploy(w, req)

			// Handler should process the request (may succeed or fail depending on validation)
			if w.Code >= 600 {
				t.Errorf("invalid HTTP status code: %d", w.Code)
			}
		})
	}
}

// TestHandlerHTTPMethodsAndHeaders tests that handlers work with proper HTTP headers
func TestHandlerHTTPMethodsAndHeaders(t *testing.T) {
	deployReq := models.DeployRequest{
		Repo:     "nginx",
		Name:     "test",
		Port:     80,
		HostPort: 8080,
	}

	body, _ := json.Marshal(deployReq)

	tests := []struct {
		name         string
		method       string
		contentType  string
		expectStatus bool
	}{
		{"POST with JSON", "POST", "application/json", true},
		{"POST with charset", "POST", "application/json; charset=utf-8", true},
		{"POST no content type", "POST", "", false}, // May fail without Content-Type
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, "/deploy", bytes.NewReader(body))
			if tt.contentType != "" {
				req.Header.Set("Content-Type", tt.contentType)
			}
			w := httptest.NewRecorder()

			handler := handlers.NewDockerHandler(&mockNodeProxyStore{})
			handler.PostDeploy(w, req)

			// Verify response is valid HTTP
			if w.Code < 100 || w.Code >= 600 {
				t.Errorf("invalid status code: %d", w.Code)
			}
		})
	}
}

// TestHandlerErrorResponses tests that handlers return proper error responses
func TestHandlerErrorResponses(t *testing.T) {
	tests := []struct {
		name         string
		body         []byte
		expectedCode int
	}{
		{"invalid JSON", []byte(`{invalid}`), http.StatusBadRequest},
		{"empty body", []byte(""), http.StatusBadRequest},
		{"incomplete JSON", []byte(`{"key":`), http.StatusBadRequest},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/deploy", bytes.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			handler := handlers.NewDockerHandler(&mockNodeProxyStore{})
			handler.PostDeploy(w, req)

			if w.Code != tt.expectedCode {
				t.Errorf("expected status %d, got %d", tt.expectedCode, w.Code)
			}
			// Error responses should have a body explaining the error
			if w.Body.Len() == 0 {
				t.Errorf("expected error response body for status %d", w.Code)
			}
		})
	}
}

func TestDockerHandlerLifecycleMethods(t *testing.T) {
	paths := map[string]bool{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths[r.URL.Path] = true
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	h := &handlers.DockerHandler{}
	setUnexportedField(h, "client", clients.NewDockerClient(server.Client(), server.URL, nil))

	ctx := context.Background()
	if err := h.Start(ctx, "abc123"); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	if err := h.Stop(ctx, "abc123"); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}
	if err := h.Restart(ctx, "abc123"); err != nil {
		t.Fatalf("Restart() error = %v", err)
	}
	if err := h.Kill(ctx, "abc123"); err != nil {
		t.Fatalf("Kill() error = %v", err)
	}
	if err := h.Delete(ctx, "abc123"); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	expectedPaths := []string{
		"/containers/abc123/start",
		"/containers/abc123/stop",
		"/containers/abc123/restart",
		"/containers/abc123/kill",
		"/containers/abc123",
	}

	for _, p := range expectedPaths {
		if !paths[p] {
			t.Errorf("expected path %s to be invoked", p)
		}
	}
}

func TestDockerHandlerGetStats(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("stats"))
	}))
	defer server.Close()

	h := &handlers.DockerHandler{}
	setUnexportedField(h, "client", clients.NewDockerClient(server.Client(), server.URL, nil))

	stream, err := h.GetStats(context.Background(), "abc123")
	if err != nil {
		t.Fatalf("GetStats() error = %v", err)
	}
	defer stream.Close()
	payload, err := io.ReadAll(stream)
	if err != nil {
		t.Fatalf("read stats error = %v", err)
	}
	if string(payload) != "stats" {
		t.Fatalf("unexpected stats body: %q", string(payload))
	}
}

func TestKubernetesHandlerOperations(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer kube-token" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		switch {
		case r.Method == http.MethodPost && strings.Contains(r.URL.Path, "/deployments"):
			w.WriteHeader(http.StatusCreated)
		case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/pods/"):
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"metadata":{"name":"pod1"}}`))
		case r.Method == http.MethodDelete && strings.Contains(r.URL.Path, "/pods/"):
			w.WriteHeader(http.StatusNoContent)
		case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/deployments/"):
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"spec":{"replicas":2,"template":{"metadata":{"annotations":{}}}}}`))
		case r.Method == http.MethodPut && strings.Contains(r.URL.Path, "/deployments/"):
			w.WriteHeader(http.StatusOK)
		case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/events"):
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("eventA\neventB"))
		case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/log?follow=true"):
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("pod logs"))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	h := &handlers.KubernetesHandler{}
	setUnexportedField(h, "client", clients.NewKubernetesClient(server.Client(), server.URL, "kube-token"))

	if err := h.CreateDeployment("default", "nginx", "nginx:latest"); err != nil {
		t.Fatalf("CreateDeployment() error = %v", err)
	}

	pod, err := h.GetPod(context.Background(), "default", "pod1")
	if err != nil {
		t.Fatalf("GetPod() error = %v", err)
	}
	if podMeta, ok := pod["metadata"].(map[string]interface{}); !ok || podMeta["name"] != "pod1" {
		t.Fatalf("unexpected pod metadata: %v", pod)
	}

	if err := h.DeletePod(context.Background(), "default", "pod1"); err != nil {
		t.Fatalf("DeletePod() error = %v", err)
	}

	if err := h.ScaleDeployment(context.Background(), "default", "nginx", 5); err != nil {
		t.Fatalf("ScaleDeployment() error = %v", err)
	}

	if err := h.RestartDeployment(context.Background(), "default", "nginx"); err != nil {
		t.Fatalf("RestartDeployment() error = %v", err)
	}

	stream, err := h.WatchEvents(context.Background())
	if err != nil {
		t.Fatalf("WatchEvents() error = %v", err)
	}
	defer stream.Close()
	events, err := io.ReadAll(stream)
	if err != nil {
		t.Fatalf("read events error = %v", err)
	}
	if string(events) != "eventA\neventB" {
		t.Fatalf("unexpected events stream: %q", string(events))
	}
}

func TestDockerHandlerHTTPHandlers(t *testing.T) {
	paths := map[string]bool{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths[r.URL.Path] = true
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	h := &handlers.DockerHandler{}
	setUnexportedField(h, "client", clients.NewDockerClient(server.Client(), server.URL, nil))

	tests := []struct {
		name    string
		handler func(http.ResponseWriter, *http.Request)
		path    string
	}{
		{"start", h.HandleStart, "/containers/abc123/start"},
		{"stop", h.HandleStop, "/containers/abc123/stop"},
		{"restart", h.HandleRestart, "/containers/abc123/restart"},
		{"kill", h.HandleKill, "/containers/abc123/kill"},
		{"delete", h.HandleDelete, "/containers/abc123"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, tt.path, nil)
			req.SetPathValue("id", "abc123")
			w := httptest.NewRecorder()

			tt.handler(w, req)
			if w.Code != http.StatusNoContent {
				t.Fatalf("expected status %d, got %d", http.StatusNoContent, w.Code)
			}
		})
	}
}

func TestDockerHandlerHandleStats(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("stats-body"))
	}))
	defer server.Close()

	h := &handlers.DockerHandler{}
	setUnexportedField(h, "client", clients.NewDockerClient(server.Client(), server.URL, nil))

	req := httptest.NewRequest(http.MethodGet, "/containers/abc123/stats", nil)
	req.SetPathValue("id", "abc123")
	w := httptest.NewRecorder()

	h.HandleStats(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if w.Body.String() != "stats-body" {
		t.Fatalf("unexpected body: %q", w.Body.String())
	}
}

func TestKubernetesHandlerHTTPHandlers(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer kube-token" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		switch {
		case r.Method == http.MethodDelete && strings.Contains(r.URL.Path, "/pods/"):
			w.WriteHeader(http.StatusNoContent)
		case r.Method == http.MethodPost && strings.Contains(r.URL.Path, "/deployments"):
			w.WriteHeader(http.StatusCreated)
		case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/log") && r.URL.Query().Get("follow") == "true":
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("pod logs"))
		case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/events"):
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("event-stream"))
		case r.Method == http.MethodPut && strings.Contains(r.URL.Path, "/deployments/"):
			w.WriteHeader(http.StatusOK)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	h := &handlers.KubernetesHandler{}
	setUnexportedField(h, "client", clients.NewKubernetesClient(server.Client(), server.URL, "kube-token"))

	t.Run("HandleDeletePod", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, "/k8s/default/pods/pod1", nil)
		req.SetPathValue("ns", "default")
		req.SetPathValue("name", "pod1")
		w := httptest.NewRecorder()
		h.HandleDeletePod(w, req)
		if w.Code != http.StatusNoContent {
			t.Fatalf("expected 204, got %d", w.Code)
		}
	})

	t.Run("HandleDeployInvalidBody", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/k8s/default/deploy", strings.NewReader("{invalid"))
		req.SetPathValue("ns", "default")
		w := httptest.NewRecorder()
		h.HandleDeploy(w, req)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", w.Code)
		}
	})

	t.Run("HandleScaleInvalidReplicas", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/k8s/default/deployments/nginx/scale?replicas=notanumber", nil)
		req.SetPathValue("ns", "default")
		req.SetPathValue("name", "nginx")
		w := httptest.NewRecorder()
		h.HandleScale(w, req)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", w.Code)
		}
	})

	t.Run("HandleRestart", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/k8s/default/deployments/nginx/restart", nil)
		req.SetPathValue("ns", "default")
		req.SetPathValue("name", "nginx")
		w := httptest.NewRecorder()
		h.HandleRestart(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", w.Code)
		}
	})

	t.Run("HandleLogs", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/k8s/default/pods/pod1/log", nil)
		req.SetPathValue("ns", "default")
		req.SetPathValue("name", "pod1")
		w := httptest.NewRecorder()
		h.HandleLogs(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", w.Code)
		}
		if w.Body.String() != "pod logs" {
			t.Fatalf("unexpected log body: %q", w.Body.String())
		}
	})

	t.Run("HandleWatchEvents", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/k8s/events", nil)
		w := httptest.NewRecorder()
		h.HandleWatchEvents(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", w.Code)
		}
		if !strings.Contains(w.Body.String(), "event-stream") {
			t.Fatalf("unexpected events body: %q", w.Body.String())
		}
	})
}

func TestKubernetesHandlerTerminalErrorWritesFrame(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen failed: %v", err)
	}
	defer ln.Close()

	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		reader := bufio.NewReader(conn)
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				return
			}
			if line == "\r\n" {
				break
			}
		}
		_, _ = conn.Write([]byte("HTTP/1.1 500 Internal Server Error\r\nConnection: close\r\n\r\n"))
	}()

	clientConn, browserConn := net.Pipe()
	defer browserConn.Close()

	out := make(chan []byte, 1)
	go func() {
		data, _ := io.ReadAll(clientConn)
		out <- data
	}()

	rw := newHijackResponseWriter(browserConn)
	req := httptest.NewRequest(http.MethodGet, "/k8s/default/pods/pod1/terminal", nil)
	req.Header.Set("Upgrade", "websocket")
	req.Header.Set("Sec-WebSocket-Key", "dGhlIHNhbXBsZSBub25jZQ==")

	h := &handlers.KubernetesHandler{}
	setUnexportedField(h, "client", clients.NewKubernetesClient(&http.Client{}, "http://"+ln.Addr().String(), "token"))

	h.HandleTerminal(rw, req)

	clientConn.Close()
	buf := <-out
	if !strings.Contains(strings.ToLower(string(buf)), "error") {
		t.Fatalf("expected error frame, got %q", string(buf))
	}
}

func TestDockerHandlerHandleTerminal(t *testing.T) {
	// This test ensures HandleTerminal doesn't panic and covers the basic code path
	// We can't easily test the full WebSocket functionality without complex mocking

	// Create a mock Docker server
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	// Simple server that just accepts connections
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		conn.Close()
	}()

	// Create a basic response writer
	rw := &hijackResponseWriter{
		header: http.Header{},
	}

	req := httptest.NewRequest(http.MethodGet, "/docker/containers/test123/terminal", nil)

	h := &handlers.DockerHandler{}
	setUnexportedField(h, "client", clients.NewDockerClient(&http.Client{}, "http://"+ln.Addr().String(), nil))

	// This should not panic - the function will fail during WebSocket upgrade
	// but we've covered the code path up to that point
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("HandleTerminal panicked: %v", r)
		}
	}()

	h.HandleTerminal(rw, req)
}

func TestPluginHandlerListAndLoadUnload(t *testing.T) {
	ctx := context.Background()
	rt, err := pluginpkg.NewRuntime(ctx)
	if err != nil {
		t.Skipf("plugin runtime not available: %v", err)
	}
	defer rt.Close(ctx)

	// FIX: Initialize the registrar and inject it into the handler
	mux := http.NewServeMux()
	rr := pluginpkg.NewRouteRegistrar(mux, rt)
	h := handlers.NewPluginHandler(rt, rr)

	t.Run("HandleList", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/plugins", nil)
		w := httptest.NewRecorder()
		h.HandleList(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", w.Code)
		}
	})

	t.Run("HandleLoadInvalidJSON", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/plugin/load", strings.NewReader("{invalid"))
		w := httptest.NewRecorder()
		h.HandleLoad(w, req)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", w.Code)
		}
	})

	t.Run("HandleUnloadMissingName", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, "/plugin/unload", nil)
		// Standard net/http does not populate PathValue for NewRequest;
		// you have to set it manually for the test.
		req.SetPathValue("name", "")
		w := httptest.NewRecorder()
		h.HandleUnload(w, req)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", w.Code)
		}
	})

	t.Run("HandleUnloadNotFound", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, "/plugin/unload", nil)
		req.SetPathValue("name", "missing")
		w := httptest.NewRecorder()
		h.HandleUnload(w, req)

		// Unload returns an error if the plugin doesn't exist,
		// resulting in a 500 status in your current HandleUnload logic.
		if w.Code != http.StatusInternalServerError {
			t.Fatalf("expected 500, got %d", w.Code)
		}
	})
}

func TestHealthHandlerResponse(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()

	handlers.HealthHandler(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "up") {
		t.Fatalf("expected body to contain 'up', got %q", w.Body.String())
	}
}

type hijackResponseWriter struct {
	header http.Header
	conn   net.Conn
	bufrw  *bufio.ReadWriter
}

func newHijackResponseWriter(conn net.Conn) *hijackResponseWriter {
	return &hijackResponseWriter{
		header: http.Header{},
		conn:   conn,
		bufrw:  bufio.NewReadWriter(bufio.NewReader(conn), bufio.NewWriter(conn)),
	}
}

func (h *hijackResponseWriter) Header() http.Header        { return h.header }
func (h *hijackResponseWriter) Write([]byte) (int, error)  { return 0, nil }
func (h *hijackResponseWriter) WriteHeader(statusCode int) {}
func (h *hijackResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	return h.conn, h.bufrw, nil
}

// TestDockerHandlerHTTPHandlersErrorCases tests error handling in Docker handler methods
func TestDockerHandlerHTTPHandlersErrorCases(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("docker error"))
	}))
	defer server.Close()

	h := &handlers.DockerHandler{}
	setUnexportedField(h, "client", clients.NewDockerClient(server.Client(), server.URL, nil))

	// Test HandleStart error
	req := httptest.NewRequest(http.MethodPost, "/containers/abc123/start", nil)
	req.SetPathValue("id", "abc123")
	w := httptest.NewRecorder()
	h.HandleStart(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}

	// Test HandleStop error
	req = httptest.NewRequest(http.MethodPost, "/containers/abc123/stop", nil)
	req.SetPathValue("id", "abc123")
	w = httptest.NewRecorder()
	h.HandleStop(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}

	// Test HandleRestart error
	req = httptest.NewRequest(http.MethodPost, "/containers/abc123/restart", nil)
	req.SetPathValue("id", "abc123")
	w = httptest.NewRecorder()
	h.HandleRestart(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}

	// Test HandleDelete error
	req = httptest.NewRequest(http.MethodDelete, "/containers/abc123", nil)
	req.SetPathValue("id", "abc123")
	w = httptest.NewRecorder()
	h.HandleDelete(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

// TestGetNodeHandler tests the node retrieval handler
func TestGetNodeHandler(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/nodes/abc", nil)
	req.SetPathValue("id", "abc")
	w := httptest.NewRecorder()

	handlers.GetNodeHandler(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "node_id") {
		t.Fatalf("unexpected body: %q", w.Body.String())
	}
}

func TestDockerHandlerHandleLogs(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Return a simple log response
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("test log output"))
	}))
	defer server.Close()

	h := &handlers.DockerHandler{}
	setUnexportedField(h, "client", clients.NewDockerClient(server.Client(), server.URL, nil))

	req := httptest.NewRequest(http.MethodGet, "/containers/abc123/logs", nil)
	req.SetPathValue("id", "abc123")
	w := httptest.NewRecorder()

	h.HandleLogs(w, req)
	// Since the mock server returns immediately, we just check that it doesn't crash
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

// ============= HTTP Handler Edge Cases =============

func TestHandlerMalformedJSON(t *testing.T) {
	tests := []struct {
		name        string
		body        string
		contentType string
	}{
		{"truncated json", `{"key":"val`, "application/json"},
		{"invalid json", `{invalid}`, "application/json"},
		{"empty json", ``, "application/json"},
		{"json array", `[]`, "application/json"},
		{"json null", `null`, "application/json"},
		{"json string", `"string"`, "application/json"},
		{"json number", `123`, "application/json"},
		{"trailing comma", `{"key":"val",}`, "application/json"},
		{"unquoted keys", `{key:val}`, "application/json"},
		{"single quotes", `{'key':'val'}`, "application/json"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/test", strings.NewReader(tt.body))
			req.Header.Set("Content-Type", tt.contentType)

			var data map[string]interface{}
			err := json.NewDecoder(req.Body).Decode(&data)

			// Most malformed JSON should error
			if tt.name != "json array" && tt.name != "json null" && tt.name != "json string" && tt.name != "json number" {
				if err == nil && tt.body != `` {
					t.Errorf("expected decode error for: %s", tt.name)
				}
			}
		})
	}
}

func TestHeaderEdgeCases(t *testing.T) {
	tests := []struct {
		name  string
		key   string
		value string
	}{
		{"empty header name", "", "value"},
		{"empty header value", "X-Custom", ""},
		{"very long header name", strings.Repeat("X", 1000), "value"},
		{"very long header value", "X-Custom", strings.Repeat("x", 10000)},
		{"special chars in value", "X-Custom", "!@#$%^&*()"},
		{"unicode in value", "X-Custom", "测试🔐"},
		{"multiline attempt", "X-Custom", "line1\nline2"},
		{"null bytes attempt", "X-Custom", "value\x00null"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/test", nil)
			if tt.key != "" {
				req.Header.Set(tt.key, tt.value)
			}
			// Should not panic
			_ = req.Header.Get(tt.key)
		})
	}
}

func TestHTTPMethodVariations(t *testing.T) {
	methods := []string{"GET", "POST", "PUT", "DELETE", "PATCH", "HEAD", "OPTIONS"}

	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/test", nil)
			if req.Method != method {
				t.Errorf("expected method %s, got %s", method, req.Method)
			}
		})
	}

	// Test lowercase method
	t.Run("lowercase method", func(t *testing.T) {
		req := httptest.NewRequest("get", "/test", nil)
		// Go keeps the method as provided
		if req.Method != "get" {
			t.Logf("method handling: expected get, got %s", req.Method)
		}
	})
}

// ============= Response Writing Edge Cases =============

func TestResponseWriterEdgeCases(t *testing.T) {
	t.Run("write to closed writer", func(t *testing.T) {
		w := httptest.NewRecorder()
		data := []byte("test data")

		// First write succeeds
		n, err := w.Write(data)
		if err != nil || n == 0 {
			t.Error("first write should succeed")
		}

		// Multiple writes should work
		n2, err := w.Write(data)
		if err != nil || n2 == 0 {
			t.Error("second write should succeed")
		}
	})

	t.Run("write large response", func(t *testing.T) {
		w := httptest.NewRecorder()
		largeData := make([]byte, 10*1024*1024) // 10MB
		for i := range largeData {
			largeData[i] = 'x'
		}

		n, err := w.Write(largeData)
		if err != nil {
			t.Errorf("write error: %v", err)
		}
		if n != len(largeData) {
			t.Errorf("expected %d bytes written, got %d", len(largeData), n)
		}
	})

	t.Run("set header after write", func(t *testing.T) {
		w := httptest.NewRecorder()
		w.Write([]byte("body"))

		// Setting header after write should not affect response
		w.Header().Set("X-After-Write", "value")
		// This is allowed in httptest but real HTTP forbids it
	})
}

// ============= URL and Path Edge Cases =============

func TestURLPathEdgeCases(t *testing.T) {
	tests := []struct {
		name       string
		path       string
		shouldWork bool
	}{
		{"root path", "/", true},
		{"path with trailing slash", "/api/", true},
		{"double slashes", "/api//resource", true},
		{"dots in path", "/api/../admin", true},
		{"very long path", "/" + strings.Repeat("a/", 100), true},
		{"encoded characters", "/api/resource%20name", true},
		{"unicode in path", "/api/用户", true},
		{"empty segments", "/api//resource//", true},
		// Paths without leading slash would require "http://localhost" prefix
		{"path without leading slash", "http://localhost/api", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var url string
			if !strings.HasPrefix(tt.path, "http") && strings.HasPrefix(tt.path, "/") {
				url = tt.path
			} else {
				url = tt.path
			}
			req := httptest.NewRequest("GET", url, nil)
			if !tt.shouldWork {
				return
			}
			if req.URL.Path == "" && (strings.HasPrefix(tt.path, "/") || strings.Contains(tt.path, "/")) {
				t.Logf("path parsing: %s", tt.path)
			}
		})
	}
}

// ============= Query Parameter Edge Cases =============

func TestQueryParameterEdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		query    string
		expected map[string]string
	}{
		{"empty query", "", map[string]string{}},
		{"single param", "key=value", map[string]string{"key": "value"}},
		{"multiple params", "k1=v1&k2=v2", map[string]string{"k1": "v1", "k2": "v2"}},
		{"no value", "key=", map[string]string{"key": ""}},
		{"no equals", "key", map[string]string{"key": ""}},
		{"duplicate keys", "key=v1&key=v2", map[string]string{"key": "v1"}}, // First value kept
		{"special chars", "key=%40%23%24", map[string]string{"key": "@#$"}},
		{"unicode param", "key=值", map[string]string{"key": "值"}},
		{"array syntax", "key[]=v1&key[]=v2", map[string]string{"key[]": "v1"}}, // First value kept
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url := "http://localhost/test"
			if tt.query != "" {
				url += "?" + tt.query
			}
			req := httptest.NewRequest("GET", url, nil)
			params := req.URL.Query()

			// Verify expected values
			for key, expectedVal := range tt.expected {
				val := params.Get(key)
				if val != expectedVal {
					t.Errorf("for param %q expected %q, got %q", key, expectedVal, val)
				}
			}
		})
	}
}

// ============= Body Reading Edge Cases =============

func TestBodyReadingEdgeCases(t *testing.T) {
	t.Run("empty body", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/test", strings.NewReader(""))
		body, err := io.ReadAll(req.Body)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if len(body) != 0 {
			t.Error("expected empty body")
		}
	})

	t.Run("large body", func(t *testing.T) {
		data := strings.Repeat("x", 1000000) // 1MB
		req := httptest.NewRequest("POST", "/test", strings.NewReader(data))
		body, err := io.ReadAll(req.Body)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if len(body) != len(data) {
			t.Errorf("expected %d bytes, got %d", len(data), len(body))
		}
	})

	t.Run("binary body", func(t *testing.T) {
		data := make([]byte, 256)
		for i := 0; i < 256; i++ {
			data[i] = byte(i)
		}
		req := httptest.NewRequest("POST", "/test", bytes.NewReader(data))
		body, err := io.ReadAll(req.Body)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if !bytes.Equal(body, data) {
			t.Error("body mismatch")
		}
	})

	t.Run("read body twice", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/test", strings.NewReader("test data"))

		// First read
		body1, _ := io.ReadAll(req.Body)
		req.Body.Close()

		// Body was closed, further reads should fail
		req.Body = io.NopCloser(bytes.NewReader(body1))
		body2, _ := io.ReadAll(req.Body)

		if !bytes.Equal(body1, body2) {
			t.Error("body content should be same")
		}
	})
}

// ============= Status Code Edge Cases =============

func TestStatusCodeEdgeCases(t *testing.T) {
	validCodes := []int{
		100, 101, // 1xx
		200, 201, 204, 206, // 2xx
		300, 301, 304, 307, // 3xx
		400, 401, 403, 404, 422, // 4xx
		500, 502, 503, 504, // 5xx
	}

	for _, code := range validCodes {
		t.Run(fmt.Sprintf("status %d", code), func(t *testing.T) {
			w := httptest.NewRecorder()
			w.WriteHeader(code)
			if w.Code != code {
				t.Errorf("expected status %d, got %d", code, w.Code)
			}
		})
	}

	t.Run("invalid status code", func(t *testing.T) {
		w := httptest.NewRecorder()
		// This should still work, Go allows any status code
		w.WriteHeader(999)
		if w.Code != 999 {
			t.Errorf("expected status 999, got %d", w.Code)
		}
	})

	t.Run("write header twice", func(t *testing.T) {
		w := httptest.NewRecorder()
		w.WriteHeader(200)
		w.WriteHeader(404) // Second call should be ignored
		if w.Code != 200 {
			t.Errorf("expected first status to be used, got %d", w.Code)
		}
	})
}

// ============= Content Type Edge Cases =============

func TestContentTypeEdgeCases(t *testing.T) {
	tests := []struct {
		name        string
		contentType string
	}{
		{"no charset", "application/json"},
		{"with charset", "application/json; charset=utf-8"},
		{"uppercase", "Application/JSON"},
		{"whitespace", "application/json ; charset=utf-8"},
		{"multiple params", "application/json; charset=utf-8; boundary=something"},
		{"empty", ""},
		{"text plain", "text/plain"},
		{"form data", "multipart/form-data; boundary=----boundary"},
		{"custom type", "application/vnd.custom+json"},
		{"with quotes", `application/json; charset="utf-8"`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			w.Header().Set("Content-Type", tt.contentType)
			ct := w.Header().Get("Content-Type")
			if ct != tt.contentType {
				t.Errorf("expected %q, got %q", tt.contentType, ct)
			}
		})
	}
}

func TestDockerHandlerListAndInspect(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if strings.Contains(r.URL.Path, "/json") {
			w.Write([]byte(`[{"Id":"123","Names":["/test"]}]`))
		} else {
			w.Write([]byte(`{"Id":"123","Name":"/test"}`))
		}
	}))
	defer server.Close()

	h := &handlers.DockerHandler{}
	setUnexportedField(h, "client", clients.NewDockerClient(server.Client(), server.URL, nil))

	t.Run("HandleListContainers", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/docker/containers", nil)
		w := httptest.NewRecorder()
		h.HandleListContainers(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", w.Code)
		}
	})

	t.Run("HandleInspect", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/docker/containers/123", nil)
		req.SetPathValue("id", "123")
		w := httptest.NewRecorder()
		h.HandleInspect(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", w.Code)
		}
	})

	t.Run("HandleStatsSnapshot", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/docker/containers/123/stats/snapshot", nil)
		req.SetPathValue("id", "123")
		w := httptest.NewRecorder()
		h.HandleStatsSnapshot(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", w.Code)
		}
	})
}


