package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// GitOpsSource is a git repository Scutum watches and applies a manifest
// file from. ManifestType selects how the fetched content is applied:
// "compose" (docker compose up -d) or "kubernetes" (kubectl apply -f -).
// TargetNodeID empty means apply on the hub; set, it's relayed to that node
// over the same HMAC-signed hub-to-node channel other mesh actions use.
type GitOpsSource struct {
	ID                  string `json:"id"`
	Name                string `json:"name"`
	RepoURL             string `json:"repo_url"`
	Branch              string `json:"branch"`
	Path                string `json:"path"`
	ManifestType        string `json:"manifest_type"` // "compose" | "kubernetes"
	TargetNodeID        string `json:"target_node_id"`
	Username            string `json:"username,omitempty"` // paired with a token held in the secrets vault, not here
	PollIntervalSeconds int    `json:"poll_interval_seconds"`
	Enabled             bool   `json:"enabled"`
	LastSyncedAt        string `json:"last_synced_at,omitempty"`
	LastCommitSHA       string `json:"last_commit_sha"`
	LastContentHash     string `json:"last_content_hash"`
	LastStatus          string `json:"last_status"` // "" | "pending" | "synced" | "skipped" | "error"
	LastError           string `json:"last_error,omitempty"`
	CreatedBy           string `json:"created_by"`
	CreatedAt           string `json:"created_at"`
	UpdatedBy           string `json:"updated_by"`
	UpdatedAt           string `json:"updated_at"`
}

// GitOpsSyncEvent is one sync attempt for a source, successful or not.
type GitOpsSyncEvent struct {
	ID          string `json:"id"`
	SourceID    string `json:"source_id"`
	CommitSHA   string `json:"commit_sha"`
	Status      string `json:"status"` // "synced" | "skipped" | "error"
	Message     string `json:"message"`
	TriggeredBy string `json:"triggered_by"`
	StartedAt   string `json:"started_at"`
	FinishedAt  string `json:"finished_at,omitempty"`
}

func (s *Store) CreateGitOpsSource(ctx context.Context, src GitOpsSource) (GitOpsSource, error) {
	q := fmt.Sprintf(`
		INSERT INTO gitops_sources
			(id, name, repo_url, branch, path, manifest_type, target_node_id, username, poll_interval_seconds, enabled, created_by, updated_by)
		VALUES (%s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s)`,
		s.ph(1), s.ph(2), s.ph(3), s.ph(4), s.ph(5), s.ph(6), s.ph(7), s.ph(8), s.ph(9), s.ph(10), s.ph(11), s.ph(12),
	)
	_, err := s.db.ExecContext(ctx, q,
		src.ID, src.Name, src.RepoURL, src.Branch, src.Path, src.ManifestType, src.TargetNodeID, src.Username,
		src.PollIntervalSeconds, boolToInt(src.Enabled), src.CreatedBy, src.CreatedBy,
	)
	if err != nil {
		return GitOpsSource{}, err
	}
	return s.GetGitOpsSource(ctx, src.ID)
}

func gitopsSourceColumns() string {
	return `id, name, repo_url, branch, path, manifest_type, target_node_id, username, poll_interval_seconds, enabled,
		last_synced_at, last_commit_sha, last_content_hash, last_status, last_error,
		created_by, created_at, updated_by, updated_at`
}

func scanGitOpsSource(row interface{ Scan(...any) error }) (GitOpsSource, error) {
	var src GitOpsSource
	var enabled int
	var lastSyncedAt sql.NullString
	err := row.Scan(
		&src.ID, &src.Name, &src.RepoURL, &src.Branch, &src.Path, &src.ManifestType, &src.TargetNodeID, &src.Username,
		&src.PollIntervalSeconds, &enabled, &lastSyncedAt, &src.LastCommitSHA, &src.LastContentHash,
		&src.LastStatus, &src.LastError, &src.CreatedBy, &src.CreatedAt, &src.UpdatedBy, &src.UpdatedAt,
	)
	if err != nil {
		return GitOpsSource{}, err
	}
	src.Enabled = enabled == 1
	src.LastSyncedAt = lastSyncedAt.String
	return src, nil
}

func (s *Store) GetGitOpsSource(ctx context.Context, id string) (GitOpsSource, error) {
	q := fmt.Sprintf(`SELECT %s FROM gitops_sources WHERE id = %s`, gitopsSourceColumns(), s.ph(1))
	src, err := scanGitOpsSource(s.db.QueryRowContext(ctx, q, id))
	if err == sql.ErrNoRows {
		return GitOpsSource{}, fmt.Errorf("gitops source %q not found", id)
	}
	if err != nil {
		return GitOpsSource{}, err
	}
	return src, nil
}

