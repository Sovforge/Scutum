package handlers

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"time"

	"scutum/cmd/internal/clients"
	"scutum/cmd/internal/models"
	"scutum/cmd/internal/utils"
)

type KubernetesHandler struct {
	client    *clients.KubernetesClient
	nodeStore nodeProxyStore
}

func NewKubernetesHandler(ns nodeProxyStore) *KubernetesHandler {
	cfg, err := utils.GetInClusterConfig()
	if err != nil {
		cfg = &utils.KubernetesConfig{
			Host:       "http://127.0.0.1:8001",
			HTTPClient: &http.Client{},
		}
	}

	return &KubernetesHandler{
		client:    clients.NewKubernetesClient(cfg.HTTPClient, cfg.Host, cfg.Token),
		nodeStore: ns,
	}
}

func (h *KubernetesHandler) CreateDeployment(ns string, name string, image string) error {
	deploy := models.Deployment{
		KubernetesResurce: models.KubernetesResurce{
			APIVersion: "apps/v1",
			Kind:       "Deployment",
			Metadata:   models.Meta{Name: name},
		},
		Spec: models.DeploymentSpec{
			Replicas: 2,
			Selector: models.LabelSelector{
				MatchLabels: map[string]string{"app": name},
			},
			Template: models.PodTemplate{
				Metadata: models.Meta{Labels: map[string]string{"app": name}},
				Spec: models.PodSpec{
					Containers: []models.Container{
						{Name: "web", Image: image},
					},
				},
			},
		},
	}

	path := fmt.Sprintf("/apis/apps/v1/namespaces/%s/deployments", ns)
	return h.client.Do("POST", path, deploy, nil)
}

// GetPod retrieves a single pod's definition
func (h *KubernetesHandler) GetPod(ctx context.Context, ns, name string) (map[string]interface{}, error) {
	path := fmt.Sprintf("/api/v1/namespaces/%s/pods/%s", ns, name)
	var pod map[string]interface{}
	err := h.client.Do("GET", path, nil, &pod)
	return pod, err
}

// DeletePod removes a pod from the cluster
func (h *KubernetesHandler) DeletePod(ctx context.Context, ns, name string) error {
	path := fmt.Sprintf("/api/v1/namespaces/%s/pods/%s", ns, name)
	return h.client.Do("DELETE", path, nil, nil)
}

// WatchEvents streams cluster-wide events for monitoring
func (h *KubernetesHandler) WatchEvents(ctx context.Context) (io.ReadCloser, error) {
	path := "/api/v1/events?watch=true"
	return h.client.DoStream("GET", path, nil)
}

// ScaleDeployment changes the number of running pods
func (h *KubernetesHandler) ScaleDeployment(ctx context.Context, ns, name string, replicas int) error {
	path := fmt.Sprintf("/apis/apps/v1/namespaces/%s/deployments/%s", ns, name)

	var deploy models.Deployment
	if err := h.client.Do("GET", path, nil, &deploy); err != nil {
		return err
	}

	deploy.Spec.Replicas = replicas

	return h.client.Do("PUT", path, deploy, nil)
}

func (h *KubernetesHandler) RestartDeployment(ctx context.Context, ns, name string) error {
	path := fmt.Sprintf("/apis/apps/v1/namespaces/%s/deployments/%s", ns, name)

	var deploy models.Deployment
	h.client.Do("GET", path, nil, &deploy)

	if deploy.Spec.Template.Metadata.Annotations == nil {
		deploy.Spec.Template.Metadata.Annotations = make(map[string]string)
	}
	deploy.Spec.Template.Metadata.Annotations["kubectl.kubernetes.io/restartedAt"] = time.Now().Format(time.RFC3339)

	return h.client.Do("PUT", path, deploy, nil)
}

