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

// DialFunc opens a raw connection to the Docker daemon.
// On Linux this dials the Unix socket; on Windows it dials TCP.
type DialFunc func(ctx context.Context) (net.Conn, error)

type DockerClient struct {
	httpClient *http.Client
	baseurl    string
	dial       DialFunc
}

// NewDockerClient creates a new DockerClient with the given HTTP client, base URL,
// and a dial function used by Hijack to open raw connections to the daemon.
func NewDockerClient(httpClient *http.Client, baseurl string, dial DialFunc) *DockerClient {
	return &DockerClient{
		httpClient: httpClient,
		baseurl:    baseurl,
		dial:       dial,
	}
}

// Do sends an HTTP request to the Docker API and decodes the response into the provided output structure.
func (c *DockerClient) Do(method, path string, body interface{}, out interface{}) error {
	var bodyReader io.Reader
	if body != nil {
		data, _ := json.Marshal(body)
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequest(method, c.baseurl+path, bodyReader)
	if err != nil {
		return err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("Docker API error: %s", resp.Status)
	}

	if out != nil {
		return json.NewDecoder(resp.Body).Decode(out)
	}

	return nil
}

// DoStream returns the raw response body for streaming endpoints (logs, events, pulls).
// The caller is responsible for closing the returned io.ReadCloser.
func (c *DockerClient) DoStream(method, path string, body interface{}) (io.ReadCloser, error) {
	var bodyReader io.Reader
	if body != nil {
		data, _ := json.Marshal(body)
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequest(method, c.baseurl+path, bodyReader)
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode >= 400 {
		resp.Body.Close()
		return nil, fmt.Errorf("Docker API error: %s", resp.Status)
	}

	return resp.Body, nil
}

// Hijack establishes a raw connection to the Docker daemon for endpoints that
// require streaming (exec, attach). It uses the same transport as regular API
// calls — Unix socket on Linux, TCP on Windows.
func (c *DockerClient) Hijack(ctx context.Context, path string, body interface{}) (net.Conn, *bufio.Reader, error) {
	var bodyData []byte
	if body != nil {
		var err error
		bodyData, err = json.Marshal(body)
		if err != nil {
			return nil, nil, err
		}
	}

	// Dial via the stored dial function (Unix socket or TCP).
	// Fall back to plain TCP when no dial function is configured (e.g. in tests).
	var (
		conn    net.Conn
		dialErr error
	)
	if c.dial != nil {
		conn, dialErr = c.dial(ctx)
	} else {
		req2, _ := http.NewRequestWithContext(ctx, "POST", c.baseurl+path, nil)
		conn, dialErr = (&net.Dialer{}).DialContext(ctx, "tcp", req2.URL.Host)
	}
	if dialErr != nil {
		return nil, nil, fmt.Errorf("dial failed: %w", dialErr)
	}

	// Build and write the raw HTTP request.
	req, err := http.NewRequestWithContext(ctx, "POST", c.baseurl+path, bytes.NewReader(bodyData))
	if err != nil {
		conn.Close()
		return nil, nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Connection", "Upgrade")
	req.Header.Set("Upgrade", "tcp")
	// Override host so the request line uses "localhost" even over a Unix socket.
	req.Host = "localhost"

	if err := req.Write(conn); err != nil {
		conn.Close()
		return nil, nil, fmt.Errorf("write request: %w", err)
	}

	// Read the response header to confirm the upgrade succeeded.
	reader := bufio.NewReader(conn)
	resp, err := http.ReadResponse(reader, req)
	if err != nil {
		conn.Close()
		return nil, nil, fmt.Errorf("read response: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusSwitchingProtocols && resp.StatusCode != http.StatusOK {
		conn.Close()
		return nil, nil, fmt.Errorf("unexpected status %d", resp.StatusCode)
	}

	return conn, reader, nil
}
