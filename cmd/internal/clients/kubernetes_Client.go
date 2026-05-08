package clients

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
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

// HijackPod is a custom method to handle WebSocket-like connections for exec/attach. It performs the HTTP upgrade and returns the raw connection.
func (c *KubernetesClient) HijackPod(ctx context.Context, path string, protocol string) (net.Conn, *bufio.Reader, error) {
	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+path, nil)
	if err != nil {
		return nil, nil, err
	}

	// Use the protocol argument here
	req.Header.Set("Connection", "Upgrade")
	req.Header.Set("Upgrade", protocol)
	req.Header.Set("Authorization", "Bearer "+c.token)

	// Manual Dialing logic...
	host := req.URL.Host
	conn, err := (&net.Dialer{}).DialContext(ctx, "tcp", host)
	if err != nil {
		return nil, nil, err
	}

	err = req.Write(conn)
	if err != nil {
		conn.Close()
		return nil, nil, err
	}

	reader := bufio.NewReader(conn)
	resp, err := http.ReadResponse(reader, req)
	if err != nil || resp.StatusCode != http.StatusSwitchingProtocols {
		conn.Close()
		return nil, nil, fmt.Errorf("upgrade failed: %v", err)
	}

	return conn, reader, nil
}
