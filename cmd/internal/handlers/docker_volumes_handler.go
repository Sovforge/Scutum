package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"scutum/cmd/internal/clients"
	"scutum/cmd/internal/models"
	"scutum/cmd/internal/store"
)

// ── Local (this node) endpoints — always act on the Docker daemon this
// process is talking to, and support the same X-Target-Node header proxy
// every other docker_handeler.go handler does. ─────────────────────────

// volumeUsage calls /system/df once and returns RefCount/Size keyed by
// volume name — the plain /volumes list endpoint doesn't carry usage data.
func (h *DockerHandler) volumeUsage(ctx context.Context) (map[string]struct {
	RefCount int64
	Size     int64
}, error) {
	var df struct {
		Volumes []struct {
			Name      string `json:"Name"`
			UsageData struct {
				RefCount int64 `json:"RefCount"`
				Size     int64 `json:"Size"`
			} `json:"UsageData"`
		} `json:"Volumes"`
	}
	if err := h.client.Do("GET", "/system/df?verbose=0", nil, &df); err != nil {
		return nil, err
	}
	usage := make(map[string]struct {
		RefCount int64
		Size     int64
	}, len(df.Volumes))
	for _, v := range df.Volumes {
		usage[v.Name] = struct {
			RefCount int64
			Size     int64
		}{RefCount: v.UsageData.RefCount, Size: v.UsageData.Size}
	}
	return usage, nil
}

// HandleListVolumes — GET /docker/volumes.
func (h *DockerHandler) HandleListVolumes(w http.ResponseWriter, r *http.Request) {
	if proxyRequest(w, r, nil, h.nodeStore) {
		return
	}

	var raw struct {
		Volumes []models.Volume `json:"Volumes"`
	}
	if err := h.client.Do("GET", "/volumes", nil, &raw); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	usage, err := h.volumeUsage(r.Context())
	if err != nil {
		// Usage data is an enrichment, not core to listing volumes — degrade
		// to RefCount/Size left at zero rather than failing the whole list.
		usage = nil
	}

	volumes := raw.Volumes
	if volumes == nil {
		volumes = []models.Volume{}
	}
	for i := range volumes {
		if u, ok := usage[volumes[i].Name]; ok {
			volumes[i].RefCount = u.RefCount
			volumes[i].Size = u.Size
			volumes[i].InUse = u.RefCount > 0
		}
	}
	writeJSON(w, http.StatusOK, volumes)
}

// HandleInspectVolume — GET /docker/volumes/{name}.
func (h *DockerHandler) HandleInspectVolume(w http.ResponseWriter, r *http.Request) {
	if proxyRequest(w, r, nil, h.nodeStore) {
		return
	}
	name := r.PathValue("name")
	if name == "" {
		http.Error(w, "volume name is required", http.StatusBadRequest)
		return
	}
	var volume models.Volume
	if err := h.client.Do("GET", "/volumes/"+url.PathEscape(name), nil, &volume); err != nil {
		writeDockerClientError(w, err)
		return
	}
	if usage, err := h.volumeUsage(r.Context()); err == nil {
		if u, ok := usage[volume.Name]; ok {
			volume.RefCount = u.RefCount
			volume.Size = u.Size
			volume.InUse = u.RefCount > 0
		}
	}
	writeJSON(w, http.StatusOK, volume)
}

// HandleRemoveVolume — DELETE /docker/volumes/{name}. The Docker Engine API
// already returns 409 for an in-use volume (force=false, the default) — we
// just propagate its real status code instead of collapsing it to a
// generic 500, so the frontend can show a clear "volume is in use" message.
func (h *DockerHandler) HandleRemoveVolume(w http.ResponseWriter, r *http.Request) {
	if proxyRequest(w, r, nil, h.nodeStore) {
		return
	}
	name := r.PathValue("name")
	if name == "" {
		http.Error(w, "volume name is required", http.StatusBadRequest)
		return
	}
	if err := h.client.Do("DELETE", "/volumes/"+url.PathEscape(name), nil, nil); err != nil {
		writeDockerClientError(w, err)
		return
	}
	audit("VOLUME_REMOVED", r, "volume", name)
	w.WriteHeader(http.StatusNoContent)
}

// HandlePruneVolumes — POST /docker/volumes/prune. The Docker Engine API's
// own default (no filters) only removes *anonymous* unused volumes — a
// named-but-unused volume like one created via `docker volume create` is
// left alone unless the "all" filter is set. The issue asks for "remove
// all unused volumes", so this always passes filters={"all":["true"]},
// matching `docker volume prune --all` rather than the bare CLI default.
func (h *DockerHandler) HandlePruneVolumes(w http.ResponseWriter, r *http.Request) {
	if proxyRequest(w, r, nil, h.nodeStore) {
		return
	}
	var result models.VolumePruneResult
	path := `/volumes/prune?filters=` + url.QueryEscape(`{"all":["true"]}`)
	if err := h.client.Do("POST", path, nil, &result); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	audit("VOLUMES_PRUNED", r, "count", len(result.VolumesDeleted), "space_reclaimed", result.SpaceReclaimed)
	writeJSON(w, http.StatusOK, result)
}

// writeDockerClientError propagates a Docker Engine API error's real status
// code (e.g. 404 not found, 409 in use) instead of collapsing every error to
// a generic 500.
func writeDockerClientError(w http.ResponseWriter, err error) {
	var apiErr *clients.APIError
	if errors.As(err, &apiErr) {
		http.Error(w, apiErr.Body, apiErr.StatusCode)
		return
	}
	http.Error(w, err.Error(), http.StatusInternalServerError)
}

