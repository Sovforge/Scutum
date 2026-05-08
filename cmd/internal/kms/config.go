package kms

import (
	"context"
	"fmt"
	"os"
	"strings"
)

type Config struct {
	Provider string `toml:"provider"`

	Local struct {
		KeyFile string `toml:"key_file"`
	} `toml:"local"`

	Vault struct {
		Addr      string `toml:"addr"`
		KeyName   string `toml:"key_name"`
		TokenFile string `toml:"token_file"`
	} `toml:"vault"`

	AWS struct {
		Region    string `toml:"region"`
		KeyID     string `toml:"key_id"`
		AccessKey string `toml:"access_key"`
		SecretKey string `toml:"secret_key"`
	} `toml:"aws"`

	GCP struct {
		ProjectID  string `toml:"project_id"`
		LocationID string `toml:"location_id"`
		KeyRingID  string `toml:"key_ring_id"`
		KeyID      string `toml:"key_id"`
		TokenFile  string `toml:"token_file"`
	} `toml:"gcp"`

	Azure struct {
		VaultURL  string `toml:"vault_url"`
		KeyName   string `toml:"key_name"`
		TenantID  string `toml:"tenant_id"`
		ClientID  string `toml:"client_id"`
		TokenFile string `toml:"token_file"`
	} `toml:"azure"`
}

// FromConfig initialises the correct KMS provider from a parsed config.
func FromConfig(ctx context.Context, cfg Config) (Provider, error) {
	switch strings.ToLower(cfg.Provider) {
	case "local":
		path := cfg.Local.KeyFile
		if path == "" {
			path = "/run/secrets/master.key"
		}
		return NewLocalKeyProvider(path)

	case "vault":
		if cfg.Vault.Addr == "" || cfg.Vault.KeyName == "" {
			return nil, fmt.Errorf("vault: addr and key_name are required")
		}
		return NewVaultProvider(cfg.Vault.Addr, cfg.Vault.KeyName, cfg.Vault.TokenFile)

	case "aws":
		if cfg.AWS.Region == "" || cfg.AWS.KeyID == "" {
			return nil, fmt.Errorf("aws: region and key_id are required")
		}
		return NewAWSProvider(cfg.AWS.Region, cfg.AWS.KeyID, cfg.AWS.AccessKey, cfg.AWS.SecretKey)

	case "gcp":
		if cfg.GCP.ProjectID == "" || cfg.GCP.LocationID == "" ||
			cfg.GCP.KeyRingID == "" || cfg.GCP.KeyID == "" {
			return nil, fmt.Errorf("gcp: project_id, location_id, key_ring_id, and key_id are required")
		}
		return NewGCPProvider(cfg.GCP.ProjectID, cfg.GCP.LocationID,
			cfg.GCP.KeyRingID, cfg.GCP.KeyID, cfg.GCP.TokenFile)

	case "azure":
		if cfg.Azure.VaultURL == "" || cfg.Azure.KeyName == "" ||
			cfg.Azure.TenantID == "" || cfg.Azure.ClientID == "" {
			return nil, fmt.Errorf("azure: vault_url, key_name, tenant_id, and client_id are required")
		}
		return NewAzureProvider(cfg.Azure.VaultURL, cfg.Azure.KeyName,
			cfg.Azure.TenantID, cfg.Azure.ClientID, cfg.Azure.TokenFile)

	default:
		return nil, fmt.Errorf("unknown kms provider %q — valid options: local, vault, aws, gcp, azure", cfg.Provider)
	}
}

// LoadConfig reads and parses a TOML KMS config file.
// If the file does not exist, returns a default local config.
func LoadConfig(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return Config{Provider: "local"}, nil
	}
	if err != nil {
		return Config{}, fmt.Errorf("read kms config: %w", err)
	}
	var cfg Config
	if err := parseTOML(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse kms config: %w", err)
	}
	return cfg, nil
}

// parseTOML is a minimal TOML parser covering the subset used by kms.Config.
// This avoids any external TOML dependency.
func parseTOML(data []byte, cfg *Config) error {
	lines := strings.Split(string(data), "\n")
	section := ""

	for _, raw := range lines {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = strings.ToLower(line[1 : len(line)-1])
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		val := strings.Trim(strings.TrimSpace(parts[1]), `"`)

		switch section {
		case "":
			if key == "provider" {
				cfg.Provider = val
			}
		case "local":
			if key == "key_file" {
				cfg.Local.KeyFile = val
			}
		case "vault":
			switch key {
			case "addr":
				cfg.Vault.Addr = val
			case "key_name":
				cfg.Vault.KeyName = val
			case "token_file":
				cfg.Vault.TokenFile = val
			}
		case "aws":
			switch key {
			case "region":
				cfg.AWS.Region = val
			case "key_id":
				cfg.AWS.KeyID = val
			case "access_key":
				cfg.AWS.AccessKey = val
			case "secret_key":
				cfg.AWS.SecretKey = val
			}
		case "gcp":
			switch key {
			case "project_id":
				cfg.GCP.ProjectID = val
			case "location_id":
				cfg.GCP.LocationID = val
			case "key_ring_id":
				cfg.GCP.KeyRingID = val
			case "key_id":
				cfg.GCP.KeyID = val
			case "token_file":
				cfg.GCP.TokenFile = val
			}
		case "azure":
			switch key {
			case "vault_url":
				cfg.Azure.VaultURL = val
			case "key_name":
				cfg.Azure.KeyName = val
			case "tenant_id":
				cfg.Azure.TenantID = val
			case "client_id":
				cfg.Azure.ClientID = val
			case "token_file":
				cfg.Azure.TokenFile = val
			}
		}
	}

	if cfg.Provider == "" {
		cfg.Provider = "local"
	}
	return nil
}