func (s *Store) ListGitOpsSources(ctx context.Context) ([]GitOpsSource, error) {
	q := fmt.Sprintf(`SELECT %s FROM gitops_sources ORDER BY name ASC`, gitopsSourceColumns())
	rows, err := s.db.QueryContext(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []GitOpsSource
	for rows.Next() {
		src, err := scanGitOpsSource(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, src)
	}
	return out, rows.Err()
}

// UpdateGitOpsSourceConfig updates the operator-editable fields only —
// sync-state fields (last_*) are updated separately by RecordGitOpsSync.
func (s *Store) UpdateGitOpsSourceConfig(ctx context.Context, src GitOpsSource) (GitOpsSource, error) {
	q := fmt.Sprintf(`
		UPDATE gitops_sources SET
			name = %s, repo_url = %s, branch = %s, path = %s, manifest_type = %s, target_node_id = %s, username = %s,
			poll_interval_seconds = %s, enabled = %s, updated_by = %s, updated_at = %s
		WHERE id = %s`,
		s.ph(1), s.ph(2), s.ph(3), s.ph(4), s.ph(5), s.ph(6), s.ph(7), s.ph(8), s.ph(9), s.ph(10), s.ph(11), s.ph(12),
	)
	res, err := s.db.ExecContext(ctx, q,
		src.Name, src.RepoURL, src.Branch, src.Path, src.ManifestType, src.TargetNodeID, src.Username,
		src.PollIntervalSeconds, boolToInt(src.Enabled), src.UpdatedBy, time.Now().UTC(), src.ID,
	)
	if err != nil {
		return GitOpsSource{}, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return GitOpsSource{}, fmt.Errorf("gitops source %q not found", src.ID)
	}
	return s.GetGitOpsSource(ctx, src.ID)
}

func (s *Store) DeleteGitOpsSource(ctx context.Context, id string) error {
	q := fmt.Sprintf(`DELETE FROM gitops_sources WHERE id = %s`, s.ph(1))
	res, err := s.db.ExecContext(ctx, q, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("gitops source %q not found", id)
	}
	return nil
}

// RecordGitOpsSync inserts a sync history row and updates the source's
// last-sync-state fields in one call — every real sync attempt (whether it
// changed anything, skipped as unchanged, or failed) goes through this.
func (s *Store) RecordGitOpsSync(ctx context.Context, ev GitOpsSyncEvent, commitSHA, contentHash string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	insQ := fmt.Sprintf(`
		INSERT INTO gitops_sync_events (id, source_id, commit_sha, status, message, triggered_by, finished_at)
		VALUES (%s, %s, %s, %s, %s, %s, %s)`,
		s.ph(1), s.ph(2), s.ph(3), s.ph(4), s.ph(5), s.ph(6), s.ph(7),
	)
	if _, err := tx.ExecContext(ctx, insQ, ev.ID, ev.SourceID, ev.CommitSHA, ev.Status, ev.Message, ev.TriggeredBy, time.Now().UTC()); err != nil {
		return err
	}

	updQ := fmt.Sprintf(`
		UPDATE gitops_sources SET last_synced_at = %s, last_commit_sha = %s, last_content_hash = %s, last_status = %s, last_error = %s
		WHERE id = %s`,
		s.ph(1), s.ph(2), s.ph(3), s.ph(4), s.ph(5), s.ph(6),
	)
	lastErr := ""
	if ev.Status == "error" {
		lastErr = ev.Message
	}
	if _, err := tx.ExecContext(ctx, updQ, time.Now().UTC(), commitSHA, contentHash, ev.Status, lastErr, ev.SourceID); err != nil {
		return err
	}

	return tx.Commit()
}

func (s *Store) ListGitOpsSyncEvents(ctx context.Context, sourceID string, limit int) ([]GitOpsSyncEvent, error) {
	q := fmt.Sprintf(`
		SELECT id, source_id, commit_sha, status, message, triggered_by, started_at, finished_at
		FROM gitops_sync_events WHERE source_id = %s ORDER BY started_at DESC LIMIT %s`,
		s.ph(1), s.ph(2),
	)
	rows, err := s.db.QueryContext(ctx, q, sourceID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []GitOpsSyncEvent
	for rows.Next() {
		var ev GitOpsSyncEvent
		var finishedAt sql.NullString
		if err := rows.Scan(&ev.ID, &ev.SourceID, &ev.CommitSHA, &ev.Status, &ev.Message, &ev.TriggeredBy, &ev.StartedAt, &finishedAt); err != nil {
			return nil, err
		}
		ev.FinishedAt = finishedAt.String
		out = append(out, ev)
	}
	return out, rows.Err()
}
