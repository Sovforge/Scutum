package handlers

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"scutum/cmd/internal/store"
	"scutum/cmd/internal/utils"
)

var proxyTLSConfig *tls.Config

// SetProxyTLSConfig sets the TLS configuration used for outbound proxy requests
// to peer nodes. Call once at startup before any requests are served.
func SetProxyTLSConfig(cfg *tls.Config) { proxyTLSConfig = cfg }

type nodeProxyStore interface {
	GetNode(ctx context.Context, id string) (store.NodeRecord, error)
}

// proxyRequest proxies the current request to the node named in X-Target-Node,
// deriving the target path from r.URL (path + query). Returns true if proxied.
func proxyRequest(w http.ResponseWriter, r *http.Request, body []byte, ns nodeProxyStore) bool {
	return proxyToNode(w, r, body, ns, "/api"+r.URL.RequestURI())
}

// proxyToNode forwards the request body to the same path on the target node
// when the X-Target-Node request header is set. Returns true if the request
// was proxied (caller should return immediately); false if it should be handled
// locally.
func proxyToNode(w http.ResponseWriter, r *http.Request, body []byte, ns nodeProxyStore, path string) bool {
	nodeID := r.Header.Get("X-Target-Node")
	if nodeID == "" {
		return false
	}

	node, err := ns.GetNode(r.Context(), nodeID)
	if err != nil {
		http.Error(w, "target node not found", http.StatusBadRequest)
		return true
	}

	host := normaliseAddress(node.Address)
	target := "https://" + host + path
	req, err := http.NewRequestWithContext(r.Context(), r.Method, target, bytes.NewReader(body))
	if err != nil {
		http.Error(w, "proxy setup: "+err.Error(), http.StatusInternalServerError)
		return true
	}

	for k, vs := range r.Header {
		// Drop the browser's auth and target-node routing header.
		if strings.EqualFold(k, "X-Target-Node") || strings.EqualFold(k, "Authorization") {
			continue
		}
		for _, v := range vs {
			req.Header.Add(k, v)
		}
	}

	if len(proxyHMACKey) > 0 {
		ts := strconv.FormatInt(time.Now().Unix(), 10)
		sig := hubRequestSig(proxyHMACKey, ts, r.Method, path, body)
		req.Header.Set("X-Scutum-Hub-Ts", ts)
		req.Header.Set("X-Scutum-Hub-Sig", sig)
	}

	client := &http.Client{
		Timeout:   3 * time.Minute,
		Transport: &http.Transport{TLSClientConfig: proxyTLSConfig},
	}
	resp, err := client.Do(req)
	if err != nil {
		http.Error(w, "proxy error: "+err.Error(), http.StatusBadGateway)
		return true
	}
	defer resp.Body.Close()

	for k, vs := range resp.Header {
		for _, v := range vs {
			w.Header().Add(k, v)
		}
	}
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body) //nolint
	return true
}

// normaliseAddress ensures addr is in host:port form.
// It strips any scheme prefix, trailing slashes, and replaces the first "/" after
// the host with ":" so that addresses entered as "10.0.0.1/8080" still work.
func normaliseAddress(addr string) string {
	// Strip scheme if someone stored "http://host:port"
	addr = strings.TrimPrefix(addr, "https://")
	addr = strings.TrimPrefix(addr, "http://")
	addr = strings.TrimRight(addr, "/")

	// If the address contains no ":" but contains "/" treat the slash as the port separator.
	// e.g. "10.144.26.2/8080" → "10.144.26.2:8080"
	if !strings.Contains(addr, ":") {
		if idx := strings.Index(addr, "/"); idx != -1 {
			addr = addr[:idx] + ":" + addr[idx+1:]
		}
	}

	// Validate with url.Parse; fall back to original if it looks fine.
	if _, err := url.Parse("http://" + addr); err != nil {
		return addr
	}
	return addr
}

