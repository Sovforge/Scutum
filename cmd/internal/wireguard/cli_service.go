// internal/wireguard/cli_service.go
package wireguard

import "scutum/cmd/internal/utils"

type CLIService struct{}

func (s *CLIService) AddPeer(iface, publicKey, endpoint, allowedIPs string, keepalive int) error {
	return utils.AddPeer(iface, publicKey, endpoint, allowedIPs, keepalive)
}

func (s *CLIService) UpdatePeerEndpoint(iface, publicKey, endpoint string) error {
	return utils.UpdatePeerEndpoint(iface, publicKey, endpoint)
}

func (s *CLIService) GetStatus(iface string) (string, error) {
	return utils.GetStatus(iface)
}

func (s *CLIService) GetDump(iface string) (string, error) {
	return utils.GetDump(iface)
}
