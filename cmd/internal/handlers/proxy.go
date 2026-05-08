package handlers

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"scutum/cmd/internal/store"
)

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

	target := "http://" + node.Address + path
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

	client := &http.Client{Timeout: 3 * time.Minute}
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

// hubRequestSig computes HMAC-SHA256 over "ts\nmethod\npath\n".
// Body is intentionally excluded so the middleware can verify without consuming r.Body.
func hubRequestSig(key []byte, ts, method, path string, _ []byte) string {
	mac := hmac.New(sha256.New, key)
	fmt.Fprintf(mac, "%s\n%s\n%s\n", ts, method, path)
	return hex.EncodeToString(mac.Sum(nil))
}
