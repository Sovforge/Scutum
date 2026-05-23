package wireguard

type Service interface {
	AddPeer(iface, publicKey, endpoint, allowedIPs string, keepalive int) error
	// UpdatePeerEndpoint updates only the endpoint for an existing peer without
	// touching allowed-ips or keepalive, so WireGuard's other peer settings are preserved.
	UpdatePeerEndpoint(iface, publicKey, endpoint string) error
	GetStatus(iface string) (string, error)
	GetDump(iface string) (string, error)
}
