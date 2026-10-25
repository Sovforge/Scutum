package tests

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"scutum/cmd/internal/clients"
	"scutum/cmd/internal/handlers"
)

// fakeDockerDaemon serves just enough of the Docker Engine API for the
// volume handlers: a list with one in-use and one unused volume, an inspect
// endpoint, a delete that 409s for the in-use volume (mirroring the real
// daemon's behavior), and a prune endpoint.
func fakeDockerDaemon(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == "GET" && r.URL.Path == "/volumes":
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{ //nolint:errcheck
				"Volumes": []map[string]any{
					{"Name": "used-vol", "Driver": "local", "Mountpoint": "/var/lib/docker/volumes/used-vol/_data", "CreatedAt": "2026-01-01T00:00:00Z", "Scope": "local"},
					{"Name": "unused-vol", "Driver": "local", "Mountpoint": "/var/lib/docker/volumes/unused-vol/_data", "CreatedAt": "2026-01-02T00:00:00Z", "Scope": "local"},
				},
			})
		case r.Method == "GET" && r.URL.Path == "/system/df":
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{ //nolint:errcheck
				"Volumes": []map[string]any{
					{"Name": "used-vol", "UsageData": map[string]any{"RefCount": 1, "Size": 1024}},
					{"Name": "unused-vol", "UsageData": map[string]any{"RefCount": 0, "Size": 2048}},
				},
			})
		case r.Method == "GET" && r.URL.Path == "/volumes/used-vol":
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{ //nolint:errcheck
				"Name": "used-vol", "Driver": "local", "Mountpoint": "/var/lib/docker/volumes/used-vol/_data", "Scope": "local",
			})
		case r.Method == "DELETE" && r.URL.Path == "/volumes/used-vol":
			http.Error(w, "volume is in use", http.StatusConflict)
		case r.Method == "DELETE" && r.URL.Path == "/volumes/unused-vol":
			w.WriteHeader(http.StatusNoContent)
		case r.Method == "POST" && r.URL.Path == "/volumes/prune":
			// Mirror the real Engine API: only actually prune named-but-unused
			// volumes (not just anonymous ones) when filters={"all":["true"]}
			// is present — a request missing it gets nothing deleted, same as
			// `docker volume prune` without `--all`. This exists to catch a
			// regression back to that narrower, surprising default.
			if r.URL.Query().Get("filters") != `{"all":["true"]}` {
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(map[string]any{"VolumesDeleted": []string{}, "SpaceReclaimed": 0}) //nolint:errcheck
				return
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{ //nolint:errcheck
				"VolumesDeleted": []string{"unused-vol"}, "SpaceReclaimed": 2048,
			})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
}

func newTestDockerHandler(t *testing.T, server *httptest.Server) *handlers.DockerHandler {
	t.Helper()
	h := handlers.NewDockerHandler(&mockNodeProxyStore{}, nil)
	h.SetClient(clients.NewDockerClient(server.Client(), server.URL, nil))
	return h
}

func TestHandleListVolumesMergesUsageData(t *testing.T) {
	server := fakeDockerDaemon(t)
	defer server.Close()
	h := newTestDockerHandler(t, server)

	req := httptest.NewRequest("GET", "/docker/volumes", nil)
	rec := httptest.NewRecorder()
	h.HandleListVolumes(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var volumes []struct {
		Name     string
		InUse    bool
		RefCount int64
		Size     int64
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &volumes); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(volumes) != 2 {
		t.Fatalf("expected 2 volumes, got %d", len(volumes))
	}
	byName := map[string]bool{}
	for _, v := range volumes {
		byName[v.Name] = v.InUse
		if v.Name == "used-vol" && (!v.InUse || v.RefCount != 1 || v.Size != 1024) {
			t.Errorf("used-vol usage data not merged correctly: %+v", v)
		}
		if v.Name == "unused-vol" && (v.InUse || v.RefCount != 0) {
			t.Errorf("unused-vol should not be marked in-use: %+v", v)
		}
	}
	if _, ok := byName["used-vol"]; !ok {
		t.Error("used-vol missing from list")
	}
}

func TestHandleRemoveVolumeInUsePropagates409(t *testing.T) {
	server := fakeDockerDaemon(t)
	defer server.Close()
	h := newTestDockerHandler(t, server)

	req := httptest.NewRequest("DELETE", "/docker/volumes/used-vol", nil)
	req.SetPathValue("name", "used-vol")
	rec := httptest.NewRecorder()
	h.HandleRemoveVolume(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409 for in-use volume, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestHandleRemoveVolumeUnusedSucceeds(t *testing.T) {
	server := fakeDockerDaemon(t)
	defer server.Close()
	h := newTestDockerHandler(t, server)

	req := httptest.NewRequest("DELETE", "/docker/volumes/unused-vol", nil)
	req.SetPathValue("name", "unused-vol")
	rec := httptest.NewRecorder()
	h.HandleRemoveVolume(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestHandlePruneVolumes(t *testing.T) {
	server := fakeDockerDaemon(t)
	defer server.Close()
	h := newTestDockerHandler(t, server)

	req := httptest.NewRequest("POST", "/docker/volumes/prune", nil)
	rec := httptest.NewRecorder()
	h.HandlePruneVolumes(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var result struct {
		VolumesDeleted []string
		SpaceReclaimed int64
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(result.VolumesDeleted) != 1 || result.VolumesDeleted[0] != "unused-vol" {
		t.Errorf("unexpected deleted volumes: %+v", result.VolumesDeleted)
	}
	if result.SpaceReclaimed != 2048 {
		t.Errorf("expected 2048 bytes reclaimed, got %d", result.SpaceReclaimed)
	}
}
