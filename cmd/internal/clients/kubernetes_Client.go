package clients

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
)

type KubernetesClient struct {
	httpClient *http.Client
	baseURL    string
	token      string
}

func NewKubernetesClient(httpClient *http.Client, baseURL, token string) *KubernetesClient {
	return &KubernetesClient{
		httpClient: httpClient,
		baseURL:    baseURL,
		token:      token,
	}
}

func (c *KubernetesClient) Do(method, path string, body interface{}, out interface{}) error {
	var bodyReader io.Reader
	if body != nil {
		data, _ := json.Marshal(body)
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequest(method, c.baseURL+path, bodyReader)
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.token))

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		errBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("K8s API error (%d): %s", resp.StatusCode, string(errBody))
	}

	if out != nil {
		return json.NewDecoder(resp.Body).Decode(out)
	}

	return nil
}

// DoStream returns the raw response body for K8s streaming (Watch, Logs).
func (c *KubernetesClient) DoStream(method, path string, body interface{}) (io.ReadCloser, error) {
	var bodyReader io.Reader
	if body != nil {
		data, _ := json.Marshal(body)
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequest(method, c.baseURL+path, bodyReader)
	if err != nil {
		return nil, err
	}

	// --- K8s Specific Header ---
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.token))

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode >= 400 {
		resp.Body.Close()
		return nil, fmt.Errorf("K8s API error: %s", resp.Status)
	}

	return resp.Body, nil
}

// HijackPod upgrades an HTTP connection to the Kubernetes WebSocket exec protocol
// (v4.channel.k8s.io) and returns the raw connection for bidirectional streaming.
// It handles both plain HTTP (kubectl proxy) and in-cluster HTTPS.
func (c *KubernetesClient) HijackPod(ctx context.Context, path string, protocol string) (net.Conn, *bufio.Reader, error) {
	u, err := url.Parse(c.baseURL + path)
	if err != nil {
		return nil, nil, err
	}

	// Random WebSocket handshake key
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		return nil, nil, err
	}
	wsKey := base64.StdEncoding.EncodeToString(raw)

	// Resolve host:port
	host := u.Host
	if u.Port() == "" {
		if u.Scheme == "https" {
			host += ":443"
		} else {
			host += ":80"
		}
	}

	// Dial — use TLS for in-cluster HTTPS, plain TCP for kubectl proxy
	var conn net.Conn
	if u.Scheme == "https" {
		var tlsCfg *tls.Config
		if t, ok := c.httpClient.Transport.(*http.Transport); ok {
			tlsCfg = t.TLSClientConfig
		}
		conn, err = tls.DialWithDialer(&net.Dialer{}, "tcp", host, tlsCfg)
	} else {
		conn, err = (&net.Dialer{}).DialContext(ctx, "tcp", host)
	}
	if err != nil {
		return nil, nil, fmt.Errorf("dial %s: %w", host, err)
	}

	// Standard WebSocket upgrade with Kubernetes subprotocol
	reqStr := "GET " + u.RequestURI() + " HTTP/1.1\r\n" +
		"Host: " + u.Host + "\r\n" +
		"Connection: Upgrade\r\n" +
		"Upgrade: websocket\r\n" +
		"Sec-WebSocket-Version: 13\r\n" +
		"Sec-WebSocket-Key: " + wsKey + "\r\n" +
		"Sec-WebSocket-Protocol: " + protocol + "\r\n" +
		"Authorization: Bearer " + c.token + "\r\n\r\n"

	if _, err := conn.Write([]byte(reqStr)); err != nil {
		conn.Close()
		return nil, nil, fmt.Errorf("write upgrade request: %w", err)
	}

	reader := bufio.NewReader(conn)
	resp, err := http.ReadResponse(reader, nil)
	if err != nil {
		conn.Close()
		return nil, nil, fmt.Errorf("read upgrade response: %w", err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusSwitchingProtocols {
		conn.Close()
		return nil, nil, fmt.Errorf("exec upgrade failed (HTTP %d) — check pod is running and container name is correct", resp.StatusCode)
	}

	return conn, reader, nil
}
