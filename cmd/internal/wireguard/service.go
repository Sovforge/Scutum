package wireguard

type Service interface {
	AddPeer(iface, publicKey, endpoint, allowedIPs string, keepalive int) error
	GetStatus(iface string) (string, error)
	GetDump(iface string) (string, error)
}
