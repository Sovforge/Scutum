package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"text/template"
	"time"

	"github.com/google/uuid"

	"scutum/cmd/internal/store"
)

// FilesHandler distributes files (certs, app configs, scripts) to individual
// nodes or whole groups over the same HMAC-authenticated hub-to-node channel
// Docker/Kubernetes actions already use. Destination paths are always
// relative and resolve under a managed root
// (<DATA_DIR>/distributed-files/<destination_path> on the *target* node) —
// deliberately not arbitrary absolute host paths, since this process
// typically runs as root with NET_ADMIN and an unrestricted write-any-path
// primitive would be a real privilege-escalation vector.
type filesStore interface {
	GetNode(ctx context.Context, id string) (store.NodeRecord, error)
	ListNodesInGroup(ctx context.Context, groupID string) ([]store.NodeRecord, error)
	CreateFileTransfer(ctx context.Context, t store.FileTransfer) error
	UpdateFileTransferStatus(ctx context.Context, id, status, errMsg, completedAt string) error
	ListFileTransfers(ctx context.Context, limit int) ([]store.FileTransfer, error)
}

type FilesHandler struct {
	store   filesStore
	dataDir string
}

func NewFilesHandler(s filesStore, dataDir string) *FilesHandler {
	return &FilesHandler{store: s, dataDir: dataDir}
}

const distributedFilesDirName = "distributed-files"

func (h *FilesHandler) managedRoot() string {
	return filepath.Join(h.dataDir, distributedFilesDirName)
}

// SafeJoin resolves destPath under root, rejecting absolute paths and any
// ".." segment that would escape it.
func SafeJoin(root, destPath string) (string, error) {
	if destPath == "" {
		return "", fmt.Errorf("destination_path is required")
	}
	if filepath.IsAbs(destPath) {
		return "", fmt.Errorf("destination_path must be relative — it resolves under a managed directory on the target node")
	}
	cleaned := filepath.Clean(destPath)
	if cleaned == ".." || strings.HasPrefix(cleaned, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("destination_path may not contain ..")
	}
	rootClean := filepath.Clean(root)
	full := filepath.Join(rootClean, cleaned)
	if full != rootClean && !strings.HasPrefix(full, rootClean+string(os.PathSeparator)) {
		return "", fmt.Errorf("destination_path escapes the managed directory")
	}
	return full, nil
}

func writeManagedFile(root, destPath string, perm os.FileMode, content []byte) error {
	full, err := SafeJoin(root, destPath)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(full), 0750); err != nil {
		return fmt.Errorf("create directory: %w", err)
	}
	if err := os.WriteFile(full, content, perm); err != nil {
		return fmt.Errorf("write file: %w", err)
	}
	return nil
}

type fileUploadRequest struct {
	filename      string
	destPath      string
	permissions   os.FileMode
	content       []byte
	templateVars  map[string]string
	applyTemplate bool
}

func parseFileUploadRequest(r *http.Request) (*fileUploadRequest, error) {
	if err := r.ParseMultipartForm(64 << 20); err != nil { // 64MB cap
		return nil, fmt.Errorf("failed to parse form: %w", err)
	}

	perm := os.FileMode(0644)
	if permStr := r.FormValue("permissions"); permStr != "" {
		v, err := strconv.ParseUint(strings.TrimPrefix(permStr, "0"), 8, 32)
		if err != nil {
			return nil, fmt.Errorf("invalid permissions %q — use octal like 0644", permStr)
		}
		perm = os.FileMode(v)
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		return nil, fmt.Errorf("file is required")
	}
	defer file.Close()
	content, err := io.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	req := &fileUploadRequest{
		filename:    header.Filename,
		destPath:    r.FormValue("destination_path"),
		permissions: perm,
		content:     content,
	}

	// Template substitution is opt-in (the caller must send template_vars,
	// even "{}" for just NodeIP/NodeName/NodeID) — binary uploads like TLS
	// certs are never accidentally run through text/template.
	if raw := r.FormValue("template_vars"); raw != "" {
		var vars map[string]string
		if err := json.Unmarshal([]byte(raw), &vars); err != nil {
			return nil, fmt.Errorf("invalid template_vars JSON: %w", err)
		}
		req.templateVars = vars
		req.applyTemplate = true
	}
	return req, nil
}

