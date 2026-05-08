package sync

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"bytes"
	"sync"
	"time"

	"scutum/cmd/internal/metrics"
)

// ---------------------------------------------------------------------------
// Payload
// ---------------------------------------------------------------------------

// SyncPayload is what the hub pushes to every edge on any config change.
// It is signed with an HMAC so edges can verify it came from the hub.
type SyncPayload struct {
	Version   int64       `json:"version"` // monotonic; edge rejects if <= its last seen
	Peers     []PeerEntry `json:"peers"`   // full WireGuard peer list
	Plugins   []string    `json:"plugins"` // enabled plugin names
	IssuedAt  time.Time   `json:"issued_at"`
	Signature []byte      `json:"sig,omitempty"` // HMAC-SHA256, set by Sign()
}

// PeerEntry is a single WireGuard peer the edge should have configured.
type PeerEntry struct {
	PublicKey  string `json:"public_key"`
	Endpoint   string `json:"endpoint"`
	AllowedIPs string `json:"allowed_ips"`
}

// sign computes an HMAC-SHA256 over the payload fields (excluding Signature)
// and sets p.Signature. key is the pre-shared edge token.
func (p *SyncPayload) sign(key []byte) error {
	p.Signature = nil
	body, err := json.Marshal(p)
	if err != nil {
		return err
	}
	mac := hmac.New(sha256.New, key)
	mac.Write(body)
	p.Signature = mac.Sum(nil)
	return nil
}

// Verify checks the HMAC. Returns nil if the signature is valid.
func (p *SyncPayload) Verify(key []byte) error {
	sig := p.Signature
	p.Signature = nil
	body, err := json.Marshal(p)
	p.Signature = sig
	if err != nil {
		return err
	}
	mac := hmac.New(sha256.New, key)
	mac.Write(body)
	expected := mac.Sum(nil)
	if !hmac.Equal(sig, expected) {
		return fmt.Errorf("payload signature invalid")
	}
	return nil
}

// ---------------------------------------------------------------------------
// Edge registry
// ---------------------------------------------------------------------------

// EdgeSink is anything that can receive a push from the hub.
// Implement this for WebSocket connections, HTTP endpoints, etc.
type EdgeSink interface {
	// NodeID uniquely identifies this edge.
	NodeID() string
	// Send delivers the payload to the edge. Returns an error if delivery
	// failed and the caller should retry.
	Send(ctx context.Context, p SyncPayload) error
}

// ---------------------------------------------------------------------------
// Pusher
// ---------------------------------------------------------------------------

// PushConfig controls retry behaviour.
type PushConfig struct {
	// HMACKey is the pre-shared secret used to sign payloads.
	// Each edge should share the same key with the hub, or you can
	// use per-edge keys by implementing EdgeSink.Send to pick the right key.
	HMACKey []byte

	// MaxRetries per edge per push attempt. Default 3.
	MaxRetries int

	// RetryBase is the initial backoff duration. Default 500ms.
	RetryBase time.Duration

	// PushTimeout is the per-edge deadline for a single Send attempt.
	PushTimeout time.Duration
}

func (c *PushConfig) setDefaults() {
	if c.MaxRetries == 0 {
		c.MaxRetries = 3
	}
	if c.RetryBase == 0 {
		c.RetryBase = 500 * time.Millisecond
	}
	if c.PushTimeout == 0 {
		c.PushTimeout = 10 * time.Second
	}
}

// Pusher fans out signed SyncPayloads to all registered edges.
// It is safe for concurrent use.
type Pusher struct {
	cfg   PushConfig
	mu    sync.RWMutex
	edges map[string]EdgeSink
}

// NewPusher creates a Pusher with the given config.
func NewPusher(cfg PushConfig) *Pusher {
	cfg.setDefaults()
	return &Pusher{
		cfg:   cfg,
		edges: make(map[string]EdgeSink),
	}
}

// Register adds an edge sink. If an edge with the same NodeID is already
// registered it is replaced (e.g. on reconnect).
func (p *Pusher) Register(sink EdgeSink) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.edges[sink.NodeID()] = sink
}

// Deregister removes an edge (e.g. on disconnect).
func (p *Pusher) Deregister(nodeID string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.edges, nodeID)
}

// EdgeCount returns the number of currently registered edges.
func (p *Pusher) EdgeCount() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return len(p.edges)
}

// Stop cleanly shuts down the pusher.
func (p *Pusher) Stop() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.edges = make(map[string]EdgeSink)
}

// PushResult reports per-edge outcome.
type PushResult struct {
	NodeID string
	Err    error
}

