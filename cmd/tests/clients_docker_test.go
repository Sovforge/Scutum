package tests

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"scutum/cmd/internal/clients"
)

func TestDockerClient(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1.41/containers/json" {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`[{"Id":"123","Names":["/test"]}]`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	c := clients.NewDockerClient(server.Client(), server.URL, nil)

	t.Run("Do", func(t *testing.T) {
		var containers []map[string]interface{}
		err := c.Do("GET", "/v1.41/containers/json", nil, &containers)
		if err != nil {
			t.Fatalf("Do failed: %v", err)
		}
		if len(containers) != 1 {
			t.Errorf("expected 1 container, got %d", len(containers))
		}
		if containers[0]["Id"] != "123" {
			t.Errorf("id mismatch: %v", containers[0]["Id"])
		}
	})

	t.Run("Do-Error", func(t *testing.T) {
		err := c.Do("GET", "/notfound", nil, nil)
		if err == nil {
			t.Error("expected error for 404")
		}
	})
}
