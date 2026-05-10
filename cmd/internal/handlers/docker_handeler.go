package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"scutum/cmd/internal/clients"
	"scutum/cmd/internal/models"
	"scutum/cmd/internal/utils"
)

type DockerHandler struct {
	client    *clients.DockerClient
	nodeStore nodeProxyStore
}

func NewDockerHandler(ns nodeProxyStore) *DockerHandler {
	return &DockerHandler{
		client:    utils.GetPlatformClient(),
		nodeStore: ns,
	}
}

// PostDeploy handles the deployment of a new container based on the user's request.
func (h *DockerHandler) PostDeploy(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 512*1024))
	if err != nil {
		http.Error(w, "read body: "+err.Error(), http.StatusBadRequest)
		return
	}
	if proxyRequest(w, r, body, h.nodeStore) {
		return
	}

	var req models.DeployRequest
	if err := json.NewDecoder(bytes.NewReader(body)).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	dockerConfig := models.ContainerCreateConfig{
		Image: req.Repo,
		Env:   req.Env,
		ExposedPorts: map[string]struct{}{
			fmt.Sprintf("%d/tcp", req.Port): {},
		},
		HostConfig: models.HostConfig{
			Memory:   req.MemoryLimit,
			NanoCpus: int64(req.CPULimit * 1e9),
			PortBindings: map[string][]models.PortBinding{
				fmt.Sprintf("%d/tcp", req.Port): {
					{HostPort: fmt.Sprintf("%d", req.HostPort)},
				},
			},
			Binds: req.Volumes,
			RestartPolicy: models.RestartPolicy{
				Name: req.Restart,
			},
			NetworkMode: "bridge",
		},
	}

	var createResp struct {
		ID       string   `json:"Id"`
		Warnings []string `json:"Warnings"`
	}

	createPath := fmt.Sprintf("/containers/create?name=%s", req.Name)
	if err := h.client.Do("POST", createPath, dockerConfig, &createResp); err != nil {
		http.Error(w, "Create failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	startPath := fmt.Sprintf("/containers/%s/start", createResp.ID)
	if err := h.client.Do("POST", startPath, nil, nil); err != nil {
		http.Error(w, "Start failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	audit("CONTAINER_DEPLOYED", r, "container_id", createResp.ID, "image", req.Repo, "name", req.Name)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(models.DeployResponse{
		ID:      createResp.ID,
		Status:  "running",
		Message: "Deployment successful",
	})
}

// GetStats streams real-time stats for a container.
func (h *DockerHandler) GetStats(ctx context.Context, id string) (io.ReadCloser, error) {
	path := fmt.Sprintf("/containers/%s/stats?stream=true", id)
	return h.client.DoStream("GET", path, nil)
}

// listens to logs and writes them to the provided stdout writer.
func (h *DockerHandler) ListenToLogs(ctx context.Context, id string, stdout io.Writer) error {
	path := fmt.Sprintf("/containers/%s/logs?follow=true&stdout=true&stderr=true", id)
	stream, err := h.client.DoStream("GET", path, nil)
	if err != nil {
		return err
	}
	defer stream.Close()

	header := make([]byte, 8)
	for {
		if _, err := io.ReadFull(stream, header); err != nil {
			return err
		}
		size := uint32(header[4])<<24 | uint32(header[5])<<16 | uint32(header[6])<<8 | uint32(header[7])
		if _, err := io.CopyN(stdout, stream, int64(size)); err != nil {
			return err
		}
	}
}

// Start initiates a created container.
func (h *DockerHandler) Start(ctx context.Context, id string) error {
	path := fmt.Sprintf("/containers/%s/start", id)
	return h.client.Do("POST", path, nil, nil)
}

// Stop performs a graceful shutdown.
// The 't' query param specifies the number of seconds to wait before a SIGKILL.
func (h *DockerHandler) Stop(ctx context.Context, id string) error {
	path := fmt.Sprintf("/containers/%s/stop?t=10", id)
	return h.client.Do("POST", path, nil, nil)
}

// Restart stops and then starts a container.
func (h *DockerHandler) Restart(ctx context.Context, id string) error {
	path := fmt.Sprintf("/containers/%s/restart?t=10", id)
	return h.client.Do("POST", path, nil, nil)
}

// Kill sends an immediate SIGKILL to the container.
func (h *DockerHandler) Kill(ctx context.Context, id string) error {
	path := fmt.Sprintf("/containers/%s/kill", id)
	return h.client.Do("POST", path, nil, nil)
}

// Delete removes the container.
// 'v=true' removes associated volumes; 'force=true' kills it if it's still running.
func (h *DockerHandler) Delete(ctx context.Context, id string) error {
	path := fmt.Sprintf("/containers/%s?v=true&force=true", id)
	return h.client.Do("DELETE", path, nil, nil)
}

func (h *DockerHandler) HandleStart(w http.ResponseWriter, r *http.Request) {
	if proxyRequest(w, r, nil, h.nodeStore) {
		return
	}
	id := r.PathValue("id")
	if err := h.Start(r.Context(), id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	audit("CONTAINER_STARTED", r, "container_id", id)
	w.WriteHeader(http.StatusNoContent)
}

func (h *DockerHandler) HandleStop(w http.ResponseWriter, r *http.Request) {
	if proxyRequest(w, r, nil, h.nodeStore) {
		return
	}
	id := r.PathValue("id")
	if err := h.Stop(r.Context(), id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	audit("CONTAINER_STOPPED", r, "container_id", id)
	w.WriteHeader(http.StatusNoContent)
}

func (h *DockerHandler) HandleRestart(w http.ResponseWriter, r *http.Request) {
	if proxyRequest(w, r, nil, h.nodeStore) {
		return
	}
	id := r.PathValue("id")
	if err := h.Restart(r.Context(), id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	audit("CONTAINER_RESTARTED", r, "container_id", id)
	w.WriteHeader(http.StatusNoContent)
}

func (h *DockerHandler) HandleKill(w http.ResponseWriter, r *http.Request) {
	if proxyRequest(w, r, nil, h.nodeStore) {
		return
	}
	id := r.PathValue("id")
	if err := h.Kill(r.Context(), id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	audit("CONTAINER_KILLED", r, "container_id", id)
	w.WriteHeader(http.StatusNoContent)
}

func (h *DockerHandler) HandleDelete(w http.ResponseWriter, r *http.Request) {
	if proxyRequest(w, r, nil, h.nodeStore) {
		return
	}
	id := r.PathValue("id")
	if err := h.Delete(r.Context(), id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	audit("CONTAINER_DELETED", r, "container_id", id)
	w.WriteHeader(http.StatusNoContent)
}

// HandleStatsSnapshot returns a single stats snapshot with computed CPU %, memory, network, and I/O.
func (h *DockerHandler) HandleStatsSnapshot(w http.ResponseWriter, r *http.Request) {
	if proxyRequest(w, r, nil, h.nodeStore) {
		return
	}
	id := r.PathValue("id")

	// stream=false makes Docker return one object and close the connection.
	path := fmt.Sprintf("/containers/%s/stats?stream=false", id)
	stream, err := h.client.DoStream("GET", path, nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer stream.Close()

	var ds struct {
		CPUStats struct {
			CPUUsage       struct{ TotalUsage int64 `json:"total_usage"` } `json:"cpu_usage"`
			SystemCPUUsage int64                                           `json:"system_cpu_usage"`
			OnlineCPUs     int                                             `json:"online_cpus"`
		} `json:"cpu_stats"`
		PreCPUStats struct {
			CPUUsage       struct{ TotalUsage int64 `json:"total_usage"` } `json:"cpu_usage"`
			SystemCPUUsage int64                                           `json:"system_cpu_usage"`
		} `json:"precpu_stats"`
		MemoryStats struct {
			Usage int64 `json:"usage"`
			Limit int64 `json:"limit"`
			Stats struct {
				Cache int64 `json:"cache"`
			} `json:"stats"`
		} `json:"memory_stats"`
		Networks map[string]struct {
			RxBytes int64 `json:"rx_bytes"`
			TxBytes int64 `json:"tx_bytes"`
		} `json:"networks"`
		BlkioStats struct {
			IoServiceBytesRecursive []struct {
				Op    string `json:"op"`
				Value int64  `json:"value"`
			} `json:"io_service_bytes_recursive"`
		} `json:"blkio_stats"`
	}

	if err := json.NewDecoder(stream).Decode(&ds); err != nil {
		http.Error(w, "decode stats: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// CPU %
	cpuDelta := float64(ds.CPUStats.CPUUsage.TotalUsage - ds.PreCPUStats.CPUUsage.TotalUsage)
	sysDelta := float64(ds.CPUStats.SystemCPUUsage - ds.PreCPUStats.SystemCPUUsage)
	cpus := ds.CPUStats.OnlineCPUs
	if cpus == 0 {
		cpus = 1
	}
	var cpuPct float64
	if sysDelta > 0 {
		cpuPct = (cpuDelta / sysDelta) * float64(cpus) * 100.0
	}

	// Memory (subtract cache so it matches docker stats output)
	memUsage := ds.MemoryStats.Usage - ds.MemoryStats.Stats.Cache
	if memUsage < 0 {
		memUsage = ds.MemoryStats.Usage
	}

	// Network totals across all interfaces
	var netRx, netTx int64
	for _, n := range ds.Networks {
		netRx += n.RxBytes
		netTx += n.TxBytes
	}

	// Block I/O
	var blkRead, blkWrite int64
	for _, entry := range ds.BlkioStats.IoServiceBytesRecursive {
		switch strings.ToLower(entry.Op) {
		case "read":
			blkRead += entry.Value
		case "write":
			blkWrite += entry.Value
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"cpu_percent": cpuPct,
		"mem_usage":   memUsage,
		"mem_limit":   ds.MemoryStats.Limit,
		"net_rx":      netRx,
		"net_tx":      netTx,
		"blk_read":    blkRead,
		"blk_write":   blkWrite,
	})
}

// HandleStats streams container stats to the client in real-time.
func (h *DockerHandler) HandleStats(w http.ResponseWriter, r *http.Request) {
	if proxyRequest(w, r, nil, h.nodeStore) {
		return
	}
	id := r.PathValue("id")
	stream, err := h.GetStats(r.Context(), id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer stream.Close()

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Transfer-Encoding", "chunked")

	io.Copy(w, stream)
}

// HandleLogs streams multiplexed container logs to the client.
func (h *DockerHandler) HandleLogs(w http.ResponseWriter, r *http.Request) {
	if proxyRequest(w, r, nil, h.nodeStore) {
		return
	}
	id := r.PathValue("id")

	w.Header().Set("Content-Type", "text/plain")
	w.Header().Set("Transfer-Encoding", "chunked")

	if err := h.ListenToLogs(r.Context(), id, w); err != nil {
		fmt.Printf("Log streaming error: %v\n", err)
	}
}

func (h *DockerHandler) HandleTerminal(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	ctx := r.Context()

	audit("TERMINAL_SESSION_STARTED", r, "container_id", id)

	// 1. Upgrade to WebSocket first so errors can be delivered over the channel.
	browserConn, err := utils.UpgradeToWebSocket(w, r)
	if err != nil {
		return
	}
	defer browserConn.Close()

	sendErr := func(msg string) {
		utils.WriteWSFrame(browserConn, []byte(msg))
	}

	// 2. Create the exec instance.
	execConfig := map[string]interface{}{
		"AttachStdin":  true,
		"AttachStdout": true,
		"AttachStderr": true,
		"Tty":          true,
		"Cmd":          []string{"/bin/sh"},
	}
	var execCreateResp struct {
		ID string `json:"Id"`
	}
	if err := h.client.Do("POST", fmt.Sprintf("/containers/%s/exec", id), execConfig, &execCreateResp); err != nil {
		sendErr("Error: failed to create exec session: " + err.Error())
		return
	}
	if execCreateResp.ID == "" {
		sendErr("Error: Docker returned an empty exec ID")
		return
	}

	// 3. Hijack the Docker connection to start the exec session.
	startConfig := map[string]bool{"Detach": false, "Tty": true}
	dockerConn, dockerReader, err := h.client.Hijack(ctx, fmt.Sprintf("/exec/%s/start", execCreateResp.ID), startConfig)
	if err != nil {
		sendErr("Error: failed to start exec session: " + err.Error())
		return
	}
	defer dockerConn.Close()

	errChan := make(chan error, 2)

	// Docker → browser
	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := dockerReader.Read(buf)
			if n > 0 {
				if werr := utils.WriteWSFrame(browserConn, buf[:n]); werr != nil {
					errChan <- werr
					return
				}
			}
			if err != nil {
				errChan <- err
				return
			}
		}
	}()

	// Browser → Docker
	go func() {
		for {
			payload, err := utils.ReadWSFrame(browserConn)
			if err != nil {
				errChan <- err
				return
			}
			if _, err := dockerConn.Write(payload); err != nil {
				errChan <- err
				return
			}
		}
	}()

	<-errChan
}

// HandleListContainers lists all containers from the Docker daemon.
func (h *DockerHandler) HandleListContainers(w http.ResponseWriter, r *http.Request) {
	if proxyRequest(w, r, nil, h.nodeStore) {
		return
	}
	var result json.RawMessage
	if err := h.client.Do("GET", "/containers/json?all=true", nil, &result); err != nil {
		http.Error(w, "failed to list containers: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(result)
}

// HandleInspect returns the full Docker inspect JSON for a container.
func (h *DockerHandler) HandleInspect(w http.ResponseWriter, r *http.Request) {
	if proxyRequest(w, r, nil, h.nodeStore) {
		return
	}
	id := r.PathValue("id")
	var result json.RawMessage
	if err := h.client.Do("GET", fmt.Sprintf("/containers/%s/json", id), nil, &result); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(result)
}

// HandleLogsJSON returns the last N log lines as a JSON array with ts/stream/msg fields.
func (h *DockerHandler) HandleLogsJSON(w http.ResponseWriter, r *http.Request) {
	if proxyRequest(w, r, nil, h.nodeStore) {
		return
	}
	id := r.PathValue("id")
	tail := r.URL.Query().Get("tail")
	if tail == "" {
		tail = "100"
	}
	path := fmt.Sprintf("/containers/%s/logs?stdout=true&stderr=true&timestamps=true&tail=%s", id, tail)
	stream, err := h.client.DoStream("GET", path, nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer stream.Close()

	type logLine struct {
		TS     string `json:"ts"`
		Stream string `json:"stream"`
		Msg    string `json:"msg"`
	}

	var lines []logLine
	hdr := make([]byte, 8)
	for {
		if _, err := io.ReadFull(stream, hdr); err != nil {
			break
		}
		streamName := "stdout"
		if hdr[0] == 2 {
			streamName = "stderr"
		}
		size := int(uint32(hdr[4])<<24 | uint32(hdr[5])<<16 | uint32(hdr[6])<<8 | uint32(hdr[7]))
		buf := make([]byte, size)
		if _, err := io.ReadFull(stream, buf); err != nil {
			break
		}
		text := strings.TrimRight(string(buf), "\n")
		ts, msg := "", text
		if i := strings.IndexByte(text, ' '); i > 0 {
			ts, msg = text[:i], text[i+1:]
		}
		lines = append(lines, logLine{TS: ts, Stream: streamName, Msg: msg})
	}

	if lines == nil {
		lines = []logLine{}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(lines)
}

// HandleDeployCompose runs docker compose up for the provided YAML body.
func (h *DockerHandler) HandleDeployCompose(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 512*1024))
	if err != nil {
		http.Error(w, "read body: "+err.Error(), http.StatusBadRequest)
		return
	}

	if proxyRequest(w, r, body, h.nodeStore) {
		return
	}

	tmp, err := os.CreateTemp("", "compose-*.yml")
	if err != nil {
		http.Error(w, "create temp file", http.StatusInternalServerError)
		return
	}
	defer os.Remove(tmp.Name())

	if _, err = tmp.Write(body); err != nil {
		tmp.Close()
		http.Error(w, "write temp file", http.StatusInternalServerError)
		return
	}
	tmp.Close()

	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Minute)
	defer cancel()

	out, err := exec.CommandContext(ctx, "docker", "compose", "-f", tmp.Name(), "up", "-d").CombinedOutput()
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": string(out)})
		return
	}

	audit("COMPOSE_DEPLOYED", r)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"output": string(out)})
}
