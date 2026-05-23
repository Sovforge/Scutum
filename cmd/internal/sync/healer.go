package sync

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"scutum/cmd/internal/metrics"
)

// ---------------------------------------------------------------------------
// Service registry
// ---------------------------------------------------------------------------

// HealthCheck returns nil if the service is healthy.
type HealthCheck func(ctx context.Context) error

// RestartFn restarts the service. It is called when a health check fails.
type RestartFn func(ctx context.Context) error

// ServiceEntry is a named service the healer monitors.
type ServiceEntry struct {
	Name    string
	Check   HealthCheck
	Restart RestartFn
}

// ---------------------------------------------------------------------------
// WireGuard peer registry
// ---------------------------------------------------------------------------

// WGPeer is a WireGuard peer the healer monitors.
type WGPeer struct {
	IfaceName  string
	PublicKey  string
	Endpoint   string
	AllowedIPs string
	// FreshEndpoint, if non-nil, is called when the peer's handshake is stale to
	// fetch the latest endpoint from persistent storage. If it returns an endpoint
	// that differs from the one last used for a re-add, the healer re-adds the peer
	// with the new endpoint. This lets registered endpoint updates (from
	// HandleRegisterEndpoint) propagate into WireGuard without overwriting runtime-
	// learned endpoints when nothing has changed.
	FreshEndpoint func(ctx context.Context) (string, error)
}

// WGChecker checks if a WireGuard peer is reachable and re-adds it if not.
// Implement using utils.GetStatus / utils.AddPeer.
type WGChecker interface {
	// PeerHandshakeAge returns how long ago the peer last completed a
	// handshake. Returns an error if the peer is unknown.
	PeerHandshakeAge(ctx context.Context, ifaceName, publicKey string) (time.Duration, error)
	// ReAddPeer removes and re-adds the peer, triggering a fresh handshake.
	ReAddPeer(ctx context.Context, peer WGPeer) error
}

// ---------------------------------------------------------------------------
// Healer config
// ---------------------------------------------------------------------------

// HealerConfig controls timing and thresholds.
type HealerConfig struct {
	// Interval between health check rounds. Default 30s.
	Interval time.Duration

	// HandshakeMaxAge is how old a WireGuard handshake can be before the
	// peer is considered down. Default 3 minutes (WG itself uses ~2m30s).
	HandshakeMaxAge time.Duration

	// MaxRestartBackoff caps exponential backoff for service restarts.
	MaxRestartBackoff time.Duration

	// CheckTimeout is the deadline for a single health check. Default 10s.
	CheckTimeout time.Duration

	// Logger is used for all healer output. Defaults to slog.Default().
	Logger *slog.Logger
}

func (c *HealerConfig) setDefaults() {
	if c.Interval == 0 {
		c.Interval = 30 * time.Second
	}
	if c.HandshakeMaxAge == 0 {
		c.HandshakeMaxAge = 3 * time.Minute
	}
	if c.MaxRestartBackoff == 0 {
		c.MaxRestartBackoff = 5 * time.Minute
	}
	if c.CheckTimeout == 0 {
		c.CheckTimeout = 10 * time.Second
	}
	if c.Logger == nil {
		c.Logger = slog.Default()
	}
}

// ---------------------------------------------------------------------------
// Healer
// ---------------------------------------------------------------------------

// Healer runs a background loop that monitors WireGuard peers and registered
// services, healing faults as they are detected.
// It is safe for concurrent use.
type Healer struct {
	cfg              HealerConfig
	wgCheck          WGChecker
	mu               sync.Mutex
	peers            []WGPeer
	services         []ServiceEntry
	backoffs         map[string]time.Duration
	lastAttemptAt    map[string]time.Time
	lastUsedEndpoint map[string]string // key: "ifaceName:pubkey"
	cancel           context.CancelFunc
	stopped          sync.WaitGroup
}

// NewHealer creates a Healer. wgCheck may be nil if you don't need WireGuard
// healing (service-only mode).
func NewHealer(cfg HealerConfig, wgCheck WGChecker) *Healer {
	cfg.setDefaults()
	return &Healer{
		cfg:              cfg,
		wgCheck:          wgCheck,
		backoffs:         make(map[string]time.Duration),
		lastAttemptAt:    make(map[string]time.Time),
		lastUsedEndpoint: make(map[string]string),
	}
}

// AddPeer registers a WireGuard peer to monitor.
func (h *Healer) AddPeer(p WGPeer) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.peers = append(h.peers, p)
}

// AddService registers a service to monitor.
func (h *Healer) AddService(s ServiceEntry) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.services = append(h.services, s)
}

// Start begins the healing loop. Call Stop to shut it down cleanly.
func (h *Healer) Start(ctx context.Context) {
	ctx, h.cancel = context.WithCancel(ctx)
	h.stopped.Add(1)
	go func() {
		defer h.stopped.Done()
		h.loop(ctx)
	}()
}

