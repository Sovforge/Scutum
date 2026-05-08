package tests

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"strings"
	"scutum/cmd/internal/handlers"
	"scutum/cmd/internal/clients"
)

func TestKubernetesHandler(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.Contains(r.URL.Path, "/log") {
			w.Write([]byte("2023-05-01T10:00:00Z line 1\n2023-05-01T10:00:01Z line 2\n"))
			return
		}
		if strings.Contains(r.URL.Path, "/pods") || strings.Contains(r.URL.Path, "/namespaces") || strings.Contains(r.URL.Path, "/nodes") || strings.Contains(r.URL.Path, "/deployments") {
			w.Write([]byte(`{"items":[{"status":{"phase":"Running","readyReplicas":1,"replicas":1}}]}`))
			return
		}
		w.Write([]byte(`{"status":"ok"}`))
	}))
	defer server.Close()

	// Using reflection or a helper to set unexported client if needed, 
	// but here we can just create a new handler and inject a client if possible.
	// NewKubernetesHandler uses in-cluster config by default.
	// For testing, we can use a custom constructor or a helper.
	h := handlers.NewKubernetesHandler(nil)
	setUnexportedField(h, "client", clients.NewKubernetesClient(server.Client(), server.URL, ""))

	t.Run("HandleGetPod", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/k8s/ns/pods/name", nil)
		req.SetPathValue("ns", "default")
		req.SetPathValue("name", "mypod")
		w := httptest.NewRecorder()
		h.HandleGetPod(w, req)
		if w.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", w.Code)
		}
	})

	t.Run("HandleListAllPods", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/k8s/pods", nil)
		w := httptest.NewRecorder()
		h.HandleListAllPods(w, req)
		if w.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", w.Code)
		}
	})

	t.Run("HandlePodLogsJSON", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/k8s/ns/pods/name/logs/json", nil)
		req.SetPathValue("ns", "default")
		req.SetPathValue("name", "mypod")
		w := httptest.NewRecorder()
		h.HandlePodLogsJSON(w, req)
		if w.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", w.Code)
		}
	})

	t.Run("HandleK8sSummary", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/k8s/summary", nil)
		w := httptest.NewRecorder()
		h.HandleK8sSummary(w, req)
		if w.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", w.Code)
		}
		var summary map[string]int
		json.NewDecoder(w.Body).Decode(&summary)
		if summary["pods"] != 1 {
			t.Errorf("expected 1 pod, got %d", summary["pods"])
		}
	})
}