// HandleDeletePod removes the pod
func (h *KubernetesHandler) HandleDeletePod(w http.ResponseWriter, r *http.Request) {
	if proxyRequest(w, r, nil, h.nodeStore) {
		return
	}
	ns := r.PathValue("ns")
	name := r.PathValue("name")

	if err := h.DeletePod(r.Context(), ns, name); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// HandleLogs streams logs from a K8s pod
func (h *KubernetesHandler) HandleLogs(w http.ResponseWriter, r *http.Request) {
	if proxyRequest(w, r, nil, h.nodeStore) {
		return
	}
	ns := r.PathValue("ns")
	name := r.PathValue("name")

	path := fmt.Sprintf("/api/v1/namespaces/%s/pods/%s/log?follow=true", ns, name)
	stream, err := h.client.DoStream("GET", path, nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer stream.Close()

	w.Header().Set("Content-Type", "text/plain")
	io.Copy(w, stream)
}

// HandleDeploy handles POST /k8s/{ns}/deploy
func (h *KubernetesHandler) HandleDeploy(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 512*1024))
	if err != nil {
		http.Error(w, "read body: "+err.Error(), http.StatusBadRequest)
		return
	}
	if proxyRequest(w, r, body, h.nodeStore) {
		return
	}
	r.Body = io.NopCloser(bytes.NewReader(body))

	ns := r.PathValue("ns")
	var req struct {
		Name  string `json:"name"`
		Image string `json:"image"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if err := h.CreateDeployment(ns, req.Name, req.Image); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusCreated)
}

// HandleGetPod returns the full pod JSON from the Kubernetes API.
func (h *KubernetesHandler) HandleGetPod(w http.ResponseWriter, r *http.Request) {
	if proxyRequest(w, r, nil, h.nodeStore) {
		return
	}
	ns := r.PathValue("ns")
	name := r.PathValue("name")
	var result json.RawMessage
	if err := h.client.Do("GET", fmt.Sprintf("/api/v1/namespaces/%s/pods/%s", ns, name), nil, &result); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(result)
}

// HandlePodLogsJSON returns the last N log lines of a pod container as JSON.
func (h *KubernetesHandler) HandlePodLogsJSON(w http.ResponseWriter, r *http.Request) {
	if proxyRequest(w, r, nil, h.nodeStore) {
		return
	}
	ns := r.PathValue("ns")
	name := r.PathValue("name")
	container := r.URL.Query().Get("container")
	tail := r.URL.Query().Get("tail")
	if tail == "" {
		tail = "100"
	}
	path := fmt.Sprintf("/api/v1/namespaces/%s/pods/%s/log?tail=%s&timestamps=true", ns, name, tail)
	if container != "" {
		path += "&container=" + url.QueryEscape(container)
	}
	stream, err := h.client.DoStream("GET", path, nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer stream.Close()

	type logLine struct {
		TS  string `json:"ts"`
		Msg string `json:"msg"`
	}
	var lines []logLine
	sc := bufio.NewScanner(stream)
	for sc.Scan() {
		text := sc.Text()
		ts, msg := "", text
		if i := strings.IndexByte(text, ' '); i > 0 {
			ts, msg = text[:i], text[i+1:]
		}
		lines = append(lines, logLine{TS: ts, Msg: msg})
	}
	if lines == nil {
		lines = []logLine{}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(lines)
}

// HandleListAllPods lists all pods across every namespace.
func (h *KubernetesHandler) HandleListAllPods(w http.ResponseWriter, r *http.Request) {
	if proxyRequest(w, r, nil, h.nodeStore) {
		return
	}
	var result json.RawMessage
	if err := h.client.Do("GET", "/api/v1/pods", nil, &result); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(result)
}

// HandleListDeployments returns every Deployment across all namespaces —
// just enough (namespace, name, desired/ready replica counts) for the
// Deployments tab to list, scale, and restart. models.Deployment has no
// Status field (it's only ever used as an outgoing apply/scale payload), so
// this decodes into a local struct instead, same as HandleK8sSummary does.
func (h *KubernetesHandler) HandleListDeployments(w http.ResponseWriter, r *http.Request) {
	if proxyRequest(w, r, nil, h.nodeStore) {
		return
	}
	var deplList struct {
		Items []struct {
			Metadata struct {
				Name      string `json:"name"`
				Namespace string `json:"namespace"`
			} `json:"metadata"`
			Spec struct {
				Replicas int `json:"replicas"`
			} `json:"spec"`
			Status struct {
				Replicas      int `json:"replicas"`
				ReadyReplicas int `json:"readyReplicas"`
			} `json:"status"`
		} `json:"items"`
	}
	if err := h.client.Do("GET", "/apis/apps/v1/deployments", nil, &deplList); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	type deploymentInfo struct {
		Namespace     string `json:"namespace"`
		Name          string `json:"name"`
		Replicas      int    `json:"replicas"`
		ReadyReplicas int    `json:"ready_replicas"`
	}
	out := make([]deploymentInfo, 0, len(deplList.Items))
	for _, d := range deplList.Items {
		out = append(out, deploymentInfo{
			Namespace:     d.Metadata.Namespace,
			Name:          d.Metadata.Name,
			Replicas:      d.Spec.Replicas,
			ReadyReplicas: d.Status.ReadyReplicas,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(out)
}

// HandleK8sSummary returns aggregated cluster stats for the metrics tab.
func (h *KubernetesHandler) HandleK8sSummary(w http.ResponseWriter, r *http.Request) {
	if proxyRequest(w, r, nil, h.nodeStore) {
		return
	}
	type itemList struct {
		Items []json.RawMessage `json:"items"`
	}

	var podList struct {
		Items []struct {
			Status struct {
				Phase string `json:"phase"`
			} `json:"status"`
		} `json:"items"`
	}
	h.client.Do("GET", "/api/v1/pods", nil, &podList)

	running, pending, failed, succeeded := 0, 0, 0, 0
	for _, p := range podList.Items {
		switch p.Status.Phase {
		case "Running":
			running++
		case "Pending":
			pending++
		case "Failed":
			failed++
		case "Succeeded":
			succeeded++
		}
	}

	var nsList, nodeList itemList
	h.client.Do("GET", "/api/v1/namespaces", nil, &nsList)
	h.client.Do("GET", "/api/v1/nodes", nil, &nodeList)

	var deplList struct {
		Items []struct {
			Status struct {
				ReadyReplicas int `json:"readyReplicas"`
				Replicas      int `json:"replicas"`
			} `json:"status"`
		} `json:"items"`
	}
	h.client.Do("GET", "/apis/apps/v1/deployments", nil, &deplList)

	healthyDeploys, unhealthyDeploys := 0, 0
	for _, d := range deplList.Items {
		if d.Status.ReadyReplicas >= d.Status.Replicas && d.Status.Replicas > 0 {
			healthyDeploys++
		} else {
			unhealthyDeploys++
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]int{
		"pods":              len(podList.Items),
		"running":           running,
		"pending":           pending,
		"failed":            failed,
		"succeeded":         succeeded,
		"namespaces":        len(nsList.Items),
		"nodes":             len(nodeList.Items),
		"deployments":       len(deplList.Items),
		"healthy_deploys":   healthyDeploys,
		"unhealthy_deploys": unhealthyDeploys,
	})
}

// HandleApplyYAML pipes the request body YAML to kubectl apply -f -.
func (h *KubernetesHandler) HandleApplyYAML(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 512*1024))
	if err != nil {
		http.Error(w, "read body: "+err.Error(), http.StatusBadRequest)
		return
	}

	if proxyRequest(w, r, body, h.nodeStore) {
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Minute)
	defer cancel()

	cmd := exec.CommandContext(ctx, "kubectl", "apply", "-f", "-")
	cmd.Stdin = bytes.NewReader(body)
	out, err := cmd.CombinedOutput()
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": string(out)})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"output": string(out)})
}

// HandleScale handles POST /k8s/{ns}/deployments/{name}/scale?replicas=3
func (h *KubernetesHandler) HandleScale(w http.ResponseWriter, r *http.Request) {
	if proxyRequest(w, r, nil, h.nodeStore) {
		return
	}
	ns := r.PathValue("ns")
	name := r.PathValue("name")

	replStr := r.URL.Query().Get("replicas")
	replicas, err := strconv.Atoi(replStr)
	if err != nil {
		http.Error(w, "Invalid replicas count", http.StatusBadRequest)
		return
	}

	if err := h.ScaleDeployment(r.Context(), ns, name, replicas); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

// HandleRestart handles POST /k8s/{ns}/deployments/{name}/restart
func (h *KubernetesHandler) HandleRestart(w http.ResponseWriter, r *http.Request) {
	if proxyRequest(w, r, nil, h.nodeStore) {
		return
	}
	ns := r.PathValue("ns")
	name := r.PathValue("name")

	if err := h.RestartDeployment(r.Context(), ns, name); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

// ── Secrets ────────────────────────────────────────────────────────────────
// The Settings → Secrets tab manages real Kubernetes Secret objects, not
// app-level credentials — same cluster, same client as Pods/Deployments
// above. Values are never returned to the browser: list/get expose only
// key NAMES, matching the honest "no value is shown here" behavior the
// mock already disclosed.

type secretInfo struct {
	Namespace string   `json:"namespace"`
	Name      string   `json:"name"`
	Type      string   `json:"type"`
	CreatedAt string   `json:"created_at"`
	Keys      []string `json:"keys"`
}

// HandleListSecrets returns every Secret across all namespaces.
func (h *KubernetesHandler) HandleListSecrets(w http.ResponseWriter, r *http.Request) {
	if proxyRequest(w, r, nil, h.nodeStore) {
		return
	}
	var secretList struct {
		Items []struct {
			Metadata struct {
				Name              string `json:"name"`
				Namespace         string `json:"namespace"`
				CreationTimestamp string `json:"creationTimestamp"`
			} `json:"metadata"`
			Type string            `json:"type"`
			Data map[string]string `json:"data"`
		} `json:"items"`
	}
	if err := h.client.Do("GET", "/api/v1/secrets", nil, &secretList); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	out := make([]secretInfo, 0, len(secretList.Items))
	for _, s := range secretList.Items {
		// Kubernetes auto-creates a service-account token Secret per
		// namespace/service-account; those aren't user secrets and would
		// just clutter the list.
		if s.Type == "kubernetes.io/service-account-token" {
			continue
		}
		keys := make([]string, 0, len(s.Data))
		for k := range s.Data {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		out = append(out, secretInfo{
			Namespace: s.Metadata.Namespace,
			Name:      s.Metadata.Name,
			Type:      s.Type,
			CreatedAt: s.Metadata.CreationTimestamp,
			Keys:      keys,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(out)
}

// HandleGetSecret returns one Secret's metadata and key names — never values.
func (h *KubernetesHandler) HandleGetSecret(w http.ResponseWriter, r *http.Request) {
	if proxyRequest(w, r, nil, h.nodeStore) {
		return
	}
	ns := r.PathValue("ns")
	name := r.PathValue("name")
	path := fmt.Sprintf("/api/v1/namespaces/%s/secrets/%s", ns, name)

	var secret struct {
		Metadata struct {
			CreationTimestamp string `json:"creationTimestamp"`
		} `json:"metadata"`
		Type string            `json:"type"`
		Data map[string]string `json:"data"`
	}
	if err := h.client.Do("GET", path, nil, &secret); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	keys := make([]string, 0, len(secret.Data))
	for k := range secret.Data {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(secretInfo{
		Namespace: ns,
		Name:      name,
		Type:      secret.Type,
		CreatedAt: secret.Metadata.CreationTimestamp,
		Keys:      keys,
	})
}

type createSecretRequest struct {
	Name string            `json:"name"`
	Type string            `json:"type"`
	Data map[string]string `json:"data"`
}

// HandleCreateSecret creates a Secret from plaintext data — the API server
// base64-encodes stringData into data itself, so no encoding happens here.
func (h *KubernetesHandler) HandleCreateSecret(w http.ResponseWriter, r *http.Request) {
	if proxyRequest(w, r, nil, h.nodeStore) {
		return
	}
	ns := r.PathValue("ns")

	var req createSecretRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	if req.Name == "" {
		http.Error(w, "name is required", http.StatusBadRequest)
		return
	}
	if len(req.Data) == 0 {
		http.Error(w, "at least one data key is required", http.StatusBadRequest)
		return
	}
	secretType := req.Type
	if secretType == "" {
		secretType = "Opaque"
	}

	secret := models.Secret{
		KubernetesResurce: models.KubernetesResurce{
			APIVersion: "v1",
			Kind:       "Secret",
			Metadata:   models.Meta{Name: req.Name, Namespace: ns},
		},
		Type:       secretType,
		StringData: req.Data,
	}

	path := fmt.Sprintf("/api/v1/namespaces/%s/secrets", ns)
	if err := h.client.Do("POST", path, secret, nil); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusCreated)
}

// HandleDeleteSecret deletes a Secret.
func (h *KubernetesHandler) HandleDeleteSecret(w http.ResponseWriter, r *http.Request) {
	if proxyRequest(w, r, nil, h.nodeStore) {
		return
	}
	ns := r.PathValue("ns")
	name := r.PathValue("name")
	path := fmt.Sprintf("/api/v1/namespaces/%s/secrets/%s", ns, name)
	if err := h.client.Do("DELETE", path, nil, nil); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

// HandleRotateSecret replaces every existing data key's value with a fresh
// random one — same "invalidate and reissue fresh random material"
// semantics as recovery-code and ERK-share regeneration elsewhere in this
// app. Only meaningful for Opaque secrets: a TLS or Docker-registry secret's
// keys hold structured data (a cert, a dockerconfigjson blob), not
// independent tokens, so randomizing them would just corrupt the secret —
// those should be recreated with new material instead.
func (h *KubernetesHandler) HandleRotateSecret(w http.ResponseWriter, r *http.Request) {
	if proxyRequest(w, r, nil, h.nodeStore) {
		return
	}
	ns := r.PathValue("ns")
	name := r.PathValue("name")
	path := fmt.Sprintf("/api/v1/namespaces/%s/secrets/%s", ns, name)

	var existing models.Secret
	if err := h.client.Do("GET", path, nil, &existing); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if existing.Type != "" && existing.Type != "Opaque" {
		http.Error(w, "rotation is only supported for Opaque secrets — recreate TLS or Docker-registry secrets with new material instead", http.StatusBadRequest)
		return
	}
	if len(existing.Data) == 0 {
		http.Error(w, "secret has no data keys to rotate", http.StatusBadRequest)
		return
	}

	fresh := make(map[string]string, len(existing.Data))
	for k := range existing.Data {
		buf := make([]byte, 32)
		if _, err := rand.Read(buf); err != nil {
			http.Error(w, "failed to generate new value", http.StatusInternalServerError)
			return
		}
		fresh[k] = base64.RawURLEncoding.EncodeToString(buf)
	}
	existing.Data = nil
	existing.StringData = fresh

	if err := h.client.Do("PUT", path, existing, nil); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

// HandleWatchEvents handles GET /k8s/events (Streaming)
func (h *KubernetesHandler) HandleWatchEvents(w http.ResponseWriter, r *http.Request) {
	if proxyRequest(w, r, nil, h.nodeStore) {
		return
	}
	stream, err := h.WatchEvents(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer stream.Close()

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Transfer-Encoding", "chunked")

	io.Copy(w, stream)
}

func (h *KubernetesHandler) HandleTerminal(w http.ResponseWriter, r *http.Request) {
	namespace := r.PathValue("namespace")
	pod := r.PathValue("pod")
	container := r.URL.Query().Get("container")

	remotePath := "/api/k8s/" + namespace + "/" + pod + "/terminal"
	if container != "" {
		remotePath += "?container=" + url.QueryEscape(container)
	}
	if proxyWSToNode(w, r, h.nodeStore, remotePath) {
		return
	}

	browserConn, err := utils.UpgradeToWebSocket(w, r)
	if err != nil {
		return
	}
	defer browserConn.Close()

	execPath := fmt.Sprintf("/api/v1/namespaces/%s/pods/%s/exec?stdout=true&stdin=true&stderr=true&tty=true&command=/bin/sh", namespace, pod)
	if container != "" {
		execPath += "&container=" + url.QueryEscape(container)
	}

	k8sConn, k8sReader, err := h.client.HijackPod(r.Context(), execPath, "v4.channel.k8s.io")
	if err != nil {
		utils.WriteWSFrame(browserConn, []byte("Error: failed to connect to pod: "+err.Error()))
		return
	}
	defer k8sConn.Close()

	done := make(chan struct{})

	// GOROUTINE A: Kubernetes -> Browser (STDOUT/STDERR)
	go func() {
		defer close(done)
		buf := make([]byte, 4096)
		for {
			n, err := k8sReader.Read(buf)
			if err != nil {
				return
			}

			// K8s multiplexed stream: first byte is channel ID.
			// Channel 1 = stdout, Channel 2 = stderr.
			if n > 1 && (buf[0] == 1 || buf[0] == 2) {
				if err := utils.WriteWSFrame(browserConn, buf[1:n]); err != nil {
					return
				}
			}
		}
	}()

	// GOROUTINE B: Browser -> Kubernetes (STDIN)
	go func() {
		for {
			// Read and Unmask the WebSocket frame from the browser
			payload, err := utils.ReadWSFrame(browserConn)
			if err != nil {
				return
			}

			// K8s Protocol: Every STDIN packet MUST start with byte 0x00
			k8sInput := append([]byte{0}, payload...)
			if _, err := k8sConn.Write(k8sInput); err != nil {
				return
			}
		}
	}()

	// Wait until one of the streams is closed
	<-done
}

// HandlePodTraces scrapes the last N log lines of a pod, extracts OTEL-compatible
// spans from structured JSON lines, and returns them.
// GET /kubernetes/{ns}/pods/{name}/traces?tail=200&container=...
func (h *KubernetesHandler) HandlePodTraces(w http.ResponseWriter, r *http.Request) {
	if proxyRequest(w, r, nil, h.nodeStore) {
		return
	}
	ns := r.PathValue("ns")
	name := r.PathValue("name")
	container := r.URL.Query().Get("container")
	tail := r.URL.Query().Get("tail")
	if tail == "" {
		tail = "200"
	}

	path := fmt.Sprintf("/api/v1/namespaces/%s/pods/%s/log?tail=%s&timestamps=true", ns, name, tail)
	if container != "" {
		path += "&container=" + url.QueryEscape(container)
	}
	stream, err := h.client.DoStream("GET", path, nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer stream.Close()

	serviceName := name
	if container != "" {
		serviceName = container
	}

	var spans []utils.TraceEntry
	sc := bufio.NewScanner(stream)
	for sc.Scan() {
		line := sc.Text()
		// Strip K8s timestamps (RFC3339 prefix before first space)
		if len(line) > 30 && line[4] == '-' {
			if idx := strings.Index(line, " "); idx > 0 {
				line = line[idx+1:]
			}
		}
		if s := utils.ParseSpanFromLogLine(line, serviceName, "k8s"); s != nil {
			spans = append(spans, *s)
		}
	}
	if spans == nil {
		spans = []utils.TraceEntry{}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(spans)
}

// HandlePodMetricsScrape scrapes a Prometheus /metrics endpoint from a pod via
// the Kubernetes API server proxy.
// GET /kubernetes/{ns}/pods/{name}/metrics-scrape?port=9090&path=/metrics
func (h *KubernetesHandler) HandlePodMetricsScrape(w http.ResponseWriter, r *http.Request) {
	if proxyRequest(w, r, nil, h.nodeStore) {
		return
	}
	ns := r.PathValue("ns")
	name := r.PathValue("name")
	port := r.URL.Query().Get("port")
	if port == "" {
		port = "9090"
	}
	metricsPath := r.URL.Query().Get("path")
	if metricsPath == "" {
		metricsPath = "/metrics"
	}

	// Use K8s API server proxy to reach the pod.
	k8sPath := fmt.Sprintf("/api/v1/namespaces/%s/pods/%s:%s/proxy%s", ns, name, port, metricsPath)
	stream, err := h.client.DoStream("GET", k8sPath, nil)
	if err != nil {
		http.Error(w, "scrape failed: "+err.Error(), http.StatusBadGateway)
		return
	}
	defer stream.Close()

	var points []utils.MetricPoint
	sc := bufio.NewScanner(stream)
	for sc.Scan() {
		if p := utils.ParsePrometheusLine(sc.Text(), name, "k8s"); p != nil {
			utils.AppendMetric(*p)
			points = append(points, *p)
		}
	}
	if points == nil {
		points = []utils.MetricPoint{}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(points)
}
