package handlers

import (
	"context"
	cryptorand "crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	"scutum/cmd/internal/auth"
	"scutum/cmd/internal/kms"
	"scutum/cmd/internal/store"
	"scutum/cmd/internal/utils"

	"github.com/google/uuid"
)

type setupStore interface {
	IsSetupComplete(ctx context.Context) (bool, error)
	MarkSetupComplete(ctx context.Context) error
	SetKMSProvider(ctx context.Context, provider string) error
	SetInstallType(ctx context.Context, t store.InstallType) error
	CreateUser(ctx context.Context, id, username, passwordHash string) error
	AssignRole(ctx context.Context, userID, roleID string) error
	SetSecret(ctx context.Context, key string, value []byte) error
	GetSecret(ctx context.Context, key string) ([]byte, error)
	SetWireGuardPrivateKey(ctx context.Context, ifaceName string, privateKey []byte) error
	CreateNode(ctx context.Context, n store.NodeRecord) error
	UpsertWGPeer(ctx context.Context, p store.WGPeerRecord) error
}

type SetupHandler struct {
	store      setupStore
	configPath string
	APIPort    string // set by main so the local node record has the right host:port
	onComplete func(provider kms.Provider)
}

func NewSetupHandler(store setupStore, configPath string, onComplete func(kms.Provider)) *SetupHandler {
	return &SetupHandler{
		store:      store,
		configPath: configPath,
		onComplete: onComplete,
	}
}

type kmsConfig struct {
	Provider string `json:"provider"`

	Local *struct {
		KeyFile string `json:"key_file"`
	} `json:"local,omitempty"`

	Vault *struct {
		Addr      string `json:"addr"`
		KeyName   string `json:"key_name"`
		TokenFile string `json:"token_file"`
	} `json:"vault,omitempty"`

	AWS *struct {
		Region    string `json:"region"`
		KeyID     string `json:"key_id"`
		AccessKey string `json:"access_key"`
		SecretKey string `json:"secret_key"`
	} `json:"aws,omitempty"`

	GCP *struct {
		ProjectID  string `json:"project_id"`
		LocationID string `json:"location_id"`
		KeyRingID  string `json:"key_ring_id"`
		KeyID      string `json:"key_id"`
		TokenFile  string `json:"token_file"`
	} `json:"gcp,omitempty"`

	Azure *struct {
		VaultURL  string `json:"vault_url"`
		KeyName   string `json:"key_name"`
		TenantID  string `json:"tenant_id"`
		ClientID  string `json:"client_id"`
		TokenFile string `json:"token_file"`
	} `json:"azure,omitempty"`
}

type wireguardConfig struct {
	ListenPort    int    `json:"listen_port,omitempty"`
	Address       string `json:"address,omitempty"`
	MTU           int    `json:"mtu,omitempty"`
	HubEndpoint   string `json:"hub_endpoint,omitempty"`
	HubPublicKey  string `json:"hub_public_key,omitempty"`
	HubAllowedIPs string `json:"hub_allowed_ips,omitempty"`
	HubHMACKey    string `json:"hub_hmac_key,omitempty"`
}

type setupRequest struct {
	InstallType string          `json:"install_type"`
	KMS         kmsConfig       `json:"kms"`
	WireGuard   wireguardConfig `json:"wireguard"`
	Admin       struct {
		Username string `json:"username"`
		Password string `json:"password"`
	} `json:"admin"`
	Recovery struct {
		NShares   int `json:"n_shares"`
		Threshold int `json:"threshold"`
	} `json:"recovery"`
}

type wireGuardSetupResult struct {
	PublicKey  string `json:"public_key"`
	Address    string `json:"address,omitempty"`
	ListenPort int    `json:"listen_port,omitempty"`
	Warning    string `json:"warning,omitempty"`
}

type setupResponse struct {
	Message        string                `json:"message"`
	AdminID        string                `json:"admin_id"`
	KMSProvider    string                `json:"kms_provider"`
	InstallType    string                `json:"install_type"`
	WireGuard      *wireGuardSetupResult `json:"wireguard,omitempty"`
	RecoveryShares []string              `json:"recovery_shares,omitempty"`
}

func (h *SetupHandler) HandleStatus(w http.ResponseWriter, r *http.Request) {
	complete, err := h.store.IsSetupComplete(r.Context())
	if err != nil {
		http.Error(w, "failed to check setup state", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"complete": complete})
}

