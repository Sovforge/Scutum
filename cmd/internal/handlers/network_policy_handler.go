package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"scutum/cmd/internal/netpolicy"
	"scutum/cmd/internal/roaming"
	"scutum/cmd/internal/store"
	"scutum/cmd/internal/utils"

	"github.com/google/uuid"
)

var validActions = map[string]bool{"allow": true, "deny": true}
var validProtocols = map[string]bool{"tcp": true, "udp": true, "icmp": true, "any": true}
var validTargetTypes = map[string]bool{"any": true, "node": true, "group": true}

// networkPolicyStore narrows *store.Store to what this feature needs —
// policy/settings CRUD, plus enough of the node/group/peer model to
// validate targets at write time and resolve them to mesh IPs at apply time.
type networkPolicyStore interface {
	CreateNetworkPolicy(ctx context.Context, p store.NetworkPolicy) (store.NetworkPolicy, error)
	GetNetworkPolicy(ctx context.Context, id string) (store.NetworkPolicy, error)
	ListNetworkPolicies(ctx context.Context) ([]store.NetworkPolicy, error)
	UpdateNetworkPolicy(ctx context.Context, p store.NetworkPolicy) (store.NetworkPolicy, error)
	DeleteNetworkPolicy(ctx context.Context, id string) error
	GetNetworkDefaultDeny(ctx context.Context) (bool, error)
	SetNetworkDefaultDeny(ctx context.Context, defaultDeny bool, updatedBy string) error

	GetNode(ctx context.Context, id string) (store.NodeRecord, error)
	GetNodeGroup(ctx context.Context, id string) (store.NodeGroup, error)
	ListNodesInGroup(ctx context.Context, groupID string) ([]store.NodeRecord, error)
	GetWGPeer(ctx context.Context, nodeID string) (store.WGPeerRecord, error)
	ListNodes(ctx context.Context) ([]store.NodeRecord, error)
}

type NetworkPolicyHandler struct {
	store networkPolicyStore
	iface string
}

func NewNetworkPolicyHandler(st networkPolicyStore, iface string) *NetworkPolicyHandler {
	return &NetworkPolicyHandler{store: st, iface: iface}
}

// ── CRUD ─────────────────────────────────────────────────────────────────

func (h *NetworkPolicyHandler) HandleList(w http.ResponseWriter, r *http.Request) {
	policies, err := h.store.ListNetworkPolicies(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if policies == nil {
		policies = []store.NetworkPolicy{}
	}
	writeJSON(w, http.StatusOK, policies)
}

type policyReq struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Enabled     *bool  `json:"enabled"`
	Priority    int    `json:"priority"`
	Action      string `json:"action"`
	Protocol    string `json:"protocol"`
	Port        string `json:"port"`
	SrcType     string `json:"src_type"`
	SrcID       string `json:"src_id"`
	DstType     string `json:"dst_type"`
	DstID       string `json:"dst_id"`
}

func (h *NetworkPolicyHandler) validate(r *http.Request, req policyReq) error {
	if req.Name == "" {
		return fmt.Errorf("name is required")
	}
	if !validActions[req.Action] {
		return fmt.Errorf(`action must be "allow" or "deny"`)
	}
	if req.Protocol == "" {
		req.Protocol = "any"
	}
	if !validProtocols[req.Protocol] {
		return fmt.Errorf(`protocol must be "tcp", "udp", "icmp", or "any"`)
	}
	if err := h.validateTarget(r, req.SrcType, req.SrcID); err != nil {
		return fmt.Errorf("source: %w", err)
	}
	if err := h.validateTarget(r, req.DstType, req.DstID); err != nil {
		return fmt.Errorf("destination: %w", err)
	}
	return nil
}

func (h *NetworkPolicyHandler) validateTarget(r *http.Request, targetType, targetID string) error {
	if targetType == "" {
		targetType = "any"
	}
	if !validTargetTypes[targetType] {
		return fmt.Errorf(`type must be "any", "node", or "group"`)
	}
	if targetType == "any" {
		return nil
	}
	if targetID == "" {
		return fmt.Errorf("id is required for type %q", targetType)
	}
	var err error
	if targetType == "node" {
		_, err = h.store.GetNode(r.Context(), targetID)
	} else {
		_, err = h.store.GetNodeGroup(r.Context(), targetID)
	}
	if err != nil {
		return fmt.Errorf("%s %q not found", targetType, targetID)
	}
	return nil
}

