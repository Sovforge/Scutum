package kms

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
)

type GCPProvider struct {
	projectID  string
	locationID string
	keyRingID  string
	keyID      string
	token      string
	client     *http.Client
}

func NewGCPProvider(projectID, locationID, keyRingID, keyID, tokenFile string) (*GCPProvider, error) {
	return NewGCPProviderWithClient(projectID, locationID, keyRingID, keyID, tokenFile, nil)
}

func NewGCPProviderWithClient(projectID, locationID, keyRingID, keyID, tokenFile string, client *http.Client) (*GCPProvider, error) {
	token, err := loadToken(tokenFile, "GOOGLE_TOKEN")
	if err != nil {
		return nil, fmt.Errorf("gcp: %w", err)
	}
	if client == nil {
		client = &http.Client{}
	}
	return &GCPProvider{
		projectID:  projectID,
		locationID: locationID,
		keyRingID:  keyRingID,
		keyID:      keyID,
		token:      token,
		client:     client,
	}, nil
}

func (g *GCPProvider) Encrypt(ctx context.Context, plaintext []byte) ([]byte, error) {
	url := fmt.Sprintf(
		"https://cloudkms.googleapis.com/v1/projects/%s/locations/%s/keyRings/%s/cryptoKeys/%s:encrypt",
		g.projectID, g.locationID, g.keyRingID, g.keyID,
	)
	body, _ := json.Marshal(map[string]string{
		"plaintext": base64.StdEncoding.EncodeToString(plaintext),
	})

	resp, err := g.post(ctx, url, body)
	if err != nil {
		return nil, err
	}

	var result struct {
		Ciphertext string `json:"ciphertext"`
	}
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("gcp encrypt parse: %w", err)
	}
	return base64.StdEncoding.DecodeString(result.Ciphertext)
}

func (g *GCPProvider) Decrypt(ctx context.Context, ciphertext []byte) ([]byte, error) {
	url := fmt.Sprintf(
		"https://cloudkms.googleapis.com/v1/projects/%s/locations/%s/keyRings/%s/cryptoKeys/%s:decrypt",
		g.projectID, g.locationID, g.keyRingID, g.keyID,
	)
	body, _ := json.Marshal(map[string]string{
		"ciphertext": base64.StdEncoding.EncodeToString(ciphertext),
	})

	resp, err := g.post(ctx, url, body)
	if err != nil {
		return nil, err
	}

	var result struct {
		Plaintext string `json:"plaintext"`
	}
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("gcp decrypt parse: %w", err)
	}
	return base64.StdEncoding.DecodeString(result.Plaintext)
}

func (g *GCPProvider) Name() string { return "gcp" }

// Wipe is a no-op for GCP — key material lives in Cloud KMS, not in memory.
func (g *GCPProvider) Wipe() {}

// LoadMasterKey is not supported for GCP Cloud KMS. Emergency recovery
// requires switching to the local provider first.
func (g *GCPProvider) LoadMasterKey(_ []byte) error {
	return errors.New("gcp: LoadMasterKey not supported — emergency recovery requires the local provider")
}

// MasterKeyDigest is not supported for GCP Cloud KMS. The master key never
// exists in memory so no digest can be computed.
func (g *GCPProvider) MasterKeyDigest() ([]byte, error) {
	return nil, errors.New("gcp: MasterKeyDigest not supported — emergency recovery requires the local provider")
}

func (g *GCPProvider) post(ctx context.Context, url string, body []byte) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("gcp request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+g.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := g.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("gcp kms call: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("gcp read response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("gcp kms error %d: %s", resp.StatusCode, data)
	}
	return data, nil
}

// loadToken reads a token from a file, falling back to an environment variable.
func loadToken(filePath, envVar string) (string, error) {
	if filePath != "" {
		data, err := os.ReadFile(filePath)
		if err == nil {
			return strings.TrimSpace(string(data)), nil
		}
		if !os.IsNotExist(err) {
			return "", fmt.Errorf("read token file: %w", err)
		}
	}
	token := os.Getenv(envVar)
	if token == "" {
		return "", fmt.Errorf("no token found in %s or %s", filePath, envVar)
	}
	return token, nil
}
