package alerts

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"time"

	"scutum/cmd/internal/sysinfo"
	"scutum/cmd/internal/store"
	"scutum/cmd/internal/webhooks"

	"github.com/google/uuid"
)

// EvaluatorStore is the subset of store.Store the evaluator needs.
type EvaluatorStore interface {
	ListAlertRules(ctx context.Context) ([]store.AlertRule, error)
	InsertAlertEvent(ctx context.Context, e store.AlertEvent) error
	OpenAlertEvent(ctx context.Context, ruleID string) (store.AlertEvent, error)
	ResolveAlertEvent(ctx context.Context, eventID, resolvedAt string) error
	WGPeerHandshakeAges(ctx context.Context) (map[string]time.Time, error)
}

// WebhookDispatcher matches the webhooks.Dispatcher.Send signature.
type WebhookDispatcher interface {
	Send(e webhooks.Event)
}

type Evaluator struct {
	store   EvaluatorStore
	webhook WebhookDispatcher
}

func NewEvaluator(s EvaluatorStore, w WebhookDispatcher) *Evaluator {
	return &Evaluator{store: s, webhook: w}
}

// Start runs the evaluation loop until ctx is cancelled.
func (e *Evaluator) Start(ctx context.Context) {
	tick := time.NewTicker(60 * time.Second)
	defer tick.Stop()
	// Run once immediately on startup.
	e.evaluate(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			e.evaluate(ctx)
		}
	}
}

func (e *Evaluator) evaluate(ctx context.Context) {
	rules, err := e.store.ListAlertRules(ctx)
	if err != nil {
		log.Printf("[alerts] list rules: %v", err)
		return
	}

	stats, err := sysinfo.Collect()
	if err != nil {
		log.Printf("[alerts] collect sysinfo: %v", err)
	}

	handshakes, err := e.store.WGPeerHandshakeAges(ctx)
	if err != nil {
		log.Printf("[alerts] wg handshake ages: %v", err)
	}

	now := time.Now().UTC()
	for _, r := range rules {
		if !r.Enabled {
			continue
		}
		// Honour silence window.
		if r.SilencedUntil != "" {
			if t, err := time.Parse(time.RFC3339, r.SilencedUntil); err == nil && now.Before(t) {
				continue
			}
		}

		firing, msg := e.check(r, stats, handshakes, now)
		e.reconcile(ctx, r, firing, msg, now)
	}
}

// check returns (isFiring, message) for a single rule given current state.
func (e *Evaluator) check(r store.AlertRule, stats sysinfo.Stats, handshakes map[string]time.Time, now time.Time) (bool, string) {
	switch r.Condition {
	case "cpu_percent":
		if stats.CPUPercent >= r.Threshold {
			return true, fmt.Sprintf("CPU usage %.1f%% ≥ threshold %.1f%%", stats.CPUPercent, r.Threshold)
		}
	case "mem_percent":
		if stats.MemPercent >= r.Threshold {
			return true, fmt.Sprintf("Memory usage %.1f%% ≥ threshold %.1f%%", stats.MemPercent, r.Threshold)
		}
	case "disk_percent":
		if stats.DiskPercent >= r.Threshold {
			return true, fmt.Sprintf("Disk usage %.1f%% ≥ threshold %.1f%%", stats.DiskPercent, r.Threshold)
		}
	case "node_offline":
		// Fires if ANY peer's last handshake is older than threshold minutes (or never seen).
		thresholdDur := time.Duration(r.Threshold) * time.Minute
		for nodeID, hs := range handshakes {
			if now.Sub(hs) > thresholdDur {
				return true, fmt.Sprintf("Node %s last handshake %.0f min ago (threshold %.0f min)", nodeID, now.Sub(hs).Minutes(), r.Threshold)
			}
		}
	case "handshake_age":
		thresholdDur := time.Duration(r.Threshold) * time.Minute
		for nodeID, hs := range handshakes {
			if now.Sub(hs) > thresholdDur {
				return true, fmt.Sprintf("WireGuard peer %s handshake age %.0f min ≥ %.0f min", nodeID, now.Sub(hs).Minutes(), r.Threshold)
			}
		}
	}
	return false, ""
}

// reconcile fires or resolves an alert event based on current state.
func (e *Evaluator) reconcile(ctx context.Context, r store.AlertRule, firing bool, msg string, now time.Time) {
	open, err := e.store.OpenAlertEvent(ctx, r.ID)
	hasOpen := err == nil
	notFound := errors.Is(err, sql.ErrNoRows)
	if err != nil && !notFound {
		log.Printf("[alerts] open event for rule %s: %v", r.ID, err)
		return
	}

	ts := now.Format(time.RFC3339)

	if firing && !hasOpen {
		ev := store.AlertEvent{
			ID:       uuid.New().String(),
			RuleID:   r.ID,
			RuleName: r.Name,
			Severity: r.Severity,
			Message:  msg,
			FiredAt:  ts,
		}
		if err := e.store.InsertAlertEvent(ctx, ev); err != nil {
			log.Printf("[alerts] insert event: %v", err)
			return
		}
		if e.webhook != nil {
			e.webhook.Send(webhooks.Event{Type: "alert.fired", Timestamp: now, Payload: map[string]any{
				"rule_id": ev.RuleID, "rule_name": ev.RuleName, "severity": ev.Severity, "message": ev.Message,
			}})
		}
		log.Printf("[alerts] fired %s: %s (%s)", r.Name, msg, r.Severity)
		return
	}

	if !firing && hasOpen && open.ResolvedAt == "" {
		if err := e.store.ResolveAlertEvent(ctx, open.ID, ts); err != nil {
			log.Printf("[alerts] resolve event: %v", err)
			return
		}
		open.ResolvedAt = ts
		if e.webhook != nil {
			e.webhook.Send(webhooks.Event{Type: "alert.resolved", Timestamp: now, Payload: map[string]any{
				"rule_id": open.RuleID, "rule_name": open.RuleName, "severity": open.Severity,
			}})
		}
		log.Printf("[alerts] resolved %s", r.Name)
	}
}