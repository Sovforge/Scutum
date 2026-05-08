package tests

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"scutum/cmd/internal/clients"
)

type dockerResponse struct {
	State string `json:"state"`
}

func TestDockerClientDoSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/containers/create" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(dockerResponse{State: "created"})
	}))
	defer server.Close()

	client := clients.NewDockerClient(server.Client(), server.URL, nil)
	var out dockerResponse
	if err := client.Do(http.MethodPost, "/containers/create", map[string]string{"foo": "bar"}, &out); err != nil {
		t.Fatal(err)
	}
	if out.State != "created" {
		t.Fatalf("unexpected response: %#v", out)
	}
}

func TestDockerClientDoErrorStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("bad request"))
	}))
	defer server.Close()

	client := clients.NewDockerClient(server.Client(), server.URL, nil)
	err := client.Do(http.MethodGet, "/error", nil, nil)
	if err == nil || err.Error() == "" {
		t.Fatal("expected error on bad status")
	}
}

func TestDockerClientDoStreamSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("streaming-data"))
	}))
	defer server.Close()

	client := clients.NewDockerClient(server.Client(), server.URL, nil)
	stream, err := client.DoStream(http.MethodGet, "/stream", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer stream.Close()
	body, err := io.ReadAll(stream)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "streaming-data" {
		t.Fatalf("unexpected body: %q", string(body))
	}
}

func TestKubernetesClientDoSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer token-123" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}))
	defer server.Close()

	client := clients.NewKubernetesClient(server.Client(), server.URL, "token-123")
	var out map[string]string
	if err := client.Do(http.MethodGet, "/api/v1/nodes", nil, &out); err != nil {
		t.Fatal(err)
	}
	if out["status"] != "ok" {
		t.Fatalf("unexpected response: %#v", out)
	}
}

func TestKubernetesClientDoStreamError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client := clients.NewKubernetesClient(server.Client(), server.URL, "token-123")
	_, err := client.DoStream(http.MethodGet, "/watch", nil)
	if err == nil {
		t.Fatal("expected error from DoStream")
	}
}
