package handlers

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"scutum/cmd/internal/models"
	"scutum/cmd/internal/store"
)

// ── Local (this node) endpoints — always act on the Docker daemon this
// process is talking to, and support the same X-Target-Node header proxy
// every other docker_handeler.go handler does. ─────────────────────────

// HandleListImages — GET /docker/images.
func (h *DockerHandler) HandleListImages(w http.ResponseWriter, r *http.Request) {
	if proxyRequest(w, r, nil, h.nodeStore) {
		return
	}
	var images []models.Image
	if err := h.client.Do("GET", "/images/json?all=false", nil, &images); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if images == nil {
		images = []models.Image{}
	}
	writeJSON(w, http.StatusOK, images)
}

// HandlePullImage — POST /docker/images/pull.
func (h *DockerHandler) HandlePullImage(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 64*1024))
	if err != nil {
		http.Error(w, "read body: "+err.Error(), http.StatusBadRequest)
		return
	}
	if proxyRequest(w, r, body, h.nodeStore) {
		return
	}

	var req models.PullImageRequest
	if err := json.Unmarshal(body, &req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.Image == "" {
		http.Error(w, "image is required", http.StatusBadRequest)
		return
	}

	password, err := h.resolvePullPassword(r, req.Password)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	result := h.pullImage(r.Context(), req.Image, req.Username, password)
	status := http.StatusOK
	if !result.Success {
		status = http.StatusBadGateway
	}
	audit("IMAGE_PULLED", r, "image", req.Image, "success", result.Success)
	writeJSON(w, status, result)
}

// resolvePullPassword treats a "secret://name" password as a reference into
// the secrets vault (resolved here, never persisted) — anything else is
// used literally. Mirrors the secret:// convention already used for
// Compose/Kubernetes env values, just for a single field instead of a map.
func (h *DockerHandler) resolvePullPassword(r *http.Request, password string) (string, error) {
	if !strings.HasPrefix(password, "secret://") {
		return password, nil
	}
	name := strings.TrimPrefix(password, "secret://")
	val, err := h.secrets.GetAppSecretValue(r.Context(), name)
	if err != nil {
		return "", fmt.Errorf("resolve registry password: %w", err)
	}
	audit("SECRET_ACCESSED", r, "names", name, "context", "image_pull")
	return string(val), nil
}

// pullImage drives the Docker Engine's streamed image-pull API and distills
// the newline-delimited progress events into a single success/failure
// result. The existing pull-on-404 fallback in PostDeploy discards this
// stream entirely (and so silently ignores pull failures); this scans every
// line for an "error"/"errorDetail" field instead.
func (h *DockerHandler) pullImage(ctx context.Context, image, username, password string) models.PullImageResult {
	result := models.PullImageResult{Image: image}

	repo, tag := SplitImageRef(image)
	path := fmt.Sprintf("/images/create?fromImage=%s&tag=%s", url.QueryEscape(repo), url.QueryEscape(tag))

	var headers map[string]string
	if username != "" || password != "" {
		authJSON, _ := json.Marshal(map[string]string{"username": username, "password": password})
		headers = map[string]string{"X-Registry-Auth": base64.URLEncoding.EncodeToString(authJSON)}
	}

	stream, err := h.client.DoStreamWithHeaders("POST", path, nil, headers)
	if err != nil {
		result.Error = err.Error()
		return result
	}
	defer stream.Close()

	scanner := bufio.NewScanner(stream)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	var lastErr string
	for scanner.Scan() {
		var evt struct {
			Error       string `json:"error"`
			ErrorDetail struct {
				Message string `json:"message"`
			} `json:"errorDetail"`
		}
		if err := json.Unmarshal(scanner.Bytes(), &evt); err == nil {
			if evt.Error != "" {
				lastErr = evt.Error
			} else if evt.ErrorDetail.Message != "" {
				lastErr = evt.ErrorDetail.Message
			}
		}
	}
	if lastErr != "" {
		result.Error = lastErr
		return result
	}
	result.Success = true
	return result
}

// SplitImageRef splits "image[:tag]" the way Docker itself does: the tag
// separator is the last ":" in the reference, but only when it appears
// after the last "/" — otherwise it's a registry port
// ("myregistry:5000/nginx"), not a tag.
func SplitImageRef(ref string) (repo, tag string) {
	tag = "latest"
	lastSlash := strings.LastIndex(ref, "/")
	lastColon := strings.LastIndex(ref, ":")
	if lastColon > lastSlash {
		return ref[:lastColon], ref[lastColon+1:]
	}
	return ref, tag
}

// HandleRemoveImage — DELETE /docker/images/{imageId}.
func (h *DockerHandler) HandleRemoveImage(w http.ResponseWriter, r *http.Request) {
	if proxyRequest(w, r, nil, h.nodeStore) {
		return
	}
	imageID := r.PathValue("imageId")
	if imageID == "" {
		http.Error(w, "image id is required", http.StatusBadRequest)
		return
	}
	var result []models.ImageDeleteEntry
	path := fmt.Sprintf("/images/%s?force=true", url.PathEscape(imageID))
	if err := h.client.Do("DELETE", path, nil, &result); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	audit("IMAGE_REMOVED", r, "image_id", imageID)
	writeJSON(w, http.StatusOK, result)
}

// HandlePruneImages — POST /docker/images/prune. No filters means the
// Docker Engine API's own default: dangling (untagged, unreferenced)
// images only — exactly what the issue asks for.
func (h *DockerHandler) HandlePruneImages(w http.ResponseWriter, r *http.Request) {
	if proxyRequest(w, r, nil, h.nodeStore) {
		return
	}
	var result models.ImagePruneResult
	if err := h.client.Do("POST", "/images/prune", nil, &result); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	audit("IMAGES_PRUNED", r, "count", len(result.ImagesDeleted), "space_reclaimed", result.SpaceReclaimed)
	writeJSON(w, http.StatusOK, result)
}