// RenderTemplate substitutes {{ .NodeIP }} / {{ .NodeName }} / {{ .NodeID }}
// and any caller-supplied template_vars into content.
func RenderTemplate(content []byte, node store.NodeRecord, extra map[string]string) ([]byte, error) {
	data := map[string]string{
		"NodeIP":   strings.SplitN(node.Address, ":", 2)[0],
		"NodeName": node.Name,
		"NodeID":   node.ID,
	}
	for k, v := range extra {
		data[k] = v
	}
	tmpl, err := template.New("file").Option("missingkey=error").Parse(string(content))
	if err != nil {
		return nil, fmt.Errorf("template parse error: %w", err)
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return nil, fmt.Errorf("template render error: %w", err)
	}
	return buf.Bytes(), nil
}

// deliverToNode writes content under the managed root on the target node —
// locally if node is the hub itself, otherwise relayed over the same
// HMAC-authenticated hub-to-node channel Docker/Kubernetes proxying uses.
func (h *FilesHandler) deliverToNode(ctx context.Context, node store.NodeRecord, filename, destPath string, perm os.FileMode, content []byte) error {
	if node.Type == "hub" {
		return writeManagedFile(h.managedRoot(), destPath, perm, content)
	}
	return relayFileToNode(ctx, node, filename, destPath, perm, content)
}