func (h *NetworkPolicyHandler) HandleCreate(w http.ResponseWriter, r *http.Request) {
	var req policyReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.Protocol == "" {
		req.Protocol = "any"
	}
	if req.SrcType == "" {
		req.SrcType = "any"
	}
	if req.DstType == "" {
		req.DstType = "any"
	}
	if err := h.validate(r, req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}

	actor := actorFromRequest(r)
	p := store.NetworkPolicy{
		ID: uuid.New().String(), Name: req.Name, Description: req.Description, Enabled: enabled,
		Priority: req.Priority, Action: req.Action, Protocol: req.Protocol, Port: req.Port,
		SrcType: req.SrcType, SrcID: req.SrcID, DstType: req.DstType, DstID: req.DstID, CreatedBy: actor,
	}
	created, err := h.store.CreateNetworkPolicy(r.Context(), p)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	audit("NETWORK_POLICY_CREATED", r, "policy_id", created.ID, "name", created.Name, "action", created.Action)
	go Reconcile(context.Background(), h.store, h.iface)
	writeJSON(w, http.StatusCreated, created)
}

func (h *NetworkPolicyHandler) HandleUpdate(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req policyReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.Protocol == "" {
		req.Protocol = "any"
	}
	if req.SrcType == "" {
		req.SrcType = "any"
	}
	if req.DstType == "" {
		req.DstType = "any"
	}
	if err := h.validate(r, req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}

	p := store.NetworkPolicy{
		ID: id, Name: req.Name, Description: req.Description, Enabled: enabled,
		Priority: req.Priority, Action: req.Action, Protocol: req.Protocol, Port: req.Port,
		SrcType: req.SrcType, SrcID: req.SrcID, DstType: req.DstType, DstID: req.DstID,
		UpdatedBy: actorFromRequest(r),
	}
	updated, err := h.store.UpdateNetworkPolicy(r.Context(), p)
	if err != nil {
		code := http.StatusInternalServerError
		if strings.Contains(err.Error(), "not found") {
			code = http.StatusNotFound
		}
		http.Error(w, err.Error(), code)
		return
	}

	audit("NETWORK_POLICY_UPDATED", r, "policy_id", updated.ID, "name", updated.Name)
	go Reconcile(context.Background(), h.store, h.iface)
	writeJSON(w, http.StatusOK, updated)
}

func (h *NetworkPolicyHandler) HandleDelete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := h.store.DeleteNetworkPolicy(r.Context(), id); err != nil {
		code := http.StatusInternalServerError
		if strings.Contains(err.Error(), "not found") {
			code = http.StatusNotFound
		}
		http.Error(w, err.Error(), code)
		return
	}
	audit("NETWORK_POLICY_DELETED", r, "policy_id", id)
	go Reconcile(context.Background(), h.store, h.iface)
	w.WriteHeader(http.StatusNoContent)
}

// ── Default-deny setting ────────────────────────────────────────────────

func (h *NetworkPolicyHandler) HandleGetSettings(w http.ResponseWriter, r *http.Request) {
	defaultDeny, err := h.store.GetNetworkDefaultDeny(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"default_deny":       defaultDeny,
		"iptables_available": netpolicy.IsAvailable(),
	})
}

