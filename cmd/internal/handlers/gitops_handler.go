package handlers

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"scutum/cmd/internal/auth"
	"scutum/cmd/internal/gitops"
	"scutum/cmd/internal/store"
	"scutum/cmd/internal/utils"

	"github.com/google/uuid"
)

var validManifestTypes = map[string]bool{"compose": true, "kubernetes": true}

// gitopsStore narrows *store.Store to what this feature needs: source/event
// CRUD, node lookup (for validating and relaying to target_node_id), the
// secrets vault (private-repo tokens and webhook secrets), and enough of
// the user model to authenticate a manual sync call made without a JWT.
type gitopsStore interface {
	CreateGitOpsSource(ctx context.Context, src store.GitOpsSource) (store.GitOpsSource, error)
	GetGitOpsSource(ctx context.Context, id string) (store.GitOpsSource, error)
	ListGitOpsSources(ctx context.Context) ([]store.GitOpsSource, error)
	UpdateGitOpsSourceConfig(ctx context.Context, src store.GitOpsSource) (store.GitOpsSource, error)
	DeleteGitOpsSource(ctx context.Context, id string) error
	RecordGitOpsSync(ctx context.Context, ev store.GitOpsSyncEvent, commitSHA, contentHash string) error
	ListGitOpsSyncEvents(ctx context.Context, sourceID string, limit int) ([]store.GitOpsSyncEvent, error)

	GetNode(ctx context.Context, id string) (store.NodeRecord, error)

	CreateAppSecret(ctx context.Context, id, name, description string, value []byte, createdBy string) (store.AppSecret, error)
	UpdateAppSecret(ctx context.Context, name string, description *string, value []byte, updatedBy string) (store.AppSecret, error)
	GetAppSecretValue(ctx context.Context, name string) ([]byte, error)
	DeleteAppSecret(ctx context.Context, name string) error

	UserByAPIKey(keyHash string) (userID, username string, err error)
	UserHasPermission(userID, resource, action string) (bool, error)
}

type GitOpsHandler struct {
	store      gitopsStore
	dataDir    string
	jwtSecret  []byte
	dockerCtrl *DockerHandler
	k8sCtrl    *KubernetesHandler
}

func NewGitOpsHandler(st gitopsStore, dataDir string, jwtSecret []byte, dockerCtrl *DockerHandler, k8sCtrl *KubernetesHandler) *GitOpsHandler {
	return &GitOpsHandler{store: st, dataDir: dataDir, jwtSecret: jwtSecret, dockerCtrl: dockerCtrl, k8sCtrl: k8sCtrl}
}

func tokenSecretName(sourceID string) string   { return "gitops:" + sourceID + ":token" }
func webhookSecretName(sourceID string) string { return "gitops:" + sourceID + ":webhook_secret" }

// ── CRUD ─────────────────────────────────────────────────────────────────