// ── Node-targeted endpoints — GET|POST|DELETE /nodes/{id}/images... Looks
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

// relayDockerRequest HMAC-signs and forwards a request to a specific path
// on node — distinct from proxyToNode (proxy.go), which always relays to
// the *same* path the incoming request used; here the hub-facing path
// (/nodes/{id}/images) and the node-local path (/docker/images) differ.
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

func (h *DockerHandler) HandleListImagesForNode(w http.ResponseWriter, r *http.Request) {
	if h.relayToNode(w, r, nil, "/docker/images") {
		return
	}
	h.HandleListImages(w, r)
}

func (h *DockerHandler) HandlePullImageForNode(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 64*1024))
	if err != nil {
		http.Error(w, "read body: "+err.Error(), http.StatusBadRequest)
		return
	}
	if h.relayToNode(w, r, body, "/docker/images/pull") {
		return
	}
	r.Body = io.NopCloser(bytes.NewReader(body))
	h.HandlePullImage(w, r)
}

func (h *DockerHandler) HandleRemoveImageForNode(w http.ResponseWriter, r *http.Request) {
	imageID := r.PathValue("imageId")
	if h.relayToNode(w, r, nil, "/docker/images/"+url.PathEscape(imageID)) {
		return
	}
	h.HandleRemoveImage(w, r)
}

func (h *DockerHandler) HandlePruneImagesForNode(w http.ResponseWriter, r *http.Request) {
	if h.relayToNode(w, r, nil, "/docker/images/prune") {
		return
	}
	h.HandlePruneImages(w, r)
}

// ── Group-targeted endpoints — GET /groups/{id}/images (the "image diff"
// view: every node's image list, side by side) and POST
// /groups/{id}/images/pull (fan-out pull, one result per node). ────────

type nodeImageList struct {
	NodeID   string         `json:"node_id"`
	NodeName string         `json:"node_name"`
	Images   []models.Image `json:"images,omitempty"`
	Error    string         `json:"error,omitempty"`
}

func (h *DockerHandler) imagesForNode(ctx context.Context, n store.NodeRecord) ([]models.Image, error) {
	var images []models.Image
	if n.Type == "hub" {
		if err := h.client.Do("GET", "/images/json?all=false", nil, &images); err != nil {
			return nil, err
		}
		return images, nil
	}
	out, err := relayDockerRequest(ctx, n, http.MethodGet, "/docker/images", nil)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(out, &images); err != nil {
		return nil, err
	}
	return images, nil
}

// HandleListImagesForGroup — GET /groups/{id}/images. The frontend builds
// the node-vs-image diff table from this; the backend just reports what's
// actually on each node rather than computing the diff itself.
func (h *DockerHandler) HandleListImagesForGroup(w http.ResponseWriter, r *http.Request) {
	if h.groupStore == nil {
		http.Error(w, "group image operations are not configured", http.StatusInternalServerError)
		return
	}
	nodes, err := h.groupStore.ListNodesInGroup(r.Context(), r.PathValue("id"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	results := make([]nodeImageList, 0, len(nodes))
	for _, n := range nodes {
		entry := nodeImageList{NodeID: n.ID, NodeName: n.Name}
		images, err := h.imagesForNode(r.Context(), n)
		if err != nil {
			entry.Error = err.Error()
		} else {
			entry.Images = images
		}
		results = append(results, entry)
	}
	writeJSON(w, http.StatusOK, results)
}

type nodePullResult struct {
	NodeID   string                 `json:"node_id"`
	NodeName string                 `json:"node_name"`
	Result   models.PullImageResult `json:"result"`
}

// HandlePullImageForGroup — POST /groups/{id}/images/pull. Fans out
// sequentially, one result per node — a slow/unreachable node never blocks
// or fails the others, mirroring the file-distribution group fan-out.
func (h *DockerHandler) HandlePullImageForGroup(w http.ResponseWriter, r *http.Request) {
	if h.groupStore == nil {
		http.Error(w, "group image operations are not configured", http.StatusInternalServerError)
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, 64*1024))
	if err != nil {
		http.Error(w, "read body: "+err.Error(), http.StatusBadRequest)
		return
	}
	var req models.PullImageRequest
	if err := json.Unmarshal(body, &req); err != nil || req.Image == "" {
		http.Error(w, "image is required", http.StatusBadRequest)
		return
	}

	password, err := h.resolvePullPassword(r, req.Password)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	resolvedBody, _ := json.Marshal(models.PullImageRequest{Image: req.Image, Username: req.Username, Password: password})

	groupID := r.PathValue("id")
	nodes, err := h.groupStore.ListNodesInGroup(r.Context(), groupID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	results := make([]nodePullResult, 0, len(nodes))
	for _, n := range nodes {
		var result models.PullImageResult
		if n.Type == "hub" {
			result = h.pullImage(r.Context(), req.Image, req.Username, password)
		} else if out, relayErr := relayDockerRequest(r.Context(), n, http.MethodPost, "/docker/images/pull", resolvedBody); relayErr != nil {
			result = models.PullImageResult{Image: req.Image, Error: relayErr.Error()}
		} else {
			json.Unmarshal(out, &result) //nolint:errcheck
		}
		results = append(results, nodePullResult{NodeID: n.ID, NodeName: n.Name, Result: result})
	}

	audit("IMAGE_PULLED_GROUP", r, "group_id", groupID, "image", req.Image, "node_count", len(nodes))
	writeJSON(w, http.StatusOK, results)
}