func (h *NetworkPolicyHandler) HandleUpdateSettings(w http.ResponseWriter, r *http.Request) {
	var req struct {
		DefaultDeny bool `json:"default_deny"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if err := h.store.SetNetworkDefaultDeny(r.Context(), req.DefaultDeny, actorFromRequest(r)); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	audit("NETWORK_POLICY_DEFAULT_DENY_CHANGED", r, "default_deny", req.DefaultDeny)
	go Reconcile(context.Background(), h.store, h.iface)
	w.WriteHeader(http.StatusNoContent)
}

// ── Local apply (relay target — called by the hub's reconciler on itself
// in-process, and over the HMAC-signed hub-to-node channel on every other
// node) ──────────────────────────────────────────────────────────────────

type applyPolicyReq struct {
	Rules       []netpolicy.Rule `json:"rules"`
	DefaultDeny bool             `json:"default_deny"`
}

func (h *NetworkPolicyHandler) HandleApplyLocal(w http.ResponseWriter, r *http.Request) {
	var req applyPolicyReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if !netpolicy.IsAvailable() {
		http.Error(w, "iptables not available on this node", http.StatusServiceUnavailable)
		return
	}
	if err := netpolicy.Apply(utils.DefaultCommandRunner, h.iface, req.Rules, req.DefaultDeny); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	audit("NETWORK_POLICY_APPLIED", r, "rule_count", len(req.Rules), "default_deny", req.DefaultDeny)
	w.WriteHeader(http.StatusNoContent)
}

// ── Resolution + reconciliation ─────────────────────────────────────────

// policyResolver implements netpolicy.Resolver against the store.
type policyResolver struct{ store networkPolicyStore }

func (pr *policyResolver) ResolveTarget(ctx context.Context, targetType, targetID string) ([]string, error) {
	switch targetType {
	case "", "any":
		return nil, nil
	case "node":
		peer, err := pr.store.GetWGPeer(ctx, targetID)
		if err != nil {
			return nil, err
		}
		ip := roaming.HubMeshIP(peer.AllowedIPs)
		if ip == "" {
			return nil, fmt.Errorf("node %s has no resolvable single-host mesh IP", targetID)
		}
		return []string{ip}, nil
	case "group":
		nodes, err := pr.store.ListNodesInGroup(ctx, targetID)
		if err != nil {
			return nil, err
		}
		var ips []string
		for _, n := range nodes {
			peer, err := pr.store.GetWGPeer(ctx, n.ID)
			if err != nil {
				continue // best-effort: a node with no established peer yet just isn't matched
			}
			if ip := roaming.HubMeshIP(peer.AllowedIPs); ip != "" {
				ips = append(ips, ip)
			}
		}
		return ips, nil
	default:
		return nil, fmt.Errorf("unknown target type %q", targetType)
	}
}

// Reconcile recomputes the full rule set from the current policies and
// pushes it everywhere: applied in-process for the hub itself, relayed over
// the HMAC-signed hub-to-node channel to every other node. Best-effort
// throughout — one node being unreachable, or iptables missing on a given
// host, never stops the others from being reconciled, and is only ever
// logged/audited, never returned as a hard failure to an API caller (this
// always runs in its own goroutine).
func Reconcile(ctx context.Context, st networkPolicyStore, iface string) {
	if !netpolicy.IsAvailable() {
		return
	}
	policies, err := st.ListNetworkPolicies(ctx)
	if err != nil {
		handlerLogger.Warn("network policy reconcile: list policies failed", "error", err)
		return
	}
	defaultDeny, err := st.GetNetworkDefaultDeny(ctx)
	if err != nil {
		handlerLogger.Warn("network policy reconcile: get default-deny failed", "error", err)
		return
	}

	resolver := &policyResolver{store: st}
	rules, resolveErrs := netpolicy.BuildRules(ctx, resolver, policies)
	for policyID, rerr := range resolveErrs {
		handlerLogger.Warn("network policy reconcile: could not resolve rule", "policy_id", policyID, "error", rerr)
	}

	if err := netpolicy.Apply(utils.DefaultCommandRunner, iface, rules, defaultDeny); err != nil {
		handlerLogger.Error("network policy reconcile: local apply failed", "error", err)
	}

	nodes, err := st.ListNodes(ctx)
	if err != nil {
		handlerLogger.Warn("network policy reconcile: list nodes failed", "error", err)
		return
	}
	body, err := json.Marshal(applyPolicyReq{Rules: rules, DefaultDeny: defaultDeny})
	if err != nil {
		handlerLogger.Error("network policy reconcile: marshal rules failed", "error", err)
		return
	}
	for _, n := range nodes {
		if n.Type == "hub" {
			continue // already applied in-process above
		}
		if err := pushPolicyToNode(ctx, n, body); err != nil {
			handlerLogger.Warn("network policy reconcile: push to node failed", "node_id", n.ID, "node_name", n.Name, "error", err)
		}
	}
}

const applyPolicyPath = "/api/network/policy/apply"

func pushPolicyToNode(ctx context.Context, node store.NodeRecord, body []byte) error {
	host := normaliseAddress(node.Address)
	target := "https://" + host + applyPolicyPath
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, target, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if len(proxyHMACKey) > 0 {
		ts := strconv.FormatInt(time.Now().Unix(), 10)
		sig := hubRequestSig(proxyHMACKey, ts, http.MethodPost, applyPolicyPath, body)
		req.Header.Set("X-Scutum-Hub-Ts", ts)
		req.Header.Set("X-Scutum-Hub-Sig", sig)
	}
	client := &http.Client{Timeout: 15 * time.Second, Transport: &http.Transport{TLSClientConfig: proxyTLSConfig}}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("status %d: %s", resp.StatusCode, strings.TrimSpace(string(b)))
	}
	return nil
}