// Push signs payload and delivers it to every registered edge concurrently.
// It returns one PushResult per edge. Errors are per-edge; a partial failure
// does not abort delivery to other edges.
func (p *Pusher) Push(ctx context.Context, payload SyncPayload) []PushResult {
	// Assign a nonce so replayed payloads can be detected.
	if payload.IssuedAt.IsZero() {
		payload.IssuedAt = time.Now().UTC()
	}
	if err := payload.sign(p.cfg.HMACKey); err != nil {
		return []PushResult{{Err: fmt.Errorf("sign payload: %w", err)}}
	}

	p.mu.RLock()
	sinks := make([]EdgeSink, 0, len(p.edges))
	for _, s := range p.edges {
		sinks = append(sinks, s)
	}
	p.mu.RUnlock()

	results := make([]PushResult, len(sinks))
	var wg sync.WaitGroup
	for i, sink := range sinks {
		wg.Add(1)
		go func(idx int, s EdgeSink) {
			defer wg.Done()
			results[idx] = PushResult{
				NodeID: s.NodeID(),
				Err:    p.sendWithRetry(ctx, s, payload),
			}
		}(i, sink)
	}
	wg.Wait()
	return results
}

func (p *Pusher) sendWithRetry(ctx context.Context, sink EdgeSink, payload SyncPayload) error {
	backoff := p.cfg.RetryBase
	var lastErr error
	for attempt := 0; attempt <= p.cfg.MaxRetries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(backoff):
				backoff *= 2
			}
		}
		sendCtx, cancel := context.WithTimeout(ctx, p.cfg.PushTimeout)
		start := time.Now()
		err := sink.Send(sendCtx, payload)
		duration := time.Since(start).Seconds()
		status := "ok"
		if err != nil {
			status = "fail"
		}
		metrics.MeshSyncLatency.WithLabelValues(sink.NodeID(), status).Observe(duration)

		cancel()
		if err == nil {
			return nil
		}
		lastErr = err
	}
	return fmt.Errorf("all %d attempts failed, last: %w", p.cfg.MaxRetries+1, lastErr)
}

// ---------------------------------------------------------------------------
// HTTP edge sink — for edges that expose a POST /sync endpoint
// ---------------------------------------------------------------------------

// HTTPEdgeSink implements EdgeSink for edges reachable over HTTP/HTTPS.
type HTTPEdgeSink struct {
	nodeID string
	url    string
	client *http.Client
	token  string // Bearer token the edge uses to authenticate the hub
}

// NewHTTPEdgeSink creates an EdgeSink that POSTs payloads to url.
// token is sent as a Bearer Authorization header so the edge can authenticate
// the hub independently of the HMAC (defence in depth).
func NewHTTPEdgeSink(nodeID, url, token string, tlsConfig *tls.Config) *HTTPEdgeSink {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	if tlsConfig != nil {
		transport.TLSClientConfig = tlsConfig
	}
	return &HTTPEdgeSink{
		nodeID: nodeID,
		url:    url,
		token:  token,
		client: &http.Client{
			Timeout:   15 * time.Second,
			Transport: transport,
		},
	}
}

func (s *HTTPEdgeSink) NodeID() string { return s.nodeID }

func (s *HTTPEdgeSink) Send(ctx context.Context, p SyncPayload) error {
	body, err := json.Marshal(p)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if s.token != "" {
		req.Header.Set("Authorization", "Bearer "+s.token)
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("edge returned HTTP %d", resp.StatusCode)
	}
	return nil
}

// ---------------------------------------------------------------------------
// Edge-side: apply a received payload
// ---------------------------------------------------------------------------

// EdgeApplier holds the last accepted version and applies incoming payloads.
type EdgeApplier struct {
	mu          sync.Mutex
	hmacKey     []byte
	lastVersion int64
	applyFn     func(ctx context.Context, p SyncPayload) error
}

// NewEdgeApplier creates an EdgeApplier. hmacKey must match the hub's key.
// applyFn is called on every valid, newer payload — put your WireGuard and
// plugin reconciliation logic there.
func NewEdgeApplier(hmacKey []byte, applyFn func(ctx context.Context, p SyncPayload) error) *EdgeApplier {
	return &EdgeApplier{hmacKey: hmacKey, applyFn: applyFn}
}

// Apply verifies the payload and calls applyFn if it is newer than the last
// accepted version. It is safe to call concurrently.
func (a *EdgeApplier) Apply(ctx context.Context, p SyncPayload) error {
	if err := p.Verify(a.hmacKey); err != nil {
		return fmt.Errorf("payload rejected: %w", err)
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if p.Version <= a.lastVersion {
		return nil // already applied or replayed — silently ignore
	}
	if err := a.applyFn(ctx, p); err != nil {
		return fmt.Errorf("apply version %d: %w", p.Version, err)
	}
	a.lastVersion = p.Version
	return nil
}

// ---------------------------------------------------------------------------
// Version helper
// ---------------------------------------------------------------------------

// NewVersion returns a monotonic version number suitable for SyncPayload.Version.
// Uses UnixNano so it is monotonic across restarts without a database counter.
func NewVersion() int64 {
	return time.Now().UnixNano()
}

// NewHMACKey generates a cryptographically random 32-byte HMAC key.
// Call this once at setup and store the result in your secrets store.
func NewHMACKey() ([]byte, error) {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, err
	}
	return key, nil
}