func (h *GitOpsHandler) HandleList(w http.ResponseWriter, r *http.Request) {
	sources, err := h.store.ListGitOpsSources(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if sources == nil {
		sources = []store.GitOpsSource{}
	}
	writeJSON(w, http.StatusOK, sources)
}

type gitopsSourceReq struct {
	Name                string `json:"name"`
	RepoURL             string `json:"repo_url"`
	Branch              string `json:"branch"`
	Path                string `json:"path"`
	ManifestType        string `json:"manifest_type"`
	TargetNodeID        string `json:"target_node_id"`
	PollIntervalSeconds int    `json:"poll_interval_seconds"`
	Enabled             *bool  `json:"enabled"`
	Username            string `json:"username,omitempty"`
	// Token/WebhookSecret are write-only: sent to set/update, never
	// returned by any GET. An empty string on update leaves the existing
	// value (if any) unchanged, matching the secrets vault's own semantics.
	Token         *string `json:"token,omitempty"`
	WebhookSecret *string `json:"webhook_secret,omitempty"`
}

func (h *GitOpsHandler) validate(r *http.Request, req *gitopsSourceReq) error {
	if req.Name == "" {
		return fmt.Errorf("name is required")
	}
	if req.RepoURL == "" {
		return fmt.Errorf("repo_url is required")
	}
	if req.ManifestType == "" {
		req.ManifestType = "compose"
	}
	if !validManifestTypes[req.ManifestType] {
		return fmt.Errorf(`manifest_type must be "compose" or "kubernetes"`)
	}
	if req.Branch == "" {
		req.Branch = "main"
	}
	if req.PollIntervalSeconds <= 0 {
		req.PollIntervalSeconds = 60
	} else if req.PollIntervalSeconds < 10 {
		return fmt.Errorf("poll_interval_seconds must be at least 10")
	}
	if req.TargetNodeID != "" {
		if _, err := h.store.GetNode(r.Context(), req.TargetNodeID); err != nil {
			return fmt.Errorf("target node %q not found", req.TargetNodeID)
		}
	}
	return nil
}

func (h *GitOpsHandler) HandleCreate(w http.ResponseWriter, r *http.Request) {
	var req gitopsSourceReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if err := h.validate(r, &req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}

	id := uuid.New().String()
	actor := actorFromRequest(r)
	src := store.GitOpsSource{
		ID: id, Name: req.Name, RepoURL: req.RepoURL, Branch: req.Branch, Path: req.Path,
		ManifestType: req.ManifestType, TargetNodeID: req.TargetNodeID, Username: req.Username,
		PollIntervalSeconds: req.PollIntervalSeconds, Enabled: enabled, CreatedBy: actor,
	}
	created, err := h.store.CreateGitOpsSource(r.Context(), src)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if req.Token != nil && *req.Token != "" {
		if _, err := h.store.CreateAppSecret(r.Context(), uuid.New().String(), tokenSecretName(id), "GitOps source "+req.Name+" access token", []byte(*req.Token), actor); err != nil {
			http.Error(w, "created source but failed to store token: "+err.Error(), http.StatusInternalServerError)
			return
		}
	}
	if req.WebhookSecret != nil && *req.WebhookSecret != "" {
		if _, err := h.store.CreateAppSecret(r.Context(), uuid.New().String(), webhookSecretName(id), "GitOps source "+req.Name+" webhook secret", []byte(*req.WebhookSecret), actor); err != nil {
			http.Error(w, "created source but failed to store webhook secret: "+err.Error(), http.StatusInternalServerError)
			return
		}
	}

	audit("GITOPS_SOURCE_CREATED", r, "source_id", id, "name", req.Name, "repo_url", req.RepoURL)
	go h.reconcileOne(context.Background(), id, "create")
	writeJSON(w, http.StatusCreated, created)
}

func (h *GitOpsHandler) HandleUpdate(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req gitopsSourceReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if err := h.validate(r, &req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}

	actor := actorFromRequest(r)
	src := store.GitOpsSource{
		ID: id, Name: req.Name, RepoURL: req.RepoURL, Branch: req.Branch, Path: req.Path,
		ManifestType: req.ManifestType, TargetNodeID: req.TargetNodeID, Username: req.Username,
		PollIntervalSeconds: req.PollIntervalSeconds, Enabled: enabled, UpdatedBy: actor,
	}
	updated, err := h.store.UpdateGitOpsSourceConfig(r.Context(), src)
	if err != nil {
		code := http.StatusInternalServerError
		if strings.Contains(err.Error(), "not found") {
			code = http.StatusNotFound
		}
		http.Error(w, err.Error(), code)
		return
	}

	if req.Token != nil && *req.Token != "" {
		desc := "GitOps source " + req.Name + " access token"
		if _, err := h.store.UpdateAppSecret(r.Context(), tokenSecretName(id), &desc, []byte(*req.Token), actor); err != nil {
			if _, cerr := h.store.CreateAppSecret(r.Context(), uuid.New().String(), tokenSecretName(id), desc, []byte(*req.Token), actor); cerr != nil {
				http.Error(w, "updated source but failed to store token: "+cerr.Error(), http.StatusInternalServerError)
				return
			}
		}
	}
	if req.WebhookSecret != nil && *req.WebhookSecret != "" {
		desc := "GitOps source " + req.Name + " webhook secret"
		if _, err := h.store.UpdateAppSecret(r.Context(), webhookSecretName(id), &desc, []byte(*req.WebhookSecret), actor); err != nil {
			if _, cerr := h.store.CreateAppSecret(r.Context(), uuid.New().String(), webhookSecretName(id), desc, []byte(*req.WebhookSecret), actor); cerr != nil {
				http.Error(w, "updated source but failed to store webhook secret: "+cerr.Error(), http.StatusInternalServerError)
				return
			}
		}
	}

	audit("GITOPS_SOURCE_UPDATED", r, "source_id", id, "name", req.Name)
	go h.reconcileOne(context.Background(), id, "update")
	writeJSON(w, http.StatusOK, updated)
}

func (h *GitOpsHandler) HandleDelete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := h.store.DeleteGitOpsSource(r.Context(), id); err != nil {
		code := http.StatusInternalServerError
		if strings.Contains(err.Error(), "not found") {
			code = http.StatusNotFound
		}
		http.Error(w, err.Error(), code)
		return
	}
	h.store.DeleteAppSecret(r.Context(), tokenSecretName(id))   //nolint:errcheck
	h.store.DeleteAppSecret(r.Context(), webhookSecretName(id)) //nolint:errcheck
	os.RemoveAll(h.repoWorkDir(id))                             //nolint:errcheck
	audit("GITOPS_SOURCE_DELETED", r, "source_id", id)
	w.WriteHeader(http.StatusNoContent)
}

