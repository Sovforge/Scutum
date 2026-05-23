package tests

import (
	"context"
	"testing"
	"time"
	"scutum/cmd/internal/sync"
)

type mockWGChecker struct {
	age    time.Duration
	err    error
	reAdds int
}

func (m *mockWGChecker) PeerHandshakeAge(_ context.Context, iface, pubkey string) (time.Duration, error) {
	return m.age, m.err
}

func (m *mockWGChecker) ReAddPeer(_ context.Context, peer sync.WGPeer) error {
	m.reAdds++
	return nil
}

func TestHealerPeers(t *testing.T) {
	cfg := sync.HealerConfig{
		Interval:        100 * time.Millisecond,
		HandshakeMaxAge: 1 * time.Minute,
	}

	t.Run("fresh peer no re-add", func(t *testing.T) {
		checker := &mockWGChecker{age: 10 * time.Second}
		h := sync.NewHealer(cfg, checker)
		h.AddPeer(sync.WGPeer{IfaceName: "wg0", PublicKey: "abc="})
		h.Start(context.Background())
		time.Sleep(150 * time.Millisecond)
		h.Stop()
		if checker.reAdds != 0 {
			t.Errorf("expected 0 re-adds for fresh peer, got %d", checker.reAdds)
		}
	})

	// Stale peers are logged but NOT re-added; WireGuard's own keepalive/rekey
	// recovers the tunnel without endpoint interference.
	t.Run("stale peer no re-add", func(t *testing.T) {
		checker := &mockWGChecker{age: 5 * time.Minute}
		h := sync.NewHealer(cfg, checker)
		h.AddPeer(sync.WGPeer{IfaceName: "wg0", PublicKey: "abc="})
		h.Start(context.Background())
		time.Sleep(150 * time.Millisecond)
		h.Stop()
		if checker.reAdds != 0 {
			t.Errorf("expected 0 re-adds for stale peer (WG heals itself), got %d", checker.reAdds)
		}
	})

	// Missing peers (PeerHandshakeAge returns an error) must be re-added.
	t.Run("missing peer triggers re-add", func(t *testing.T) {
		checker := &mockWGChecker{err: context.DeadlineExceeded}
		h := sync.NewHealer(cfg, checker)
		h.AddPeer(sync.WGPeer{IfaceName: "wg0", PublicKey: "abc="})
		h.Start(context.Background())
		time.Sleep(150 * time.Millisecond)
		h.Stop()
		if checker.reAdds == 0 {
			t.Error("expected at least one re-add for missing peer")
		}
	})
}

func TestHealerServices(t *testing.T) {
	restarts := 0
	h := sync.NewHealer(sync.HealerConfig{Interval: 50 * time.Millisecond}, nil)
	h.AddService(sync.ServiceEntry{
		Name: "test-svc",
		Check: func(ctx context.Context) error {
			return context.DeadlineExceeded // unhealthy
		},
		Restart: func(ctx context.Context) error {
			restarts++
			return nil
		},
	})

	h.Start(context.Background())
	time.Sleep(150 * time.Millisecond)
	h.Stop()

	if restarts == 0 {
		t.Error("expected service restart")
	}
}