// ── Node-targeted endpoints — GET|DELETE|POST /nodes/{id}/volumes... Looks
// up {id}; the hub handles the request itself, any other node gets it
// relayed over the existing HMAC-signed hub-to-node channel. This is a
// second way to reach the same local endpoints above (by node ID in the
// path, rather than X-Target-Node header + whichever node the topbar
// switcher has selected) — handy for a specific row action on the Nodes
// page, or for scripting against a known node ID. ──────────────────────

// relayToNode looks up {id}; if it's not the hub, relays the request to
// localPath on that node and writes the response, returning true (caller
// should return immediately). Returns false when {id} is the hub itself —
// the caller should handle the request locally.
func (h *DockerHandler) relayToNode(w http.ResponseWriter, r *http.Request, body []byte, localPath string) bool {
	id := r.PathValue("id")
	node, err := h.nodeStore.GetNode(r.Context(), id)
	if err != nil {
		http.Error(w, "node not found", http.StatusNotFound)
		return true
	}
	if node.Type == "hub" {
		return false
	}
	out, err := relayDockerRequest(r.Context(), node, r.Method, localPath, body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return true
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(out) //nolint:errcheck
	return true
}

// relayDockerRequest HMAC-signs and forwards a request to a specific path on
// node — distinct from proxyToNode (proxy.go), which always relays to the
// *same* path the incoming request used; here the hub-facing path
// (/nodes/{id}/volumes) and the node-local path (/docker/volumes) differ.
func relayDockerRequest(ctx context.Context, node store.NodeRecord, method, path string, body []byte) ([]byte, error) {
	host := normaliseAddress(node.Address)
	target := "https://" + host + "/api" + path
	var bodyReader io.Reader
	if body != nil {
		bodyReader = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, target, bodyReader)
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if len(proxyHMACKey) > 0 {
		ts := strconv.FormatInt(time.Now().Unix(), 10)
		sig := hubRequestSig(proxyHMACKey, ts, method, "/api"+path, body)
		req.Header.Set("X-Scutum-Hub-Ts", ts)
		req.Header.Set("X-Scutum-Hub-Sig", sig)
	}
	client := &http.Client{Timeout: 3 * time.Minute, Transport: &http.Transport{TLSClientConfig: proxyTLSConfig}}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("node returned status %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}
	return respBody, nil
}

func (h *DockerHandler) HandleListVolumesForNode(w http.ResponseWriter, r *http.Request) {
	if h.relayToNode(w, r, nil, "/docker/volumes") {
		return
	}
	h.HandleListVolumes(w, r)
}

func (h *DockerHandler) HandleInspectVolumeForNode(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if h.relayToNode(w, r, nil, "/docker/volumes/"+url.PathEscape(name)) {
		return
	}
	h.HandleInspectVolume(w, r)
}

func (h *DockerHandler) HandleRemoveVolumeForNode(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if h.relayToNode(w, r, nil, "/docker/volumes/"+url.PathEscape(name)) {
		return
	}
	h.HandleRemoveVolume(w, r)
}

func (h *DockerHandler) HandlePruneVolumesForNode(w http.ResponseWriter, r *http.Request) {
	if h.relayToNode(w, r, nil, "/docker/volumes/prune") {
		return
	}
	h.HandlePruneVolumes(w, r)
}

// ── Group-targeted endpoint — GET /groups/{id}/volumes: every member
// node's volume list, side by side. ─────────────────────────────────────

type nodeVolumeList struct {
	NodeID   string          `json:"node_id"`
	NodeName string          `json:"node_name"`
	Volumes  []models.Volume `json:"volumes,omitempty"`
	Error    string          `json:"error,omitempty"`
}

func (h *DockerHandler) volumesForNode(ctx context.Context, n store.NodeRecord) ([]models.Volume, error) {
	if n.Type == "hub" {
		var raw struct {
			Volumes []models.Volume `json:"Volumes"`
		}
		if err := h.client.Do("GET", "/volumes", nil, &raw); err != nil {
			return nil, err
		}
		if usage, err := h.volumeUsage(ctx); err == nil {
			for i := range raw.Volumes {
				if u, ok := usage[raw.Volumes[i].Name]; ok {
					raw.Volumes[i].RefCount = u.RefCount
					raw.Volumes[i].Size = u.Size
					raw.Volumes[i].InUse = u.RefCount > 0
				}
			}
		}
		return raw.Volumes, nil
	}
	out, err := relayDockerRequest(ctx, n, http.MethodGet, "/docker/volumes", nil)
	if err != nil {
		return nil, err
	}
	var volumes []models.Volume
	if err := json.Unmarshal(out, &volumes); err != nil {
		return nil, err
	}
	return volumes, nil
}

// HandleListVolumesForGroup — GET /groups/{id}/volumes. The frontend builds
// the per-node volume table from this; the backend just reports what's
// actually on each node.
func (h *DockerHandler) HandleListVolumesForGroup(w http.ResponseWriter, r *http.Request) {
	if h.groupStore == nil {
		http.Error(w, "group volume operations are not configured", http.StatusInternalServerError)
		return
	}
	nodes, err := h.groupStore.ListNodesInGroup(r.Context(), r.PathValue("id"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	results := make([]nodeVolumeList, 0, len(nodes))
	for _, n := range nodes {
		entry := nodeVolumeList{NodeID: n.ID, NodeName: n.Name}
		volumes, err := h.volumesForNode(r.Context(), n)
		if err != nil {
			entry.Error = err.Error()
		} else {
			entry.Volumes = volumes
		}
		results = append(results, entry)
	}
	writeJSON(w, http.StatusOK, results)
}
