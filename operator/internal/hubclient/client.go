// Package hubclient provides an HTTP client for the Scutum hub API.
package hubclient

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// CreateNodeRequest is the body sent to POST /api/nodes.
type CreateNodeRequest struct {
	Name      string `json:"name"`
	Type      string `json:"type"`
	Address   string `json:"address"`
	PublicKey string `json:"public_key"`
}

// NodeRecord mirrors the hub's store.NodeRecord.
type NodeRecord struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Type      string `json:"type"`
	Address   string `json:"address"`
	PublicKey string `json:"public_key"`
}

// BootstrapInfo is the response from GET /api/operator/bootstrap.
type BootstrapInfo struct {
	HubWGPublicKey string `json:"hub_wg_public_key"`
	HubWGPort      int    `json:"hub_wg_port"`
	HubHMACKey     string `json:"hub_hmac_key"`
	HubMeshCIDR    string `json:"hub_mesh_cidr"`
}

// Client talks to the Scutum hub REST API.
type Client struct {
	http *http.Client
}

// New creates a Client. When tlsSkipVerify is true the client will accept
// self-signed certificates — appropriate for operator-internal calls.
func New(tlsSkipVerify bool) *Client {
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: tlsSkipVerify, //nolint:gosec
			MinVersion:         tls.VersionTLS12,
		},
	}
	return &Client{
		http: &http.Client{
			Timeout:   30 * time.Second,
			Transport: transport,
		},
	}
}

// Login authenticates with the hub and returns a JWT token.
func (c *Client) Login(ctx context.Context, apiBase, username, password string) (string, error) {
	body, _ := json.Marshal(map[string]string{
		"username": username,
		"password": password,
	})
	resp, err := c.post(ctx, apiBase+"/api/auth/login", "", body)
	if err != nil {
		return "", fmt.Errorf("login: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("login: hub returned HTTP %d: %s", resp.StatusCode, string(raw))
	}

	var result struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("login: decode response: %w", err)
	}
	if result.Token == "" {
		return "", fmt.Errorf("login: empty token in response")
	}
	return result.Token, nil
}

// GetNodes returns all nodes registered with the hub.
func (c *Client) GetNodes(ctx context.Context, apiBase, token string) ([]NodeRecord, error) {
	resp, err := c.get(ctx, apiBase+"/api/nodes", token)
	if err != nil {
		return nil, fmt.Errorf("get nodes: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("get nodes: hub returned HTTP %d: %s", resp.StatusCode, string(raw))
	}

	var nodes []NodeRecord
	if err := json.NewDecoder(resp.Body).Decode(&nodes); err != nil {
		return nil, fmt.Errorf("get nodes: decode: %w", err)
	}
	return nodes, nil
}

// CreateNode registers a new node with the hub and returns its record.
func (c *Client) CreateNode(ctx context.Context, apiBase, token string, req CreateNodeRequest) (NodeRecord, error) {
	body, _ := json.Marshal(req)
	resp, err := c.post(ctx, apiBase+"/api/nodes", token, body)
	if err != nil {
		return NodeRecord{}, fmt.Errorf("create node: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		raw, _ := io.ReadAll(resp.Body)
		return NodeRecord{}, fmt.Errorf("create node: hub returned HTTP %d: %s", resp.StatusCode, string(raw))
	}

	var node NodeRecord
	if err := json.NewDecoder(resp.Body).Decode(&node); err != nil {
		return NodeRecord{}, fmt.Errorf("create node: decode: %w", err)
	}
	return node, nil
}

// RegisterEdge calls POST /api/sync/register-edge to store the edge sync token on the hub.
func (c *Client) RegisterEdge(ctx context.Context, apiBase, token, nodeID, syncURL, edgeToken string) error {
	body, _ := json.Marshal(map[string]string{
		"node_id": nodeID,
		"url":     syncURL,
		"token":   edgeToken,
	})
	resp, err := c.post(ctx, apiBase+"/api/sync/register-edge", token, body)
	if err != nil {
		return fmt.Errorf("register edge: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("register edge: hub returned HTTP %d: %s", resp.StatusCode, string(raw))
	}
	return nil
}

// GetBootstrap fetches the operator bootstrap info from GET /api/operator/bootstrap.
func (c *Client) GetBootstrap(ctx context.Context, apiBase, token string) (BootstrapInfo, error) {
	resp, err := c.get(ctx, apiBase+"/api/operator/bootstrap", token)
	if err != nil {
		return BootstrapInfo{}, fmt.Errorf("get bootstrap: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(resp.Body)
		return BootstrapInfo{}, fmt.Errorf("get bootstrap: hub returned HTTP %d: %s", resp.StatusCode, string(raw))
	}

	var info BootstrapInfo
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return BootstrapInfo{}, fmt.Errorf("get bootstrap: decode: %w", err)
	}
	return info, nil
}

// --- helpers ---

func (c *Client) get(ctx context.Context, url, token string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	return c.http.Do(req)
}

func (c *Client) post(ctx context.Context, url, token string, body []byte) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	return c.http.Do(req)
}
