package handlers

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"scutum/cmd/internal/store"
	"scutum/cmd/internal/utils"
)

type complianceStore interface {
	ListAuditLogs(ctx context.Context, limit int) ([]utils.AuditEntry, error)
	ListNodes(ctx context.Context) ([]store.NodeRecord, error)
	ListUsers(ctx context.Context) ([]store.UserRecord, error)
	GetSecret(ctx context.Context, key string) ([]byte, error)
}

type ComplianceHandler struct {
	store   complianceStore
	version string
}

func NewComplianceHandler(s complianceStore, version string) *ComplianceHandler {
	return &ComplianceHandler{store: s, version: version}
}

// CRAReport is the structured output of a CRA compliance report.
type CRAReport struct {
	Meta        reportMeta        `json:"meta"`
	System      systemInfo        `json:"system"`
	Users       userSummary       `json:"users"`
	Mesh        meshSummary       `json:"mesh"`
	Audit       auditSummary      `json:"audit"`
	Security    securitySummary   `json:"security"`
	KeyMgmt     keyMgmtStatus     `json:"key_management"`
	Incidents   []incidentEvent   `json:"incidents"`
}

type reportMeta struct {
	GeneratedAt time.Time `json:"generated_at"`
	Version     string    `json:"version"`
	TimeRangeFrom *time.Time `json:"time_range_from,omitempty"`
	TimeRangeTo   *time.Time `json:"time_range_to,omitempty"`
	Standard    string    `json:"standard"`
}

type systemInfo struct {
	Version     string `json:"version"`
	AuditEnabled bool  `json:"audit_enabled"`
}

type userSummary struct {
	Total int         `json:"total"`
	Users []userEntry `json:"users"`
}

type userEntry struct {
	ID        string `json:"id"`
	Username  string `json:"username"`
	CreatedAt string `json:"created_at"`
}

type meshSummary struct {
	TotalNodes int              `json:"total_nodes"`
	Nodes      []store.NodeRecord `json:"nodes"`
}

type auditSummary struct {
	TotalEvents   int            `json:"total_events"`
	ByOutcome     map[string]int `json:"by_outcome"`
	ByAction      map[string]int `json:"by_action"`
	FailedLogins  int            `json:"failed_logins"`
	PermDenials   int            `json:"permission_denials"`
}

type securitySummary struct {
	MeshEncryption   string `json:"mesh_encryption"`
	AuthMethods      []string `json:"auth_methods"`
	RateLimitingEnabled bool `json:"rate_limiting_enabled"`
	AuditRetentionDays int  `json:"audit_retention_days"`
}

type keyMgmtStatus struct {
	HubHMACKeyPresent bool `json:"hub_hmac_key_present"`
	WG0ConfigPresent  bool `json:"wg0_config_present"`
}

type incidentEvent struct {
	Time     time.Time `json:"time"`
	Action   string    `json:"action"`
	Actor    string    `json:"actor"`
	Outcome  string    `json:"outcome"`
	ClientIP string    `json:"client_ip"`
	Path     string    `json:"path"`
}

func (h *ComplianceHandler) HandleReport(w http.ResponseWriter, r *http.Request) {
	format := r.URL.Query().Get("format")
	if format == "" {
		format = "json"
	}

	ctx := r.Context()

	auditLogs, err := h.store.ListAuditLogs(ctx, 10000)
	if err != nil {
		http.Error(w, "failed to load audit logs", http.StatusInternalServerError)
		return
	}

	nodes, _ := h.store.ListNodes(ctx)
	users, _ := h.store.ListUsers(ctx)

	report := h.buildReport(ctx, auditLogs, nodes, users)

	switch format {
	case "csv":
		h.writeCSV(w, auditLogs)
	case "text":
		h.writeText(w, report)
	default:
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Disposition", `attachment; filename="cra-compliance-report.json"`)
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		enc.Encode(report)
	}
}