// Stop shuts down the healing loop and waits for the current round to finish.
func (h *Healer) Stop() {
	if h.cancel != nil {
		h.cancel()
	}
	h.stopped.Wait()
}

func (h *Healer) loop(ctx context.Context) {
	ticker := time.NewTicker(h.cfg.Interval)
	defer ticker.Stop()
	// Run once immediately on start.
	h.round(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			h.round(ctx)
		}
	}
}

func (h *Healer) round(ctx context.Context) {
	h.mu.Lock()
	peers := append([]WGPeer(nil), h.peers...)
	services := append([]ServiceEntry(nil), h.services...)
	h.mu.Unlock()

	var wg sync.WaitGroup

	// Check WireGuard peers.
	if h.wgCheck != nil {
		for _, peer := range peers {
			wg.Add(1)
			go func(p WGPeer) {
				defer wg.Done()
				h.healPeer(ctx, p)
			}(peer)
		}
	}

	// Check services.
	for _, svc := range services {
		wg.Add(1)
		go func(s ServiceEntry) {
			defer wg.Done()
			h.healService(ctx, s)
		}(svc)
	}

	wg.Wait()
}

// ---------------------------------------------------------------------------
// WireGuard healing
// ---------------------------------------------------------------------------

func (h *Healer) healPeer(ctx context.Context, peer WGPeer) {
	checkCtx, cancel := context.WithTimeout(ctx, h.cfg.CheckTimeout)
	defer cancel()

	age, err := h.wgCheck.PeerHandshakeAge(checkCtx, peer.IfaceName, peer.PublicKey)
	if err != nil {
		h.cfg.Logger.Warn("wg peer unknown, re-adding",
			"iface", peer.IfaceName,
			"peer", safeTruncate(peer.PublicKey, 8),
			"err", err,
		)
		metrics.HealerCheckTotal.WithLabelValues("peer_missing").Inc()
		if err := h.wgCheck.ReAddPeer(ctx, peer); err != nil {
			h.cfg.Logger.Error("wg re-add failed",
				"iface", peer.IfaceName,
				"peer", safeTruncate(peer.PublicKey, 8),
				"err", err,
			)
		}
		return
	}

	if age > h.cfg.HandshakeMaxAge {
		metrics.HealerCheckTotal.WithLabelValues("peer_stale").Inc()

		if peer.FreshEndpoint == nil {
			// No endpoint source — let WireGuard's keepalive/rekey recover the tunnel.
			h.cfg.Logger.Warn("wg peer handshake stale",
				"iface", peer.IfaceName,
				"peer", safeTruncate(peer.PublicKey, 8),
				"age", age.Round(time.Second),
			)
			return
		}

		freshEP, err := peer.FreshEndpoint(checkCtx)
		if err != nil {
			h.cfg.Logger.Warn("wg peer handshake stale, could not fetch fresh endpoint",
				"iface", peer.IfaceName,
				"peer", safeTruncate(peer.PublicKey, 8),
				"age", age.Round(time.Second),
				"err", err,
			)
			return
		}

		peerKey := peer.IfaceName + ":" + peer.PublicKey
		h.mu.Lock()
		lastEP := h.lastUsedEndpoint[peerKey]
		h.mu.Unlock()

		if freshEP == lastEP {
			// Endpoint unchanged since last re-add; WireGuard's keepalive will learn
			// the real endpoint from incoming packets — don't overwrite it.
			h.cfg.Logger.Warn("wg peer handshake stale",
				"iface", peer.IfaceName,
				"peer", safeTruncate(peer.PublicKey, 8),
				"age", age.Round(time.Second),
			)
			return
		}

		// Endpoint was updated by the register-endpoint mechanism — re-add safely.
		updated := peer
		updated.Endpoint = freshEP
		if err := h.wgCheck.ReAddPeer(ctx, updated); err != nil {
			h.cfg.Logger.Error("wg re-add with fresh endpoint failed",
				"iface", peer.IfaceName,
				"peer", safeTruncate(peer.PublicKey, 8),
				"err", err,
			)
			return
		}
		h.mu.Lock()
		h.lastUsedEndpoint[peerKey] = freshEP
		h.mu.Unlock()
		h.cfg.Logger.Info("wg peer re-added with updated endpoint",
			"iface", peer.IfaceName,
			"peer", safeTruncate(peer.PublicKey, 8),
			"endpoint", freshEP,
		)
	}
}

// ---------------------------------------------------------------------------
// Service healing
// ---------------------------------------------------------------------------

