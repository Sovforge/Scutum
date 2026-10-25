package wireguard

type Service interface {
	AddPeer(iface, publicKey, endpoint, allowedIPs string, keepalive int) error
	// UpdatePeerEndpoint updates only the endpoint for an existing peer without
	// touching allowed-ips or keepalive, so WireGuard's other peer settings are preserved.
	UpdatePeerEndpoint(iface, publicKey, endpoint string) error
	// GetPeerEndpoint returns the endpoint WireGuard currently has recorded for
	// the peer in kernel state. WireGuard updates this automatically when it
	// receives an authenticated packet from a new address (e.g. after a NAT
	// roam via persistent-keepalive), making this more accurate than the DB for
	// FreshEndpoint lookups. Returns ("", nil) if the peer exists but has never
	// connected. Returns an error if the peer is not found.
	GetPeerEndpoint(iface, publicKey string) (string, error)
	GetStatus(iface string) (string, error)
	GetDump(iface string) (string, error)
}
