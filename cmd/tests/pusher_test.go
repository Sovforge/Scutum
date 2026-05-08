package tests

import (
	"context"
	"testing"
	"scutum/cmd/internal/sync"
)

type mockEdgeSink struct {
	id     string
	sent   []sync.SyncPayload
	err    error
	called int
}

func (m *mockEdgeSink) NodeID() string { return m.id }
func (m *mockEdgeSink) Send(ctx context.Context, p sync.SyncPayload) error {
	m.called++
	m.sent = append(m.sent, p)
	return m.err
}

func TestPusher(t *testing.T) {
	key := []byte("secret")
	p := sync.NewPusher(sync.PushConfig{HMACKey: key})
	
	sink := &mockEdgeSink{id: "edge-1"}
	p.Register(sink)

	payload := sync.SyncPayload{Version: 1}
	results := p.Push(context.Background(), payload)

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Err != nil {
		t.Fatalf("push failed: %v", results[0].Err)
	}
	if sink.called != 1 {
		t.Errorf("expected 1 call, got %d", sink.called)
	}

	// Verify signature
	if err := sink.sent[0].Verify(key); err != nil {
		t.Errorf("signature verification failed: %v", err)
	}
}

func TestEdgeApplier(t *testing.T) {
	key := []byte("secret")
	applied := 0
	applier := sync.NewEdgeApplier(key, func(ctx context.Context, p sync.SyncPayload) error {
		applied++
		return nil
	})

	// Use a pusher to sign the payload
	pusher := sync.NewPusher(sync.PushConfig{HMACKey: key})
	sink := &mockEdgeSink{id: "temp"}
	pusher.Register(sink)
	pusher.Push(context.Background(), sync.SyncPayload{Version: 100})
	signedPayload := sink.sent[0]

	// Test apply
	if err := applier.Apply(context.Background(), signedPayload); err != nil {
		t.Fatalf("Apply failed: %v", err)
	}
	if applied != 1 {
		t.Error("expected applyFn to be called")
	}

	// Test replay (same version)
	if err := applier.Apply(context.Background(), signedPayload); err != nil {
		t.Fatalf("Apply replay failed: %v", err)
	}
	if applied != 1 {
		t.Error("expected applyFn NOT to be called for replay")
	}
}


// I'll add a helper to pusher_test.go to sign payloads if I can't access unexported fields.
// Actually, I can't access unexported fields in package tests.
// I'll use sync.NewPusher to sign it.
