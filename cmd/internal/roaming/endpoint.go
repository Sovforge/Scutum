package roaming

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// HubMeshIP extracts the host address from a WireGuard AllowedIPs value when
// it is a single-host route (/32 for IPv4 or /128 for IPv6). Returns "" for
// aggregate routes like "0.0.0.0/0" where no specific mesh IP can be inferred.
//
// This is used during setup to auto-derive the hub's API address from the
// WireGuard config that the operator already provided, so a separate
// "hub_api_address" field is not required. All API traffic then flows through
// the WireGuard tunnel and WireGuard's own persistent-keepalive handles NAT
// roaming without any application-layer re-registration loop.
func HubMeshIP(allowedIPs string) string {
	// Use the first entry when multiple CIDRs are comma-separated.
	entry := allowedIPs
	if idx := strings.IndexByte(allowedIPs, ','); idx != -1 {
		entry = strings.TrimSpace(allowedIPs[:idx])
	}
	slash := strings.IndexByte(entry, '/')
	if slash == -1 {
		return entry // bare IP with no CIDR
	}
	mask := entry[slash+1:]
	if mask != "32" && mask != "128" {
		return "" // aggregate route — cannot infer a single mesh IP
	}
	return entry[:slash]
}

// NodeAPIBase converts a node's stored address (which may be a WireGuard CIDR
// like "10.x.x.x/24" or a proper "host:port") into an "https://host:port" base
// URL. Returns "" if addr is empty.
func NodeAPIBase(addr string) string {
	if addr == "" {
		return ""
	}
	// Strip CIDR suffix (e.g. "10.0.0.2/24" → "10.0.0.2")
	if idx := strings.Index(addr, "/"); idx != -1 && !strings.Contains(addr[:idx], ":") {
		addr = addr[:idx]
	}
	if !strings.Contains(addr, ":") {
		addr = addr + ":8080"
	}
	return "https://" + addr
}

// CallRegisterEndpoint sends a signed POST to the hub's
// /api/network/register-endpoint. It uses the same HMAC signing format as
// hub-proxied requests so the hub's existing auth middleware accepts it without
// additional setup.
func CallRegisterEndpoint(ctx context.Context, hubAPIBase, pubKey string, listenPort int, hmacKey []byte, tlsConfig *tls.Config) error {
	body, _ := json.Marshal(map[string]interface{}{
		"public_key":  pubKey,
		"listen_port": listenPort,
	})

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		hubAPIBase+"/api/network/register-endpoint", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}

	ts := strconv.FormatInt(time.Now().Unix(), 10)
	mac := hmac.New(sha256.New, hmacKey)
	fmt.Fprintf(mac, "%s\n%s\n%s\n", ts, req.Method, "/api/network/register-endpoint")
	sig := hex.EncodeToString(mac.Sum(nil))

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Scutum-Hub-Sig", sig)
	req.Header.Set("X-Scutum-Hub-Ts", ts)

	resp, err := (&http.Client{
		Timeout:   10 * time.Second,
		Transport: &http.Transport{TLSClientConfig: tlsConfig},
	}).Do(req)
	if err != nil {
		return fmt.Errorf("http: %w", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("hub returned HTTP %d", resp.StatusCode)
	}
	return nil
}
