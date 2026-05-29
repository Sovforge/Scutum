package store

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

type WebhookConfig struct {
	ID        string
	Name      string
	URL       string
	Secret    string
	Events    []string
	Enabled   bool
	CreatedAt time.Time
}

func (s *Store) CreateWebhook(ctx context.Context, id, name, url, secret string, events []string) error {
	evJSON, _ := json.Marshal(events)
	_, err := s.db.ExecContext(ctx,
		fmt.Sprintf(`INSERT INTO webhook_configs (id,name,url,secret,events,enabled) VALUES (%s,%s,%s,%s,%s,1)`,
			s.ph(1), s.ph(2), s.ph(3), s.ph(4), s.ph(5)),
		id, name, url, secret, string(evJSON))
	return err
}

func (s *Store) ListWebhooks(ctx context.Context) ([]WebhookConfig, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id,name,url,secret,events,enabled,created_at FROM webhook_configs ORDER BY created_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []WebhookConfig
	for rows.Next() {
		var w WebhookConfig
		var evJSON string
		var enabled int
		if err := rows.Scan(&w.ID, &w.Name, &w.URL, &w.Secret, &evJSON, &enabled, &w.CreatedAt); err != nil {
			return nil, err
		}
		_ = json.Unmarshal([]byte(evJSON), &w.Events)
		w.Enabled = enabled != 0
		out = append(out, w)
	}
	return out, rows.Err()
}

func (s *Store) GetWebhook(ctx context.Context, id string) (WebhookConfig, error) {
	var w WebhookConfig
	var evJSON string
	var enabled int
	err := s.db.QueryRowContext(ctx,
		fmt.Sprintf(`SELECT id,name,url,secret,events,enabled,created_at FROM webhook_configs WHERE id=%s`, s.ph(1)), id).
		Scan(&w.ID, &w.Name, &w.URL, &w.Secret, &evJSON, &enabled, &w.CreatedAt)
	if err != nil {
		return WebhookConfig{}, err
	}
	_ = json.Unmarshal([]byte(evJSON), &w.Events)
	w.Enabled = enabled != 0
	return w, nil
}

func (s *Store) UpdateWebhook(ctx context.Context, id, name, url, secret string, events []string, enabled bool) error {
	evJSON, _ := json.Marshal(events)
	en := 0
	if enabled {
		en = 1
	}
	_, err := s.db.ExecContext(ctx,
		fmt.Sprintf(`UPDATE webhook_configs SET name=%s,url=%s,secret=%s,events=%s,enabled=%s WHERE id=%s`,
			s.ph(1), s.ph(2), s.ph(3), s.ph(4), s.ph(5), s.ph(6)),
		name, url, secret, string(evJSON), en, id)
	return err
}

func (s *Store) DeleteWebhook(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx,
		fmt.Sprintf(`DELETE FROM webhook_configs WHERE id=%s`, s.ph(1)), id)
	return err
}

func (s *Store) ListEnabledWebhooksForEvent(ctx context.Context, event string) ([]WebhookConfig, error) {
	all, err := s.ListWebhooks(ctx)
	if err != nil {
		return nil, err
	}
	var out []WebhookConfig
	for _, w := range all {
		if !w.Enabled {
			continue
		}
		for _, e := range w.Events {
			if e == event || e == "*" {
				out = append(out, w)
				break
			}
		}
	}
	return out, nil
}