func (h *Healer) healService(ctx context.Context, svc ServiceEntry) {
	checkCtx, cancel := context.WithTimeout(ctx, h.cfg.CheckTimeout)
	defer cancel()

	if err := svc.Check(checkCtx); err == nil {
		metrics.HealerCheckTotal.WithLabelValues("service_ok").Inc()
		h.mu.Lock()
		delete(h.backoffs, svc.Name)
		delete(h.lastAttemptAt, svc.Name)
		h.mu.Unlock()
		return
	}

	h.mu.Lock()
	backoff := h.backoffs[svc.Name]
	lastAttempt := h.lastAttemptAt[svc.Name]
	h.mu.Unlock()

	// Still within backoff window — skip this round.
	if !lastAttempt.IsZero() && time.Since(lastAttempt) < backoff {
		return
	}

	h.cfg.Logger.Warn("service unhealthy, restarting",
		"service", svc.Name,
		"backoff", backoff,
	)

	restartCtx, rcancel := context.WithTimeout(ctx, h.cfg.CheckTimeout)
	defer rcancel()

	metrics.HealerCheckTotal.WithLabelValues("service_fail").Inc()
	if err := svc.Restart(restartCtx); err != nil {
		h.cfg.Logger.Error("service restart failed",
			"service", svc.Name,
			"err", err,
		)
	} else {
		h.cfg.Logger.Info("service restarted", "service", svc.Name)
	}

	h.mu.Lock()
	next := backoff * 2
	if next == 0 {
		next = h.cfg.Interval
	}
	if next > h.cfg.MaxRestartBackoff {
		next = h.cfg.MaxRestartBackoff
	}
	h.backoffs[svc.Name] = next
	h.lastAttemptAt[svc.Name] = time.Now()
	h.mu.Unlock()
}

// ---------------------------------------------------------------------------
// WireGuard checker backed by utils package
// ---------------------------------------------------------------------------

// WGCommandRunner is the interface required by DefaultWGChecker.
type WGCommandRunner interface {
	Output(ctx context.Context, name string, args ...string) ([]byte, error)
}

// ContextRunner adapts a plain output func (e.g. utils.DefaultCommandRunner.Output)
// to WGCommandRunner. If ctx is cancelled before the command finishes,
// ContextRunner returns ctx.Err() without being able to kill the subprocess —
// wg CLI calls are short-lived so this is acceptable.
type ContextRunner struct {
	Fn func(name string, args ...string) ([]byte, error)
}

func (r ContextRunner) Output(ctx context.Context, name string, args ...string) ([]byte, error) {
	type res struct {
		b   []byte
		err error
	}
	ch := make(chan res, 1)
	go func() {
		b, err := r.Fn(name, args...)
		ch <- res{b, err}
	}()
	select {
	case v := <-ch:
		return v.b, v.err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// DefaultWGChecker implements WGChecker using the wg CLI via utils.
// Pass it a WGCommandRunner (e.g. ContextRunner{Fn: utils.DefaultCommandRunner.Output})
// so it is testable.
type DefaultWGChecker struct {
	Runner WGCommandRunner
}

// PeerHandshakeAge parses `wg show <iface> latest-handshakes` and returns
// how long ago the given peer last completed a handshake.
func (c *DefaultWGChecker) PeerHandshakeAge(ctx context.Context, ifaceName, publicKey string) (time.Duration, error) {
	out, err := c.Runner.Output(ctx, "wg", "show", ifaceName, "latest-handshakes")
	if err != nil {
		return 0, fmt.Errorf("wg show: %w", err)
	}
	// Output format: "<pubkey>\t<unix timestamp>\n"
	lines := splitLines(string(out))
	for _, line := range lines {
		fields := splitFields(line)
		if len(fields) < 2 {
			continue
		}
		if fields[0] != publicKey {
			continue
		}
		var ts int64
		if _, err := fmt.Sscanf(fields[1], "%d", &ts); err != nil {
			return 0, fmt.Errorf("parse timestamp: %w", err)
		}
		if ts == 0 {
			return 999 * time.Hour, nil // never handshaked
		}
		return time.Since(time.Unix(ts, 0)), nil
	}
	return 0, fmt.Errorf("peer %s not found in wg output", safeTruncate(publicKey, 8))
}

// ReAddPeer removes and re-adds the peer, triggering a new handshake.
func (c *DefaultWGChecker) ReAddPeer(ctx context.Context, peer WGPeer) error {
	// Remove first (ignore error — peer may not exist).
	c.Runner.Output(ctx, "wg", "set", peer.IfaceName, "peer", peer.PublicKey, "remove")
	_, err := c.Runner.Output(ctx, "wg", "set", peer.IfaceName,
		"peer", peer.PublicKey,
		"endpoint", peer.Endpoint,
		"allowed-ips", peer.AllowedIPs,
		"persistent-keepalive", "25",
	)
	return err
}

// ---------------------------------------------------------------------------
// Small string helpers (avoid importing strings to keep deps minimal)
// ---------------------------------------------------------------------------

func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			if i > start {
				lines = append(lines, s[start:i])
			}
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}

func splitFields(s string) []string {
	var fields []string
	inField := false
	start := 0
	for i := 0; i < len(s); i++ {
		isSpace := s[i] == ' ' || s[i] == '\t'
		if !inField && !isSpace {
			start = i
			inField = true
		} else if inField && isSpace {
			fields = append(fields, s[start:i])
			inField = false
		}
	}
	if inField {
		fields = append(fields, s[start:])
	}
	return fields
}

func safeTruncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

