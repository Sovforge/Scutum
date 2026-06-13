package license

import (
	"context"
	"log/slog"
	"os"
	"sync/atomic"
	"time"
)

// Watcher polls a license file for mtime changes and hot-reloads when it sees one.
// The active claims pointer is only ever replaced with a freshly verified license,
// so an invalid or expired replacement file does not evict the current valid license.
type Watcher struct {
	path   string
	mtime  time.Time
	active atomic.Pointer[Claims]
}

// NewWatcher creates a Watcher pre-loaded with an already-verified Claims value.
func NewWatcher(path string, initial Claims) *Watcher {
	w := &Watcher{path: path}
	w.active.Store(&initial)
	if info, err := os.Stat(path); err == nil {
		w.mtime = info.ModTime()
	}
	return w
}

// Active returns the current valid license claims, or nil if none is loaded.
func (w *Watcher) Active() *Claims {
	return w.active.Load()
}

// Run blocks until ctx is cancelled, polling the license file every 60 seconds.
// Start this in a goroutine after creating the watcher.
func (w *Watcher) Run(ctx context.Context) {
	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.reload()
		}
	}
}

func (w *Watcher) reload() {
	info, err := os.Stat(w.path)
	if err != nil || !info.ModTime().After(w.mtime) {
		return
	}
	data, err := os.ReadFile(w.path)
	if err != nil {
		slog.Warn("license: reload failed", "error", err)
		return
	}
	claims, err := Verify(string(data))
	if err != nil {
		slog.Warn("license: new file is invalid, keeping current license", "error", err)
		return
	}
	w.mtime = info.ModTime()
	w.active.Store(&claims)
	slog.Info("license: reloaded",
		"licensee", claims.Licensee,
		"expires", claims.ExpiresAt.Format(time.DateOnly),
	)
}
