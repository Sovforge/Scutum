package webhooks

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"scutum/cmd/internal/store"
)

const (
	EventNodeEnrolled  = "node.enrolled"
	EventNodeOffline   = "node.offline"
	EventNodeOnline    = "node.online"
	EventHealerRestart = "healer.service_restart"
	EventAuditCritical = "audit.critical"
	EventUserCreated   = "user.created"
	EventSSOLogin      = "auth.sso_login"
)

type Event struct {
	Type      string         `json:"type"`
	Timestamp time.Time      `json:"timestamp"`
	Payload   map[string]any `json:"payload"`
}

type dispatchStore interface {
	ListEnabledWebhooksForEvent(ctx context.Context, event string) ([]store.WebhookConfig, error)
}

type Dispatcher struct {
	store  dispatchStore
	ch     chan Event
	cancel context.CancelFunc
	done   chan struct{}
}

func NewDispatcher(s dispatchStore) *Dispatcher {
	return &Dispatcher{store: s, ch: make(chan Event, 256), done: make(chan struct{})}
}

func (d *Dispatcher) Send(e Event) {
	if e.Timestamp.IsZero() {
		e.Timestamp = time.Now().UTC()
	}
	select {
	case d.ch <- e:
	default:
	}
}

func (d *Dispatcher) Start(ctx context.Context) {
	ctx, d.cancel = context.WithCancel(ctx)
	go func() {
		defer close(d.done)
		client := &http.Client{Timeout: 5 * time.Second}
		for {
			select {
			case <-ctx.Done():
				return
			case e := <-d.ch:
				d.dispatch(ctx, client, e)
			}
		}
	}()
}

func (d *Dispatcher) Stop() {
	if d.cancel != nil {
		d.cancel()
	}
	<-d.done
}

func (d *Dispatcher) dispatch(ctx context.Context, client *http.Client, e Event) {
	hooks, err := d.store.ListEnabledWebhooksForEvent(ctx, e.Type)
	if err != nil || len(hooks) == 0 {
		return
	}
	body, err := json.Marshal(e)
	if err != nil {
		return
	}
	for _, h := range hooks {
		deliverWithRetry(client, h, e.Type, body)
	}
}

func deliverWithRetry(client *http.Client, h store.WebhookConfig, eventType string, body []byte) {
	for attempt := range 2 {
		if err := deliver(client, h, eventType, body); err == nil {
			return
		} else if attempt == 0 {
			time.Sleep(1 * time.Second)
		}
	}
}

func deliver(client *http.Client, h store.WebhookConfig, eventType string, body []byte) error {
	req, err := http.NewRequest(http.MethodPost, h.URL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Scutum-Event", eventType)
	req.Header.Set("X-Scutum-Timestamp", fmt.Sprintf("%d", time.Now().Unix()))
	if h.Secret != "" {
		mac := hmac.New(sha256.New, []byte(h.Secret))
		mac.Write(body)
		req.Header.Set("X-Scutum-Signature", "sha256="+hex.EncodeToString(mac.Sum(nil)))
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("webhook returned %d", resp.StatusCode)
	}
	return nil
}