func (h *SetupHandler) HandleSetup(w http.ResponseWriter, r *http.Request) {
	if complete, err := h.store.IsSetupComplete(r.Context()); err != nil {
		http.Error(w, "failed to check setup state", http.StatusInternalServerError)
		return
	} else if complete {
		http.Error(w, "setup has already been completed", http.StatusConflict)
		return
	}

	var req setupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	switch store.InstallType(req.InstallType) {
	case store.InstallHub, store.InstallRemote, store.InstallCombined:
	default:
		http.Error(w, "install_type must be hub, remote, or combined", http.StatusBadRequest)
		return
	}

	if req.InstallType != string(store.InstallRemote) {
		if req.Admin.Username == "" || req.Admin.Password == "" {
			http.Error(w, "admin username and password are required", http.StatusBadRequest)
			return
		}

		var hasUpper, hasLower, hasNumber, hasSpecial bool
		for _, c := range req.Admin.Password {
			switch {
			case unicode.IsUpper(c):
				hasUpper = true
			case unicode.IsLower(c):
				hasLower = true
			case unicode.IsNumber(c) || unicode.IsDigit(c):
				hasNumber = true
			case unicode.IsPunct(c) || unicode.IsSymbol(c):
				hasSpecial = true
			}
		}
		if len(req.Admin.Password) < 12 || !hasUpper || !hasLower || !hasNumber || !hasSpecial {
			http.Error(w, "admin password must be at least 12 characters and contain uppercase, lowercase, numbers, and special characters", http.StatusBadRequest)
			return
		}
	}

	if err := validateWireGuardConfig(req); err != nil {
		http.Error(w, fmt.Sprintf("wireguard: %v", err), http.StatusBadRequest)
		return
	}

	// If WireGuard is unavailable, try to auto-install wireguard-go before
	// committing any state. On success we restart; the client retries setup.
	if !utils.IsWireGuardAvailable() {
		installCtx, cancel := context.WithTimeout(r.Context(), 5*time.Minute)
		defer cancel()
		if err := utils.TryInstallWireGuardGo(installCtx); err == nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusAccepted)
			json.NewEncoder(w).Encode(map[string]string{
				"status":  "restarting",
				"message": "WireGuard has been installed. The server is restarting — please retry setup in a few seconds.",
			})
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
			go func() {
				time.Sleep(400 * time.Millisecond)
				_ = utils.SelfRestart()
			}()
			return
		}
		// Auto-install failed — continue; setupWireGuard will degrade gracefully.
	}

	cfg, err := buildKMSConfig(req.KMS, filepath.Dir(h.configPath))
	if err != nil {
		http.Error(w, fmt.Sprintf("kms config: %v", err), http.StatusBadRequest)
		return
	}

	// For local KMS: generate the master key here so we can split it into
	// recovery shares before writing it to disk. For cloud providers the key
	// never touches this process.
	var recoveryShares []string
	if cfg.Provider == "local" {
		n, t := validatedRecoveryParams(req.Recovery.NShares, req.Recovery.Threshold)

		masterKey := make([]byte, 32)
		if _, err := cryptorand.Read(masterKey); err != nil {
			handlerInternalErr(w, r, "generate master key", err)
			return
		}

		shares, err := kms.EmergencySetup(masterKey, n, t)
		if err != nil {
			handlerInternalErr(w, r, "split master key into shares", err)
			return
		}

		keyDir := filepath.Dir(cfg.Local.KeyFile)
		if keyDir != "." {
			if err := os.MkdirAll(keyDir, 0700); err != nil {
				handlerInternalErr(w, r, "create key directory", err)
				return
			}
		}
		if err := os.WriteFile(cfg.Local.KeyFile,
			[]byte(hex.EncodeToString(masterKey)+"\n"), 0600); err != nil {
			handlerInternalErr(w, r, "write master key", err)
			return
		}
		for i := range masterKey {
			masterKey[i] = 0
		}

		recoveryShares = make([]string, len(shares))
		for i, s := range shares {
			recoveryShares[i] = s.String()
		}
	}

	provider, err := kms.FromConfig(r.Context(), cfg)
	if err != nil {
		handlerInternalErr(w, r, "kms init", err)
		return
	}

	if err := os.WriteFile(h.configPath, []byte(buildTOML(cfg)), 0600); err != nil {
		handlerInternalErr(w, r, "write kms config", err)
		return
	}

	wgResult, err := h.setupWireGuard(r.Context(), req)
	if err != nil {
		handlerInternalErr(w, r, "wireguard setup", err)
		return
	}

	adminID := ""
	if req.InstallType != string(store.InstallRemote) {
		hash, err := auth.HashPassword(req.Admin.Password)
		if err != nil {
			handlerInternalErr(w, r, "hash password", err)
			return
		}

		adminID = uuid.New().String()
		if err := h.store.CreateUser(r.Context(), adminID, req.Admin.Username, hash); err != nil {
			handlerInternalErr(w, r, "create admin user", err)
			return
		}
		if err := h.store.AssignRole(r.Context(), adminID, "role_admin"); err != nil {
			handlerInternalErr(w, r, "assign admin role", err)
			return
		}
	}
	if err := h.store.SetKMSProvider(r.Context(), cfg.Provider); err != nil {
		handlerInternalErr(w, r, "record kms provider", err)
		return
	}
	if err := h.store.SetInstallType(r.Context(), store.InstallType(req.InstallType)); err != nil {
		handlerInternalErr(w, r, "record install type", err)
		return
	}
	if err := h.store.MarkSetupComplete(r.Context()); err != nil {
		handlerInternalErr(w, r, "mark setup complete", err)
		return
	}

	if req.WireGuard.HubHMACKey != "" {
		if keyBytes, err := hex.DecodeString(req.WireGuard.HubHMACKey); err == nil {
			_ = h.store.SetSecret(r.Context(), "hub_hmac_key", keyBytes)
		}
	}

	// Register the local node so it appears in the mesh/nodes view.
	if wgResult != nil {
		hostname, _ := os.Hostname()
		if hostname == "" {
			hostname = "local"
		}
		// Build a proper host:port API address from the WireGuard mesh IP.
		// wgResult.Address is a CIDR (e.g. "10.x.x.x/24"); strip the prefix.
		apiAddr := wgResult.Address
		if idx := strings.Index(apiAddr, "/"); idx != -1 {
			apiAddr = apiAddr[:idx]
		}
		port := h.APIPort
		if port == "" {
			port = "8080"
		}
		apiAddr = apiAddr + ":" + strings.TrimPrefix(port, ":")
		_ = h.store.CreateNode(r.Context(), store.NodeRecord{
			ID:        uuid.New().String(),
			Name:      hostname,
			Type:      req.InstallType,
			Address:   apiAddr,
			PublicKey: wgResult.PublicKey,
		})
	}

	if h.onComplete != nil {
		h.onComplete(provider)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(setupResponse{
		Message:        "setup complete",
		AdminID:        adminID,
		KMSProvider:    cfg.Provider,
		InstallType:    req.InstallType,
		WireGuard:      wgResult,
		RecoveryShares: recoveryShares,
	})
}

