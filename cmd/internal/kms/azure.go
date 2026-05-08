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
	"net/url"
	"strings"
)

type AzureProvider struct {
	vaultURL  string
	keyName   string
	tenantID  string
	clientID  string
	tokenFile string
	client    *http.Client
}

func NewAzureProvider(vaultURL, keyName, tenantID, clientID, tokenFile string) (*AzureProvider, error) {
	return NewAzureProviderWithClient(vaultURL, keyName, tenantID, clientID, tokenFile, nil)
}

func NewAzureProviderWithClient(vaultURL, keyName, tenantID, clientID, tokenFile string, client *http.Client) (*AzureProvider, error) {
	if client == nil {
		client = &http.Client{}
	}
	return &AzureProvider{
		vaultURL:  strings.TrimRight(vaultURL, "/"),
		keyName:   keyName,
		tenantID:  tenantID,
		clientID:  clientID,
		tokenFile: tokenFile,
		client:    client,
	}, nil
}

func (a *AzureProvider) Encrypt(ctx context.Context, plaintext []byte) ([]byte, error) {
	token, err := a.getToken(ctx)
	if err != nil {
		return nil, err
	}

	url := fmt.Sprintf("%s/keys/%s/encrypt?api-version=7.4", a.vaultURL, a.keyName)
	body, _ := json.Marshal(map[string]string{
		"alg":   "RSA-OAEP-256",
		"value": base64.RawURLEncoding.EncodeToString(plaintext),
	})

	resp, err := a.post(ctx, url, body, token)
	if err != nil {
		return nil, err
	}

	var result struct {
		Value string `json:"value"`
	}
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("azure encrypt parse: %w", err)
	}
	return base64.RawURLEncoding.DecodeString(result.Value)
}

func (a *AzureProvider) Decrypt(ctx context.Context, ciphertext []byte) ([]byte, error) {
	token, err := a.getToken(ctx)
	if err != nil {
		return nil, err
	}

	url := fmt.Sprintf("%s/keys/%s/decrypt?api-version=7.4", a.vaultURL, a.keyName)
	body, _ := json.Marshal(map[string]string{
		"alg":   "RSA-OAEP-256",
		"value": base64.RawURLEncoding.EncodeToString(ciphertext),
	})

	resp, err := a.post(ctx, url, body, token)
	if err != nil {
		return nil, err
	}

	var result struct {
		Value string `json:"value"`
	}
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("azure decrypt parse: %w", err)
	}
	return base64.RawURLEncoding.DecodeString(result.Value)
}

func (a *AzureProvider) Name() string { return "azure" }

// Wipe is a no-op for Azure — key material lives in Azure Key Vault, not in memory.
func (a *AzureProvider) Wipe() {}

// LoadMasterKey is not supported for Azure Key Vault. Emergency recovery
// requires switching to the local provider first.
func (a *AzureProvider) LoadMasterKey(_ []byte) error {
	return errors.New("azure: LoadMasterKey not supported — emergency recovery requires the local provider")
}

// MasterKeyDigest is not supported for Azure Key Vault. The master key never
// exists in memory so no digest can be computed.
func (a *AzureProvider) MasterKeyDigest() ([]byte, error) {
	return nil, errors.New("azure: MasterKeyDigest not supported — emergency recovery requires the local provider")
}

func (a *AzureProvider) getToken(ctx context.Context) (string, error) {
	token, err := loadToken(a.tokenFile, "AZURE_TOKEN")
	if err == nil {
		return token, nil
	}

	endpoint := fmt.Sprintf("https://login.microsoftonline.com/%s/oauth2/v2.0/token", a.tenantID)
	form := url.Values{
		"grant_type": {"client_credentials"},
		"client_id":  {a.clientID},
		"scope":      {"https://vault.azure.net/.default"},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint,
		strings.NewReader(form.Encode()))
	if err != nil {
		return "", fmt.Errorf("azure token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := a.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("azure token call: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("azure token read: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("azure token error %d: %s", resp.StatusCode, data)
	}

	var result struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return "", fmt.Errorf("azure token parse: %w", err)
	}
	return result.AccessToken, nil
}

func (a *AzureProvider) post(ctx context.Context, endpoint string, body []byte, token string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("azure request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := a.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("azure kms call: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("azure read response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("azure kms error %d: %s", resp.StatusCode, data)
	}
	return data, nil
}
