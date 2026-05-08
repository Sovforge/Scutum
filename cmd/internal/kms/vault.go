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

type VaultProvider struct {
	addr    string
	token   string
	keyName string
	client  *http.Client
}

// NewVaultProvider reads the Vault token from a file.
// Falls back to the VAULT_TOKEN environment variable if the file does not exist.
func NewVaultProvider(addr, keyName, tokenFilePath string) (*VaultProvider, error) {
	return NewVaultProviderWithClient(addr, keyName, tokenFilePath, nil)
}

func NewVaultProviderWithClient(addr, keyName, tokenFilePath string, client *http.Client) (*VaultProvider, error) {
	token, err := os.ReadFile(tokenFilePath)
	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("read vault token file: %w", err)
	}

	t := strings.TrimSpace(string(token))
	if t == "" {
		t = os.Getenv("VAULT_TOKEN")
	}
	if t == "" {
		return nil, fmt.Errorf("no vault token found in %s or VAULT_TOKEN", tokenFilePath)
	}
	if client == nil {
		client = &http.Client{}
	}

	return &VaultProvider{
		addr:    addr,
		token:   t,
		keyName: keyName,
		client:  client,
	}, nil
}

func (v *VaultProvider) Encrypt(ctx context.Context, plaintext []byte) ([]byte, error) {
	encoded := base64.StdEncoding.EncodeToString(plaintext)
	body, _ := json.Marshal(map[string]string{"plaintext": encoded})

	resp, err := v.vaultPost(ctx, fmt.Sprintf("/v1/transit/encrypt/%s", v.keyName), body)
	if err != nil {
		return nil, err
	}

	var result struct {
		Data struct {
			Ciphertext string `json:"ciphertext"`
		} `json:"data"`
	}
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("vault encrypt parse: %w", err)
	}
	return []byte(result.Data.Ciphertext), nil
}

func (v *VaultProvider) Decrypt(ctx context.Context, ciphertext []byte) ([]byte, error) {
	body, _ := json.Marshal(map[string]string{"ciphertext": string(ciphertext)})

	resp, err := v.vaultPost(ctx, fmt.Sprintf("/v1/transit/decrypt/%s", v.keyName), body)
	if err != nil {
		return nil, err
	}

	var result struct {
		Data struct {
			Plaintext string `json:"plaintext"`
		} `json:"data"`
	}
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("vault decrypt parse: %w", err)
	}
	return base64.StdEncoding.DecodeString(result.Data.Plaintext)
}

func (v *VaultProvider) Name() string { return "vault" }

// Wipe is a no-op for Vault — key material lives in Vault, not in memory.
func (v *VaultProvider) Wipe() {}

// LoadMasterKey is not supported for Vault. Emergency recovery requires
// switching to the local provider first.
func (v *VaultProvider) LoadMasterKey(_ []byte) error {
	return errors.New("vault: LoadMasterKey not supported — emergency recovery requires the local provider")
}

// MasterKeyDigest is not supported for Vault. The master key never exists
// in memory so no digest can be computed.
func (v *VaultProvider) MasterKeyDigest() ([]byte, error) {
	return nil, errors.New("vault: MasterKeyDigest not supported — emergency recovery requires the local provider")
}

func (v *VaultProvider) vaultPost(ctx context.Context, path string, body []byte) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		v.addr+path, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("vault request: %w", err)
	}
	req.Header.Set("X-Vault-Token", v.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := v.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("vault post: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("vault read response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("vault error %d: %s", resp.StatusCode, data)
	}
	return data, nil
}