func (h *GitOpsHandler) HandleListEvents(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	limit := 20
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			limit = n
		}
	}
	events, err := h.store.ListGitOpsSyncEvents(r.Context(), id, limit)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if events == nil {
		events = []store.GitOpsSyncEvent{}
	}
	writeJSON(w, http.StatusOK, events)
}

// ── Sync trigger — manual (authenticated UI call) or webhook (HMAC-signed,
// GitHub X-Hub-Signature-256 style) — both hit the same path, matching the
// issue's acceptance criterion exactly. Not wrapped in the usual require()
// middleware since a git host's webhook has no bearer token; auth is
// resolved inside the handler instead. ──────────────────────────────────

func (h *GitOpsHandler) HandleSync(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	source, err := h.store.GetGitOpsSource(r.Context(), id)
	if err != nil {
		http.Error(w, "gitops source not found", http.StatusNotFound)
		return
	}

	body, _ := io.ReadAll(io.LimitReader(r.Body, 1<<20))

	actor, authorized := h.authorizeSync(r, id, body)
	if !authorized {
		audit("GITOPS_SYNC_UNAUTHORIZED", r, "source_id", id, "name", source.Name)
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	audit("GITOPS_SYNC_TRIGGERED", r, "source_id", id, "name", source.Name, "actor", actor)
	go h.reconcileOne(context.Background(), id, actor)
	w.WriteHeader(http.StatusAccepted)
}

// authorizeSync accepts either a valid X-Hub-Signature-256 over the raw
// body (checked against this source's configured webhook secret — the
// public, unauthenticated path a real git host calls) or a normal Bearer
// JWT / X-API-Key with gitops:write (the UI's manual-trigger path).
func (h *GitOpsHandler) authorizeSync(r *http.Request, sourceID string, body []byte) (actor string, ok bool) {
	if sig := r.Header.Get("X-Hub-Signature-256"); sig != "" {
		secret, err := h.store.GetAppSecretValue(r.Context(), webhookSecretName(sourceID))
		if err == nil && VerifyGitHubSignature(secret, body, sig) {
			return "webhook", true
		}
		return "", false
	}

	if claims, ok := auth.ClaimsFromContext(r.Context()); ok {
		return claims.Username, true
	}

	if authHeader := r.Header.Get("Authorization"); strings.HasPrefix(authHeader, "Bearer ") {
		claims, err := auth.ValidateJWT(strings.TrimPrefix(authHeader, "Bearer "), h.jwtSecret)
		if err == nil {
			if allowed, _ := h.store.UserHasPermission(claims.UserID, "gitops", "write"); allowed {
				return claims.Username, true
			}
		}
	}
	if key := r.Header.Get("X-API-Key"); key != "" {
		userID, username, err := h.store.UserByAPIKey(auth.HashAPIKey(key))
		if err == nil {
			if allowed, _ := h.store.UserHasPermission(userID, "gitops", "write"); allowed {
				return username, true
			}
		}
	}
	return "", false
}

func VerifyGitHubSignature(secret, body []byte, header string) bool {
	const prefix = "sha256="
	if !strings.HasPrefix(header, prefix) {
		return false
	}
	mac := hmac.New(sha256.New, secret)
	mac.Write(body)
	expected := prefix + hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(header), []byte(expected))
}

// ── Reconciliation ──────────────────────────────────────────────────────

func (h *GitOpsHandler) repoWorkDir(sourceID string) string {
	return filepath.Join(h.dataDir, "gitops-repos", sourceID)
}

// reconcileOne syncs a single source by ID — used by manual/webhook
// triggers so they take effect immediately rather than waiting for the
// next reconciler tick.
func (h *GitOpsHandler) reconcileOne(ctx context.Context, sourceID, triggeredBy string) {
	source, err := h.store.GetGitOpsSource(ctx, sourceID)
	if err != nil {
		return
	}
	h.syncSource(ctx, source, triggeredBy)
}

// ReconcileDue checks every enabled source and syncs the ones whose poll
// interval has elapsed since their last sync. Called on a fixed ticker from
// main.go — see that file for the loop itself.
func (h *GitOpsHandler) ReconcileDue(ctx context.Context) {
	sources, err := h.store.ListGitOpsSources(ctx)
	if err != nil {
		handlerLogger.Warn("gitops reconcile: list sources failed", "error", err)
		return
	}
	for _, src := range sources {
		if !src.Enabled {
			continue
		}
		if !DueForSync(src) {
			continue
		}
		h.syncSource(ctx, src, "reconciler")
	}
}

