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

func TestHealerDNSEndpoints(t *testing.T) {
	cfg := sync.HealerConfig{
		Interval:        50 * time.Millisecond,
		HandshakeMaxAge: 1 * time.Second,
	}

	t.Run("stale DNS peer rate limits re-adds", func(t *testing.T) {
		checker := &mockWGChecker{age: 5 * time.Minute}
		h := sync.NewHealer(cfg, checker)
		h.AddPeer(sync.WGPeer{
			IfaceName: "wg0",
			PublicKey: "dns=",
			Endpoint:  "hub.example.com:51820",
			FreshEndpoint: func(ctx context.Context) (string, error) {
				return "hub.example.com:51820", nil
			},
		})
		h.Start(context.Background())
		time.Sleep(180 * time.Millisecond)
		h.Stop()
		// Should re-add exactly once (the initial run where lastEP is empty),
		// then rate-limit subsequent stale check rounds.
		if checker.reAdds != 1 {
			t.Errorf("expected exactly 1 re-add for stale DNS peer (due to 5m rate limit), got %d", checker.reAdds)
		}
	})

	t.Run("stale DNS peer re-adds if endpoint changes", func(t *testing.T) {
		checker := &mockWGChecker{age: 5 * time.Minute}
		h := sync.NewHealer(cfg, checker)
		counter := 0
		h.AddPeer(sync.WGPeer{
			IfaceName: "wg0",
			PublicKey: "dns2=",
			Endpoint:  "hub1.example.com:51820",
			FreshEndpoint: func(ctx context.Context) (string, error) {
				counter++
				if counter%2 == 1 {
					return "hub1.example.com:51820", nil
				}
				return "hub2.example.com:51820", nil
			},
		})
		h.Start(context.Background())
		time.Sleep(180 * time.Millisecond)
		h.Stop()
		// Initial run (1) -> "hub1" (re-adds)
		// Second run (2) -> "hub2" (changed, re-adds)
		// Third run (3) -> "hub1" (changed, re-adds)
		// Should be at least 2 or 3 re-adds.
		if checker.reAdds < 2 {
			t.Errorf("expected at least 2 re-adds when DNS endpoint changes, got %d", checker.reAdds)
		}
	})
}