func (h *SetupHandler) HandleTestKMS(w http.ResponseWriter, r *http.Request) {
	var req struct {
		KMS kmsConfig `json:"kms"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	cfg, err := buildKMSConfig(req.KMS, filepath.Dir(h.configPath))
	if err != nil {
		http.Error(w, fmt.Sprintf("kms config: %v", err), http.StatusBadRequest)
		return
	}

	provider, err := kms.FromConfig(r.Context(), cfg)
	if err != nil {
		http.Error(w, "KMS connection failed. Check your provider credentials and configuration.", http.StatusUnprocessableEntity)
		return
	}

	testVal := []byte("scutum-kms-test-" + time.Now().String())
	encrypted, err := provider.Encrypt(r.Context(), testVal)
	if err != nil {
		http.Error(w, "KMS encrypt test failed. The provider is reachable but returned an error.", http.StatusUnprocessableEntity)
		return
	}
	decrypted, err := provider.Decrypt(r.Context(), encrypted)
	if err != nil {
		http.Error(w, "KMS decrypt test failed. The provider is reachable but returned an error.", http.StatusUnprocessableEntity)
		return
	}
	if string(decrypted) != string(testVal) {
		http.Error(w, "KMS round-trip test failed. Encrypt/decrypt produced inconsistent results.", http.StatusUnprocessableEntity)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status":   "ok",
		"provider": cfg.Provider,
	})
}

func (h *SetupHandler) setupWireGuard(ctx context.Context, req setupRequest) (*wireGuardSetupResult, error) {
	wg := req.WireGuard
	installType := store.InstallType(req.InstallType)

	privateKey, err := utils.GenerateKey()
	if err != nil {
		return nil, fmt.Errorf("generate private key: %w", err)
	}

	if err := h.store.SetWireGuardPrivateKey(ctx, "wg0", []byte(privateKey)); err != nil {
		return nil, fmt.Errorf("store private key: %w", err)
	}

	cfg := utils.InterfaceConfig{
		Name:       "wg0",
		PrivateKey: privateKey,
		Address:    wg.Address,
		MTU:        wg.MTU,
	}
	if installType == store.InstallHub || installType == store.InstallCombined {
		cfg.Port = wg.ListenPort
	}

	// Save the configuration so it can be restored on reboot (without the private key)
	configToSave := cfg
	configToSave.PrivateKey = ""
	if configBytes, err := json.Marshal(configToSave); err == nil {
		_ = h.store.SetSecret(ctx, "wg0_config", configBytes)
	}

	iface, err := utils.SetupInterface(cfg)
	if err != nil {
		if !errors.Is(err, utils.ErrWireGuardUnavailable) {
			return nil, fmt.Errorf("setup interface: %w", err)
		}
		// WireGuard is unavailable on this host (no kernel module, no wireguard-go).
		// Derive the public key in pure Go so the result is still useful for peer
		// enrollment, and continue — the interface can be activated later.
		pubKey, _ := utils.DerivePublicKey(privateKey)
		res := &wireGuardSetupResult{
			PublicKey: pubKey,
			Address:   wg.Address,
			Warning:   err.Error(),
		}
		if cfg.Port > 0 {
			res.ListenPort = cfg.Port
		}
		return res, nil
	}

	// Interface is up — add the upstream hub peer where applicable.
	switch installType {
	case store.InstallRemote:
		if err := utils.AddPeer("wg0", wg.HubPublicKey, wg.HubEndpoint, wg.HubAllowedIPs, 25); err != nil {
			return nil, fmt.Errorf("add hub as peer: %w", err)
		}
		// Persist the hub peer so it can be restored after a container restart.
		_ = h.store.CreateNode(ctx, store.NodeRecord{
			ID: "hub", Name: "hub", Type: "hub",
			Address: wg.HubEndpoint, PublicKey: wg.HubPublicKey,
		})
		_ = h.store.UpsertWGPeer(ctx, store.WGPeerRecord{
			NodeID: "hub", Endpoint: wg.HubEndpoint, AllowedIPs: wg.HubAllowedIPs,
		})
	case store.InstallCombined:
		if wg.HubEndpoint != "" && wg.HubPublicKey != "" {
			if err := utils.AddPeer("wg0", wg.HubPublicKey, wg.HubEndpoint, wg.HubAllowedIPs, 25); err != nil {
				return nil, fmt.Errorf("add hub as peer: %w", err)
			}
			_ = h.store.CreateNode(ctx, store.NodeRecord{
				ID: "hub", Name: "hub", Type: "hub",
				Address: wg.HubEndpoint, PublicKey: wg.HubPublicKey,
			})
			_ = h.store.UpsertWGPeer(ctx, store.WGPeerRecord{
				NodeID: "hub", Endpoint: wg.HubEndpoint, AllowedIPs: wg.HubAllowedIPs,
			})
		}
	}

	res := &wireGuardSetupResult{
		PublicKey: iface.PublicKey,
		Address:   wg.Address,
	}
	if cfg.Port > 0 {
		res.ListenPort = cfg.Port
	}
	return res, nil
}

func validateWireGuardConfig(req setupRequest) error {
	wg := req.WireGuard
	installType := store.InstallType(req.InstallType)

	if wg.Address == "" {
		return fmt.Errorf("address is required")
	}

	switch installType {
	case store.InstallHub, store.InstallCombined:
		if wg.ListenPort == 0 {
			return fmt.Errorf("listen_port is required for hub and combined installs")
		}
		if wg.ListenPort < 1 || wg.ListenPort > 65535 {
			return fmt.Errorf("listen_port must be between 1 and 65535")
		}
	}

	switch installType {
	case store.InstallRemote:
		if wg.HubEndpoint == "" {
			return fmt.Errorf("hub_endpoint is required for remote installs")
		}
		if wg.HubPublicKey == "" {
			return fmt.Errorf("hub_public_key is required for remote installs")
		}
		if wg.HubAllowedIPs == "" {
			return fmt.Errorf("hub_allowed_ips is required for remote installs")
		}
	}

	return nil
}

func buildKMSConfig(cfg kmsConfig, secretsDir string) (kms.Config, error) {
	var c kms.Config
	c.Provider = cfg.Provider

	switch cfg.Provider {
	case "local":
		if cfg.Local == nil || cfg.Local.KeyFile == "" {
			c.Local.KeyFile = filepath.Join(secretsDir, "master.key")
		} else {
			c.Local.KeyFile = cfg.Local.KeyFile
		}
	case "vault":
		if cfg.Vault == nil || cfg.Vault.Addr == "" || cfg.Vault.KeyName == "" {
			return c, fmt.Errorf("vault requires addr and key_name")
		}
		c.Vault.Addr = cfg.Vault.Addr
		c.Vault.KeyName = cfg.Vault.KeyName
		c.Vault.TokenFile = cfg.Vault.TokenFile
	case "aws":
		if cfg.AWS == nil || cfg.AWS.Region == "" || cfg.AWS.KeyID == "" {
			return c, fmt.Errorf("aws requires region and key_id")
		}
		c.AWS.Region = cfg.AWS.Region
		c.AWS.KeyID = cfg.AWS.KeyID
		c.AWS.AccessKey = cfg.AWS.AccessKey
		c.AWS.SecretKey = cfg.AWS.SecretKey
	case "gcp":
		if cfg.GCP == nil || cfg.GCP.ProjectID == "" ||
			cfg.GCP.KeyRingID == "" || cfg.GCP.KeyID == "" {
			return c, fmt.Errorf("gcp requires project_id, key_ring_id, and key_id")
		}
		c.GCP.ProjectID = cfg.GCP.ProjectID
		c.GCP.LocationID = cfg.GCP.LocationID
		if c.GCP.LocationID == "" {
			c.GCP.LocationID = "global"
		}
		c.GCP.KeyRingID = cfg.GCP.KeyRingID
		c.GCP.KeyID = cfg.GCP.KeyID
		c.GCP.TokenFile = cfg.GCP.TokenFile
	case "azure":
		if cfg.Azure == nil || cfg.Azure.VaultURL == "" ||
			cfg.Azure.TenantID == "" || cfg.Azure.ClientID == "" {
			return c, fmt.Errorf("azure requires vault_url, tenant_id, and client_id")
		}
		c.Azure.VaultURL = cfg.Azure.VaultURL
		c.Azure.KeyName = cfg.Azure.KeyName
		if c.Azure.KeyName == "" {
			c.Azure.KeyName = "scutum"
		}
		c.Azure.TenantID = cfg.Azure.TenantID
		c.Azure.ClientID = cfg.Azure.ClientID
		c.Azure.TokenFile = cfg.Azure.TokenFile
	default:
		return c, fmt.Errorf("unknown provider %q — valid: local, vault, aws, gcp, azure", cfg.Provider)
	}
	return c, nil
}

func buildTOML(cfg kms.Config) string {
	var b strings.Builder
	fmt.Fprintf(&b, "provider = %q\n\n", cfg.Provider)
	switch cfg.Provider {
	case "local":
		fmt.Fprintf(&b, "[local]\nkey_file = %q\n", cfg.Local.KeyFile)
	case "vault":
		fmt.Fprintf(&b, "[vault]\naddr = %q\nkey_name = %q\ntoken_file = %q\n",
			cfg.Vault.Addr, cfg.Vault.KeyName, cfg.Vault.TokenFile)
	case "aws":
		fmt.Fprintf(&b, "[aws]\nregion = %q\nkey_id = %q\naccess_key = %q\nsecret_key = %q\n",
			cfg.AWS.Region, cfg.AWS.KeyID, cfg.AWS.AccessKey, cfg.AWS.SecretKey)
	case "gcp":
		fmt.Fprintf(&b, "[gcp]\nproject_id = %q\nlocation_id = %q\nkey_ring_id = %q\nkey_id = %q\ntoken_file = %q\n",
			cfg.GCP.ProjectID, cfg.GCP.LocationID, cfg.GCP.KeyRingID, cfg.GCP.KeyID, cfg.GCP.TokenFile)
	case "azure":
		fmt.Fprintf(&b, "[azure]\nvault_url = %q\nkey_name = %q\ntenant_id = %q\nclient_id = %q\ntoken_file = %q\n",
			cfg.Azure.VaultURL, cfg.Azure.KeyName, cfg.Azure.TenantID, cfg.Azure.ClientID, cfg.Azure.TokenFile)
	}
	return b.String()
}