// proxyWSToNode proxies a WebSocket terminal connection to a remote node identified
// by the "nodeId" query parameter. The browser WebSocket is accepted first, then the
// hub opens a WebSocket to the remote node (authenticated via HMAC) and relays raw
// frames bidirectionally. Returns true if the request was handled (caller must return).
func proxyWSToNode(w http.ResponseWriter, r *http.Request, ns nodeProxyStore, remotePath string) bool {
	nodeID := r.URL.Query().Get("nodeId")
	if nodeID == "" {
		return false
	}

	node, err := ns.GetNode(r.Context(), nodeID)
	if err != nil {
		http.Error(w, "target node not found", http.StatusBadRequest)
		return true
	}

	host := normaliseAddress(node.Address)

	// Upgrade the browser connection first so errors can be sent over the channel.
	browserConn, err := utils.UpgradeToWebSocket(w, r)
	if err != nil {
		return true
	}
	defer browserConn.Close()

	// Dial the remote node.
	var remoteConn net.Conn
	dialTimeout := 10 * time.Second
	if proxyTLSConfig != nil {
		remoteConn, err = tls.DialWithDialer(&net.Dialer{Timeout: dialTimeout}, "tcp", host, proxyTLSConfig)
	} else {
		remoteConn, err = net.DialTimeout("tcp", host, dialTimeout)
	}
	if err != nil {
		utils.WriteWSFrame(browserConn, []byte("Error: cannot reach node: "+err.Error())) //nolint
		return true
	}
	defer remoteConn.Close()

	// Generate Sec-WebSocket-Key for the outbound upgrade request.
	raw := make([]byte, 16)
	rand.Read(raw) //nolint:errcheck
	wsKey := base64.StdEncoding.EncodeToString(raw)

	// Build HMAC auth headers so the remote node accepts us as a hub request.
	var authHeaders string
	if len(proxyHMACKey) > 0 {
		ts := strconv.FormatInt(time.Now().Unix(), 10)
		sig := hubRequestSig(proxyHMACKey, ts, "GET", remotePath, nil)
		authHeaders = "X-Scutum-Hub-Ts: " + ts + "\r\nX-Scutum-Hub-Sig: " + sig + "\r\n"
	}

	upgradeReq := "GET " + remotePath + " HTTP/1.1\r\n" +
		"Host: " + host + "\r\n" +
		"Upgrade: websocket\r\n" +
		"Connection: Upgrade\r\n" +
		"Sec-WebSocket-Key: " + wsKey + "\r\n" +
		"Sec-WebSocket-Version: 13\r\n" +
		authHeaders +
		"\r\n"

	if _, err := remoteConn.Write([]byte(upgradeReq)); err != nil {
		utils.WriteWSFrame(browserConn, []byte("Error: upgrade request failed: "+err.Error())) //nolint
		return true
	}

	// Read the HTTP response headers (ends at \r\n\r\n).
	var respBuf []byte
	b := [1]byte{}
	for len(respBuf) < 4096 {
		if _, err := remoteConn.Read(b[:]); err != nil {
			utils.WriteWSFrame(browserConn, []byte("Error: upgrade response: "+err.Error())) //nolint
			return true
		}
		respBuf = append(respBuf, b[0])
		if len(respBuf) >= 4 && string(respBuf[len(respBuf)-4:]) == "\r\n\r\n" {
			break
		}
	}
	if !strings.Contains(string(respBuf), "101") {
		utils.WriteWSFrame(browserConn, []byte("Error: node rejected WebSocket upgrade")) //nolint
		return true
	}

	// Relay raw WebSocket frames bidirectionally:
	//   browser → hub  (masked client frames)   → remote node
	//   remote node    (unmasked server frames)  → browser
	done := make(chan struct{}, 2)
	go func() { io.Copy(remoteConn, browserConn); done <- struct{}{} }() //nolint
	go func() { io.Copy(browserConn, remoteConn); done <- struct{}{} }() //nolint
	<-done
	return true
}

// hubRequestSig computes HMAC-SHA256 over "ts\nmethod\npath\n".
// Body is intentionally excluded so the middleware can verify without consuming r.Body.
func hubRequestSig(key []byte, ts, method, path string, _ []byte) string {
	mac := hmac.New(sha256.New, key)
	fmt.Fprintf(mac, "%s\n%s\n%s\n", ts, method, path)
	return hex.EncodeToString(mac.Sum(nil))
}
