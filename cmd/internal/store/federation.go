package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

type FederationPeer struct {
	ID           string     `json:"id"`
	Name         string     `json:"name"`
	HubURL       string     `json:"hub_url"`
	WGEndpoint   string     `json:"wg_endpoint"`
	WGPublicKey  string     `json:"wg_public_key"`
	MeshCIDR     string     `json:"mesh_cidr"`
	AllowedIPs   string     `json:"allowed_ips"`
	Status       string     `json:"status"`
	LastSeen     *time.Time `json:"last_seen,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
}

func (s *Store) CreateFederationPeer(ctx context.Context, id, name, hubURL, wgEndpoint, wgPublicKey, meshCIDR, allowedIPs string) error {
	_, err := s.db.ExecContext(ctx,
		fmt.Sprintf(`INSERT INTO federation_peers (id,name,hub_url,wg_endpoint,wg_public_key,mesh_cidr,allowed_ips,status)
		             VALUES (%s,%s,%s,%s,%s,%s,%s,'pending')`,
			s.ph(1), s.ph(2), s.ph(3), s.ph(4), s.ph(5), s.ph(6), s.ph(7)),
		id, name, hubURL, wgEndpoint, wgPublicKey, meshCIDR, allowedIPs)
	return err
}

func (s *Store) ListFederationPeers(ctx context.Context) ([]FederationPeer, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id,name,hub_url,wg_endpoint,wg_public_key,mesh_cidr,allowed_ips,status,last_seen,created_at
		 FROM federation_peers ORDER BY created_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []FederationPeer
	for rows.Next() {
		var p FederationPeer
		if err := rows.Scan(&p.ID, &p.Name, &p.HubURL, &p.WGEndpoint, &p.WGPublicKey,
			&p.MeshCIDR, &p.AllowedIPs, &p.Status, &p.LastSeen, &p.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (s *Store) GetFederationPeer(ctx context.Context, id string) (FederationPeer, error) {
	var p FederationPeer
	err := s.db.QueryRowContext(ctx,
		fmt.Sprintf(`SELECT id,name,hub_url,wg_endpoint,wg_public_key,mesh_cidr,allowed_ips,status,last_seen,created_at
		             FROM federation_peers WHERE id=%s`, s.ph(1)), id).
		Scan(&p.ID, &p.Name, &p.HubURL, &p.WGEndpoint, &p.WGPublicKey,
			&p.MeshCIDR, &p.AllowedIPs, &p.Status, &p.LastSeen, &p.CreatedAt)
	if err != nil {
		return FederationPeer{}, err
	}
	return p, nil
}

func (s *Store) UpdateFederationPeerStatus(ctx context.Context, id, status string) error {
	now := time.Now()
	var err error
	if status == "connected" {
		_, err = s.db.ExecContext(ctx,
			fmt.Sprintf(`UPDATE federation_peers SET status=%s,last_seen=%s WHERE id=%s`,
				s.ph(1), s.ph(2), s.ph(3)),
			status, now, id)
	} else {
		_, err = s.db.ExecContext(ctx,
			fmt.Sprintf(`UPDATE federation_peers SET status=%s WHERE id=%s`, s.ph(1), s.ph(2)),
			status, id)
	}
	return err
}

func (s *Store) DeleteFederationPeer(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx,
		fmt.Sprintf(`DELETE FROM federation_peers WHERE id=%s`, s.ph(1)), id)
	return err
}

func (s *Store) GetFederationPeerByPublicKey(ctx context.Context, pubKey string) (FederationPeer, error) {
	var p FederationPeer
	err := s.db.QueryRowContext(ctx,
		fmt.Sprintf(`SELECT id,name,hub_url,wg_endpoint,wg_public_key,mesh_cidr,allowed_ips,status,last_seen,created_at
		             FROM federation_peers WHERE wg_public_key=%s`, s.ph(1)), pubKey).
		Scan(&p.ID, &p.Name, &p.HubURL, &p.WGEndpoint, &p.WGPublicKey,
			&p.MeshCIDR, &p.AllowedIPs, &p.Status, &p.LastSeen, &p.CreatedAt)
	if err == sql.ErrNoRows {
		return FederationPeer{}, sql.ErrNoRows
	}
	return p, err
}