func (h *ComplianceHandler) buildReport(ctx context.Context, logs []utils.AuditEntry, nodes []store.NodeRecord, users []store.UserRecord) CRAReport {
	now := time.Now().UTC()

	// Audit summary
	byOutcome := map[string]int{}
	byAction := map[string]int{}
	var failedLogins, permDenials int
	var incidents []incidentEvent
	for _, e := range logs {
		outcome := e.Outcome
		if outcome == "" {
			outcome = "success"
		}
		byOutcome[outcome]++
		byAction[e.Action]++
		if e.Outcome == utils.OutcomeFailure || e.Outcome == "denied" {
			if strings.Contains(strings.ToLower(e.Action), "login") {
				failedLogins++
			}
			if strings.Contains(strings.ToLower(e.Action), "perm") || strings.Contains(strings.ToLower(e.Path), "forbidden") {
				permDenials++
			}
			incidents = append(incidents, incidentEvent{
				Time:     e.Time,
				Action:   e.Action,
				Actor:    e.Actor,
				Outcome:  outcome,
				ClientIP: e.ClientIP,
				Path:     e.Path,
			})
		}
	}

	// User summary
	userEntries := make([]userEntry, 0, len(users))
	for _, u := range users {
		userEntries = append(userEntries, userEntry{
			ID:        u.ID,
			Username:  u.Username,
			CreatedAt: u.CreatedAt.Format(time.RFC3339),
		})
	}

	// Key management
	_, hmacErr := h.store.GetSecret(ctx, "hub_hmac_key")
	_, wgErr := h.store.GetSecret(ctx, "wg0_config")

	retentionDays := 365

	return CRAReport{
		Meta: reportMeta{
			GeneratedAt: now,
			Version:     h.version,
			Standard:    "EU Cyber Resilience Act (CRA) 2024/2847",
		},
		System: systemInfo{
			Version:      h.version,
			AuditEnabled: true,
		},
		Users: userSummary{
			Total: len(users),
			Users: userEntries,
		},
		Mesh: meshSummary{
			TotalNodes: len(nodes),
			Nodes:      nodes,
		},
		Audit: auditSummary{
			TotalEvents:  len(logs),
			ByOutcome:    byOutcome,
			ByAction:     byAction,
			FailedLogins: failedLogins,
			PermDenials:  permDenials,
		},
		Security: securitySummary{
			MeshEncryption:      "WireGuard (ChaCha20Poly1305 + Curve25519)",
			AuthMethods:         []string{"password+TOTP", "API key", "SSO (OIDC/OAuth2)"},
			RateLimitingEnabled: true,
			AuditRetentionDays:  retentionDays,
		},
		KeyMgmt: keyMgmtStatus{
			HubHMACKeyPresent: hmacErr == nil,
			WG0ConfigPresent:  wgErr == nil,
		},
		Incidents: incidents,
	}
}

func (h *ComplianceHandler) writeCSV(w http.ResponseWriter, logs []utils.AuditEntry) {
	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", `attachment; filename="audit-log.csv"`)
	cw := csv.NewWriter(w)
	cw.Write([]string{"time", "action", "actor", "actor_id", "outcome", "method", "path", "client_ip", "trace_id"})
	for _, e := range logs {
		outcome := e.Outcome
		if outcome == "" {
			outcome = "success"
		}
		cw.Write([]string{
			e.Time.Format(time.RFC3339),
			e.Action, e.Actor, e.ActorID, outcome,
			e.Method, e.Path, e.ClientIP, e.TraceID,
		})
	}
	cw.Flush()
}

func (h *ComplianceHandler) writeText(w http.ResponseWriter, r CRAReport) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="cra-compliance-report.txt"`)
	fmt.Fprintf(w, "CRA COMPLIANCE REPORT\n")
	fmt.Fprintf(w, "Standard : %s\n", r.Meta.Standard)
	fmt.Fprintf(w, "Generated: %s\n", r.Meta.GeneratedAt.Format(time.RFC3339))
	fmt.Fprintf(w, "Version  : %s\n\n", r.Meta.Version)

	fmt.Fprintf(w, "=== USERS ===\n")
	fmt.Fprintf(w, "Total: %d\n\n", r.Users.Total)
	for _, u := range r.Users.Users {
		fmt.Fprintf(w, "  %-30s %s\n", u.Username, u.CreatedAt)
	}

	fmt.Fprintf(w, "\n=== MESH NODES ===\n")
	fmt.Fprintf(w, "Total: %d\n\n", r.Mesh.TotalNodes)
	for _, n := range r.Mesh.Nodes {
		fmt.Fprintf(w, "  %-20s %-10s %s\n", n.Name, n.Type, n.Address)
	}

	fmt.Fprintf(w, "\n=== AUDIT SUMMARY ===\n")
	fmt.Fprintf(w, "Total events   : %d\n", r.Audit.TotalEvents)
	fmt.Fprintf(w, "Failed logins  : %d\n", r.Audit.FailedLogins)
	fmt.Fprintf(w, "Perm denials   : %d\n\n", r.Audit.PermDenials)
	fmt.Fprintf(w, "By outcome:\n")
	for k, v := range r.Audit.ByOutcome {
		fmt.Fprintf(w, "  %-20s %d\n", k, v)
	}

	fmt.Fprintf(w, "\n=== SECURITY ===\n")
	fmt.Fprintf(w, "Mesh encryption: %s\n", r.Security.MeshEncryption)
	fmt.Fprintf(w, "Auth methods   : %s\n", strings.Join(r.Security.AuthMethods, ", "))
	fmt.Fprintf(w, "Audit retention: %d days\n", r.Security.AuditRetentionDays)

	if len(r.Incidents) > 0 {
		fmt.Fprintf(w, "\n=== SECURITY INCIDENTS (%d) ===\n", len(r.Incidents))
		for _, inc := range r.Incidents {
			fmt.Fprintf(w, "  %s  %-25s %-10s %-15s %s\n",
				inc.Time.Format("2006-01-02 15:04:05"), inc.Action, inc.Outcome, inc.ClientIP, inc.Path)
		}
	}
}