func relayFileToNode(ctx context.Context, node store.NodeRecord, filename, destPath string, perm os.FileMode, content []byte) error {
	const remotePath = "/api/nodes/files/place"
	host := normaliseAddress(node.Address)
	target := "https://" + host + remotePath

	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	if err := mw.WriteField("destination_path", destPath); err != nil {
		return fmt.Errorf("build relay request: %w", err)
	}
	if err := mw.WriteField("permissions", fmt.Sprintf("%#o", perm)); err != nil {
		return fmt.Errorf("build relay request: %w", err)
	}
	part, err := mw.CreateFormFile("file", filename)
	if err != nil {
		return fmt.Errorf("build relay request: %w", err)
	}
	if _, err := part.Write(content); err != nil {
		return fmt.Errorf("build relay request: %w", err)
	}
	if err := mw.Close(); err != nil {
		return fmt.Errorf("build relay request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, target, &body)
	if err != nil {
		return fmt.Errorf("build relay request: %w", err)
	}
	req.Header.Set("Content-Type", mw.FormDataContentType())

	if len(proxyHMACKey) > 0 {
		ts := strconv.FormatInt(time.Now().Unix(), 10)
		sig := hubRequestSig(proxyHMACKey, ts, http.MethodPost, remotePath, nil)
		req.Header.Set("X-Scutum-Hub-Ts", ts)
		req.Header.Set("X-Scutum-Hub-Sig", sig)
	}

	client := &http.Client{Timeout: 30 * time.Second, Transport: &http.Transport{TLSClientConfig: proxyTLSConfig}}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("relay to node: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("node rejected file (%d): %s", resp.StatusCode, strings.TrimSpace(string(b)))
	}
	return nil
}

// HandlePlace is the endpoint every instance (hub and edge alike) exposes
// locally — it's what actually writes the file to disk. Reached either
// directly (the target is this node) or relayed here by the hub via HMAC
// signature (see relayFileToNode).
func (h *FilesHandler) HandlePlace(w http.ResponseWriter, r *http.Request) {
	req, err := parseFileUploadRequest(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := writeManagedFile(h.managedRoot(), req.destPath, req.permissions, req.content); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	audit("FILE_PLACED", r, "destination_path", req.destPath, "size_bytes", len(req.content))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("file placed"))
}

func (h *FilesHandler) distributeOne(ctx context.Context, r *http.Request, node store.NodeRecord, targetType, targetID string, req *fileUploadRequest) store.FileTransfer {
	now := time.Now().UTC().Format(time.RFC3339)
	t := store.FileTransfer{
		ID:              uuid.New().String(),
		TargetType:      targetType,
		TargetID:        targetID,
		NodeID:          node.ID,
		NodeName:        node.Name,
		Filename:        req.filename,
		DestinationPath: req.destPath,
		Permissions:     fmt.Sprintf("%#o", req.permissions),
		SizeBytes:       int64(len(req.content)),
		Status:          "in_progress",
		CreatedAt:       now,
	}
	_ = h.store.CreateFileTransfer(ctx, t)

	content := req.content
	if req.applyTemplate {
		rendered, err := RenderTemplate(content, node, req.templateVars)
		if err != nil {
			t.Status, t.Error = "failed", err.Error()
		} else {
			content = rendered
		}
	}

	if t.Status != "failed" {
		if err := h.deliverToNode(ctx, node, req.filename, req.destPath, req.permissions, content); err != nil {
			t.Status, t.Error = "failed", err.Error()
		} else {
			t.Status = "done"
		}
	}

	completedAt := time.Now().UTC().Format(time.RFC3339)
	_ = h.store.UpdateFileTransferStatus(ctx, t.ID, t.Status, t.Error, completedAt)
	t.CompletedAt = completedAt

	if t.Status == "done" {
		audit("FILE_TRANSFER_COMPLETED", r, "node_id", node.ID, "node_name", node.Name, "destination_path", req.destPath)
	} else {
		audit("FILE_TRANSFER_FAILED", r, "node_id", node.ID, "node_name", node.Name, "destination_path", req.destPath, "error", t.Error)
	}
	return t
}

// HandleDistributeToNode handles POST /nodes/{id}/files.
func (h *FilesHandler) HandleDistributeToNode(w http.ResponseWriter, r *http.Request) {
	nodeID := r.PathValue("id")
	node, err := h.store.GetNode(r.Context(), nodeID)
	if err != nil {
		http.Error(w, "node not found", http.StatusNotFound)
		return
	}
	req, err := parseFileUploadRequest(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if _, err := SafeJoin(h.managedRoot(), req.destPath); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	t := h.distributeOne(r.Context(), r, node, "node", nodeID, req)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(t)
}

// HandleDistributeToGroup handles POST /groups/{id}/files.
func (h *FilesHandler) HandleDistributeToGroup(w http.ResponseWriter, r *http.Request) {
	groupID := r.PathValue("id")
	nodes, err := h.store.ListNodesInGroup(r.Context(), groupID)
	if err != nil {
		http.Error(w, "group not found", http.StatusNotFound)
		return
	}
	if len(nodes) == 0 {
		http.Error(w, "group has no members", http.StatusBadRequest)
		return
	}
	req, err := parseFileUploadRequest(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if _, err := SafeJoin(h.managedRoot(), req.destPath); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	results := make([]store.FileTransfer, 0, len(nodes))
	for _, n := range nodes {
		results = append(results, h.distributeOne(r.Context(), r, n, "group", groupID, req))
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(results)
}

// HandleListTransfers handles GET /file-transfers.
func (h *FilesHandler) HandleListTransfers(w http.ResponseWriter, r *http.Request) {
	limit := 100
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			limit = n
		}
	}
	transfers, err := h.store.ListFileTransfers(r.Context(), limit)
	if err != nil {
		http.Error(w, "failed to list transfers", http.StatusInternalServerError)
		return
	}
	if transfers == nil {
		transfers = []store.FileTransfer{}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(transfers)
}
