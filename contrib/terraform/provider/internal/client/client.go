package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// Client is a minimal Scutum API client used by the Terraform provider.
type Client struct {
	base   string
	apiKey string
	http   *http.Client
}

func New(baseURL, apiKey string) *Client {
	return &Client{base: baseURL, apiKey: apiKey, http: &http.Client{}}
}

func (c *Client) do(ctx context.Context, method, path string, body, out any) error {
	var r io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		r = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.base+"/api"+path, r)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return fmt.Errorf("scutum API %s %s: %d %s", method, path, resp.StatusCode, string(data))
	}
	if out != nil && len(data) > 0 {
		return json.Unmarshal(data, out)
	}
	return nil
}

// ── Node ──────────────────────────────────────────────────────────────────────

type Node struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Type      string `json:"type"`
	Address   string `json:"address"`
	PublicKey string `json:"public_key"`
}

func (c *Client) CreateNode(ctx context.Context, n Node) (Node, error) {
	var out Node
	return out, c.do(ctx, http.MethodPost, "/nodes", n, &out)
}

func (c *Client) GetNode(ctx context.Context, id string) (Node, error) {
	var out Node
	return out, c.do(ctx, http.MethodGet, "/nodes/"+id, nil, &out)
}

func (c *Client) DeleteNode(ctx context.Context, id string) error {
	return c.do(ctx, http.MethodDelete, "/nodes/"+id, nil, nil)
}

// ── Node group ────────────────────────────────────────────────────────────────

type Group struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

func (c *Client) CreateGroup(ctx context.Context, g Group) (Group, error) {
	var out Group
	return out, c.do(ctx, http.MethodPost, "/groups", g, &out)
}

func (c *Client) GetGroup(ctx context.Context, id string) (Group, error) {
	var out Group
	return out, c.do(ctx, http.MethodGet, "/groups/"+id, nil, &out)
}

func (c *Client) DeleteGroup(ctx context.Context, id string) error {
	return c.do(ctx, http.MethodDelete, "/groups/"+id, nil, nil)
}

func (c *Client) SetGroupMembers(ctx context.Context, groupID string, nodeIDs []string) error {
	return c.do(ctx, http.MethodPut, "/groups/"+groupID+"/nodes", map[string]any{"node_ids": nodeIDs}, nil)
}

func (c *Client) GetGroupNodes(ctx context.Context, groupID string) ([]Node, error) {
	var out []Node
	return out, c.do(ctx, http.MethodGet, "/groups/"+groupID+"/nodes", nil, &out)
}

// ── Federation peer ───────────────────────────────────────────────────────────

type FederationPeer struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	HubURL      string `json:"hub_url"`
	WGEndpoint  string `json:"wg_endpoint"`
	WGPublicKey string `json:"wg_public_key"`
	MeshCIDR    string `json:"mesh_cidr"`
	AllowedIPs  string `json:"allowed_ips"`
}

func (c *Client) CreateFederationPeer(ctx context.Context, p FederationPeer) (FederationPeer, error) {
	var out FederationPeer
	return out, c.do(ctx, http.MethodPost, "/federation/peers", p, &out)
}

func (c *Client) GetFederationPeer(ctx context.Context, id string) (FederationPeer, error) {
	var out FederationPeer
	return out, c.do(ctx, http.MethodGet, "/federation/peers/"+id, nil, &out)
}

func (c *Client) DeleteFederationPeer(ctx context.Context, id string) error {
	return c.do(ctx, http.MethodDelete, "/federation/peers/"+id, nil, nil)
}

// ── Webhook ───────────────────────────────────────────────────────────────────

type Webhook struct {
	ID      string   `json:"id"`
	Name    string   `json:"name"`
	URL     string   `json:"url"`
	Secret  string   `json:"secret"`
	Events  []string `json:"events"`
	Enabled bool     `json:"enabled"`
}

func (c *Client) CreateWebhook(ctx context.Context, w Webhook) (Webhook, error) {
	var out Webhook
	return out, c.do(ctx, http.MethodPost, "/webhooks", w, &out)
}

func (c *Client) GetWebhook(ctx context.Context, id string) (Webhook, error) {
	var out Webhook
	return out, c.do(ctx, http.MethodGet, "/webhooks/"+id, nil, &out)
}

func (c *Client) UpdateWebhook(ctx context.Context, id string, w Webhook) (Webhook, error) {
	var out Webhook
	return out, c.do(ctx, http.MethodPut, "/webhooks/"+id, w, &out)
}

func (c *Client) DeleteWebhook(ctx context.Context, id string) error {
	return c.do(ctx, http.MethodDelete, "/webhooks/"+id, nil, nil)
}

// ── User ──────────────────────────────────────────────────────────────────────

type User struct {
	ID       string `json:"id"`
	Username string `json:"username"`
	Email    string `json:"email"`
	Password string `json:"password,omitempty"`
	RoleID   string `json:"role_id,omitempty"`
}

func (c *Client) CreateUser(ctx context.Context, u User) (User, error) {
	var out User
	return out, c.do(ctx, http.MethodPost, "/users", u, &out)
}

func (c *Client) GetUser(ctx context.Context, id string) (User, error) {
	var out User
	return out, c.do(ctx, http.MethodGet, "/users/"+id, nil, &out)
}

func (c *Client) UpdateUser(ctx context.Context, id string, u User) (User, error) {
	var out User
	return out, c.do(ctx, http.MethodPut, "/users/"+id, u, &out)
}

func (c *Client) DeleteUser(ctx context.Context, id string) error {
	return c.do(ctx, http.MethodDelete, "/users/"+id, nil, nil)
}