func DueForSync(src store.GitOpsSource) bool {
	if src.LastSyncedAt == "" {
		return true
	}
	last, err := time.Parse(time.RFC3339, src.LastSyncedAt)
	if err != nil {
		// Some drivers format without the "T"/timezone suffix; fall back
		// to a couple of common layouts rather than refusing to ever sync.
		for _, layout := range []string{"2006-01-02 15:04:05", time.RFC3339Nano} {
			if last, err = time.Parse(layout, src.LastSyncedAt); err == nil {
				break
			}
		}
		if err != nil {
			return true
		}
	}
	interval := time.Duration(src.PollIntervalSeconds) * time.Second
	return time.Since(last) >= interval
}

// syncSource fetches the repo, applies the manifest if it changed, and
// records the outcome — every branch of this function ends in exactly one
// RecordGitOpsSync call, whether it synced, was a no-op, or failed.
func (h *GitOpsHandler) syncSource(ctx context.Context, source store.GitOpsSource, triggeredBy string) {
	event := store.GitOpsSyncEvent{ID: uuid.New().String(), SourceID: source.ID, TriggeredBy: triggeredBy}

	repo := &utils.GitRepo{URL: source.RepoURL, Branch: source.Branch, LocalDir: h.repoWorkDir(source.ID)}
	if source.Username != "" {
		if token, err := h.store.GetAppSecretValue(ctx, tokenSecretName(source.ID)); err == nil {
			repo.AuthUser = source.Username
			repo.AuthPass = string(token)
		}
	}

	result, err := gitops.Fetch(repo, h.repoWorkDir(source.ID), source.Path, source.LastContentHash)
	if err != nil {
		event.Status, event.Message = "error", err.Error()
		h.store.RecordGitOpsSync(ctx, event, source.LastCommitSHA, source.LastContentHash) //nolint:errcheck
		handlerLogger.Warn("gitops sync failed", "source_id", source.ID, "name", source.Name, "error", err)
		return
	}
	event.CommitSHA = result.CommitSHA

	if !result.Changed {
		event.Status, event.Message = "skipped", "no change since last sync"
		h.store.RecordGitOpsSync(ctx, event, result.CommitSHA, result.ContentHash) //nolint:errcheck
		return
	}

	output, applyErr := h.apply(ctx, source, result.Content)
	if applyErr != nil {
		event.Status, event.Message = "error", applyErr.Error()
		h.store.RecordGitOpsSync(ctx, event, result.CommitSHA, source.LastContentHash) //nolint:errcheck
		handlerLogger.Warn("gitops apply failed", "source_id", source.ID, "name", source.Name, "error", applyErr)
		return
	}

	event.Status, event.Message = "synced", output
	h.store.RecordGitOpsSync(ctx, event, result.CommitSHA, result.ContentHash) //nolint:errcheck
}

// apply deploys manifest either in-process on the hub (TargetNodeID empty)
// or relayed to the target node over the HMAC-signed hub-to-node channel.
func (h *GitOpsHandler) apply(ctx context.Context, source store.GitOpsSource, manifest []byte) (string, error) {
	if source.TargetNodeID == "" {
		if source.ManifestType == "kubernetes" {
			return h.k8sCtrl.applyYAMLLocally(ctx, manifest)
		}
		return h.dockerCtrl.applyComposeLocally(ctx, manifest)
	}

	node, err := h.store.GetNode(ctx, source.TargetNodeID)
	if err != nil {
		return "", fmt.Errorf("target node: %w", err)
	}
	path := "/api/docker/deploy-compose"
	if source.ManifestType == "kubernetes" {
		path = "/api/kubernetes/apply"
	}
	return relayApplyToNode(ctx, node, path, manifest)
}

func relayApplyToNode(ctx context.Context, node store.NodeRecord, path string, body []byte) (string, error) {
	host := normaliseAddress(node.Address)
	target := "https://" + host + path
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, target, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/yaml")
	if len(proxyHMACKey) > 0 {
		ts := strconv.FormatInt(time.Now().Unix(), 10)
		sig := hubRequestSig(proxyHMACKey, ts, http.MethodPost, path, body)
		req.Header.Set("X-Scutum-Hub-Ts", ts)
		req.Header.Set("X-Scutum-Hub-Sig", sig)
	}
	client := &http.Client{Timeout: 2 * time.Minute, Transport: &http.Transport{TLSClientConfig: proxyTLSConfig}}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("node returned status %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}
	return string(respBody), nil
}
