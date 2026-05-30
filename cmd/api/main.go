package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"embed"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	stdsync "sync"
	"syscall"
	"time"

	"golang.org/x/time/rate"

	scutumacme "scutum/cmd/internal/acme"
	"scutum/cmd/internal/auth"
	"scutum/cmd/internal/handlers"
	"scutum/cmd/internal/webhooks"
	"scutum/cmd/internal/kms"
	"scutum/cmd/internal/metrics"
	plugin "scutum/cmd/internal/plugins"
	"scutum/cmd/internal/store"
	"scutum/cmd/internal/sync"
	"scutum/cmd/internal/utils"
	"scutum/cmd/internal/wireguard"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	logger *utils.Logger
)

//go:embed openapi.yaml
var openAPISpec []byte

//go:embed all:dist
var frontendFS embed.FS

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	logger = initLogger()

	dataDir := getDataDir()
	secretsDir := getSecretsDir()

	os.MkdirAll(dataDir, 0755)
	os.MkdirAll(secretsDir, 0700)

	logger.Info("starting scutum", "version", "1.0.0")

	span := logger.Trace(ctx, "startup")
	defer span.End(nil)

	kmsProvider, err := initKMS(ctx, secretsDir)
	if err != nil {
		logger.Fatal("kms init failed", "error", err)
	}

	// --- Store ---
	var dbConfig interface{}
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL != "" {
		if strings.HasPrefix(dbURL, "postgres://") || strings.HasPrefix(dbURL, "postgresql://") {
			dbConfig = store.Config{
				Driver: store.DriverPostgres,
				DSN:    dbURL,
			}
			logger.Info("using postgres store", "url", dbURL)
		} else if strings.HasPrefix(dbURL, "mysql://") {
			dbConfig = store.Config{
				Driver: store.DriverMySQL,
				DSN:    strings.TrimPrefix(dbURL, "mysql://"),
			}
			logger.Info("using mysql store")
		} else {
			dbConfig = dbURL // Assume SQLite path if no prefix
			logger.Info("using sqlite store", "path", dbURL)
		}
	} else {
		dbPath := filepath.Join(dataDir, "scutum.db")
		dbConfig = dbPath
		logger.Info("using sqlite store", "path", dbPath)
	}

	db, err := store.New(ctx, dbConfig, kmsProvider)
	if err != nil {
		logger.Fatal("store init failed", "error", err)
	}
	logger.Info("store initialized")
	defer db.Close()

	// --- Seed ---
	if err := db.Seed(ctx); err != nil {
		logger.Fatal("seed failed", "error", err)
	}

	// --- JWT secret ---
	jwtSecret, err := loadOrGenerateJWTSecret(ctx, db, secretsDir)
	if err != nil {
		logger.Fatal("jwt secret failed", "error", err)
	}

	// --- Plugin Runtime ---
	pluginRuntime, err := plugin.NewRuntime(ctx)
	if err != nil {
		logger.Fatal("plugin runtime init failed", "error", err)
	}
	defer pluginRuntime.Close(ctx)

	// --- Sync: Healer for WireGuard health monitoring ---
	healerIntervalStr := os.Getenv("HEALER_INTERVAL")
	healerInterval := 30 * time.Second
	if healerIntervalStr != "" {
		if d, err := time.ParseDuration(healerIntervalStr); err == nil {
			healerInterval = d
		}
	}

	wgChecker := &sync.DefaultWGChecker{Runner: sync.ContextRunner{Fn: utils.DefaultCommandRunner.Output}}
	healer := sync.NewHealer(sync.HealerConfig{
		Interval: healerInterval,
	}, wgChecker)
	defer healer.Stop()
	healer.Start(ctx)

	// --- Hub HA: Leader Election + Restore WireGuard Interface ---
	const leaseTTL = 30 * time.Second
	holderID, _ := os.Hostname()
	if holderID == "" {
		holderID = fmt.Sprintf("scutum-%d", os.Getpid())
	}

	isLeader := false
	if complete, _ := db.IsSetupComplete(ctx); complete {
		acquired, err := db.AcquireHubLease(ctx, holderID, leaseTTL)
		if err != nil {
			logger.Error("hub lease acquisition error, continuing as follower", "error", err)
		} else if acquired {
			isLeader = true
			logger.Info("hub leader lease acquired", "holder_id", holderID)

			// Restore wg0 from database state.
			if configBytes, err := db.GetSecret(ctx, "wg0_config"); err == nil {
				var cfg utils.InterfaceConfig
				if err := json.Unmarshal(configBytes, &cfg); err == nil {
					if privKey, err := db.GetWireGuardPrivateKey(ctx, "wg0"); err == nil {
						cfg.PrivateKey = string(privKey)
						if _, err := utils.SetupInterface(cfg); err != nil {
							if !errors.Is(err, utils.ErrWireGuardUnavailable) {
								logger.Error("failed to restore wg0 interface on startup", "error", err)
							} else {
								logger.Warn("wireguard is unavailable on this host; interface not restored")
							}
						} else {
							logger.Info("restored wireguard interface wg0")
							restoreWGPeers(ctx, db, logger)
						}
					}
				}
			}

			// Renew the lease in background so it doesn't expire while we run.
			go func() {
				ticker := time.NewTicker(leaseTTL / 2)
				defer ticker.Stop()
				for {
					select {
					case <-ctx.Done():
						_ = db.ReleaseHubLease(context.Background(), holderID)
						return
					case <-ticker.C:
						if err := db.RenewHubLease(ctx, holderID, leaseTTL); err != nil {
							logger.Error("lost hub leader lease", "error", err)
						}
					}
				}
			}()
		} else {
			logger.Info("another hub instance holds the leader lease; running as follower (API-only)")
		}
	}
	_ = isLeader // used to gate wg-specific tasks if needed in the future

	// --- TLS Configuration ---
	certFile := os.Getenv("CERT_FILE")
	if certFile == "" {
		certFile = filepath.Join(secretsDir, "server.crt")
	}
	keyFile := os.Getenv("KEY_FILE")
	if keyFile == "" {
		keyFile = filepath.Join(secretsDir, "server.key")
	}

	useTLS := false
	var serverCert tls.Certificate
	var errCert error
	if _, err := os.Stat(certFile); err == nil {
		if _, err := os.Stat(keyFile); err == nil {
			useTLS = true
			serverCert, errCert = tls.LoadX509KeyPair(certFile, keyFile)
			if errCert != nil {
				logger.Fatal("failed to load server cert/key", "error", errCert)
			}
		}
	}

	tlsConfig := &tls.Config{
		MinVersion: tls.VersionTLS12,
	}
	clientTLSConfig := &tls.Config{
		MinVersion: tls.VersionTLS12,
	}
	if useTLS {
		clientTLSConfig.Certificates = []tls.Certificate{serverCert}
	}

	caCertFile := os.Getenv("CA_CERT_FILE")
	if caCertFile != "" {
		caCert, err := os.ReadFile(caCertFile)
		if err != nil {
			logger.Fatal("failed to read CA cert", "error", err)
		}
		caCertPool := x509.NewCertPool()
		if ok := caCertPool.AppendCertsFromPEM(caCert); !ok {
			logger.Fatal("failed to parse CA cert")
		}
		tlsConfig.ClientCAs = caCertPool
		tlsConfig.ClientAuth = tls.RequireAndVerifyClientCert
		clientTLSConfig.RootCAs = caCertPool
		logger.Info("mTLS enabled", "ca_file", caCertFile)
	}

	// meshTLSConfig is used for node-to-node HTTPS API calls (sync, proxy, endpoint
	// registration). When no CA cert is configured (self-signed certs), skip server
	// verification — security comes from WireGuard + HMAC signatures on each request.
	meshTLSConfig := &tls.Config{MinVersion: tls.VersionTLS12}
	if caCertFile != "" {
		meshTLSConfig.RootCAs = clientTLSConfig.RootCAs
	} else if useTLS {
		meshTLSConfig.InsecureSkipVerify = true //nolint:gosec
		logger.Info("no CA cert configured — internal API calls will skip TLS verification")
	}

	// --- Sync: Pusher for edge config distribution ---
	hmacKey, err := loadOrGenerateHMACKey(ctx, db, secretsDir)
	if err != nil {
		logger.Fatal("hmac key failed", "error", err)
	}
	handlers.SetProxyHMACKey(hmacKey)
	handlers.SetProxyTLSConfig(meshTLSConfig)
	// Remote nodes store the hub's HMAC key as "hub_hmac_key" during setup.
	// Use it for verifying hub-proxied requests instead of the locally generated key.
	if hubKey, err := db.GetSecret(ctx, "hub_hmac_key"); err == nil && len(hubKey) > 0 {
		auth.SetHubHMACKey(hubKey)
	} else {
		auth.SetHubHMACKey(hmacKey)
	}
	pusher := sync.NewPusher(sync.PushConfig{HMACKey: hmacKey})

	// Register existing nodes as edges in background (with retry).
	// For hub-type installs this also wires up FreshEndpoint lookups on healer peers.
	go func() {
		for attempt := 1; attempt <= 5; attempt++ {
			if err := registerEdges(ctx, db, pusher, healer, meshTLSConfig); err != nil {
				logger.Error("background edge registration failed, retrying",
					"attempt", attempt, "error", err)
				select {
				case <-ctx.Done():
					return
				case <-time.After(time.Duration(attempt*2) * time.Second):
				}
				continue
			}
			logger.Info("registered existing edges")
			return
		}
		logger.Error("edge registration failed after all attempts")
	}()

	// For edge (remote/combined) installs: push our current WireGuard endpoint to
	// the hub on startup so the hub's wg_peers table stays accurate even when our
	// public IP or NAT mapping has changed since initial setup.
	go registerOwnEndpoint(ctx, db, logger, meshTLSConfig)

	// Start rate-limiter cleanup to prevent memory growth
	go cleanupVisitors(ctx)

	// Background log cleanup (runs every 24h)
	go func() {
		retentionDays := 365 // Default to 1 year for CRA compliance / enterprise standard
		if envDays := os.Getenv("AUDIT_RETENTION_DAYS"); envDays != "" {
			if d, err := strconv.Atoi(envDays); err == nil && d > 0 {
				retentionDays = d
			}
		}

		ticker := time.NewTicker(24 * time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				olderThan := time.Now().Add(-time.Duration(retentionDays) * 24 * time.Hour)
				if err := db.DeleteOldLogs(ctx, olderThan); err != nil {
					logger.Error("failed to delete old logs", "error", err)
				} else {
					logger.Info("cleaned up old audit/system logs", "older_than", olderThan, "retention_days", retentionDays)
				}
			}
		}
	}()

	// --- Router ---
	apiMux := http.NewServeMux()

	registrar := plugin.NewRouteRegistrar(apiMux, pluginRuntime)
	ctx = context.WithValue(ctx, plugin.RegistrarKey{}, registrar)

	// FIX: Pass registrar here
	if err := loadPlugins(ctx, pluginRuntime, db, registrar); err != nil {
		logger.Fatal("load plugins failed", "error", err)
	}

	// Pass logger to handlers that need it
	handlers.SetLogger(logger)

	// --- Auth middleware ---
	authMW := auth.Middleware(db, jwtSecret)
	require := func(resource, action string, h http.HandlerFunc) http.Handler {
		// Ensure that the authentication middleware runs before the permission check.
		// This populates the user context needed by auth.Require.
		return authMW(auth.Require(db, resource, action)(http.HandlerFunc(h)))
	}

	// requireDocker wraps require() and additionally checks that the Docker
	// socket is present before the handler runs. Without this, missing-socket
	// errors surface as confusing 500s instead of a clear feature-unavailable
	// message — most relevant when running in Kubernetes without docker.enabled.
	requireDocker := func(resource, action string, h http.HandlerFunc) http.Handler {
		return require(resource, action, func(w http.ResponseWriter, r *http.Request) {
			if !utils.IsDockerAvailable() {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusServiceUnavailable)
				json.NewEncoder(w).Encode(map[string]string{
					"error": "Docker is not available on this node (socket not mounted)",
				})
				return
			}
			h(w, r)
		})
	}

	// --- Handlers ---
	dockerCtrl := handlers.NewDockerHandler(db)
	kubernetesCtrl := handlers.NewKubernetesHandler(db)
	gitCtrl := handlers.NewGitHandler()
	s3Ctrl := handlers.NewS3Handler()
	storageCtrl := handlers.NewStorageHandler(db)
	wgService := &wireguard.CLIService{}
	wgCtrl := handlers.NewWireGuardHandler("wg0", wgService, healer, db, db)
	wgCtrl.SecretsDir = secretsDir
	wgCtrl.SetPeerStore(db)
	wgCtrl.SetEndpointStore(db)
	pluginCtrl := handlers.NewPluginHandler(pluginRuntime, registrar)
	authCtrl := handlers.NewAuthHandler(db, jwtSecret)
	nodeCtrl := handlers.NewNodeHandler(db)
	userCtrl := handlers.NewUserHandler(db)
	roleCtrl := handlers.NewRoleHandler(db)
	obsCtrl := handlers.NewObservabilityHandler(db, db)
	otelCtrl := handlers.NewOTelHandler(db, db)
	exportCtrl := handlers.NewExportHandler(db)
	operatorCtrl := handlers.NewOperatorHandler(db)
	webhookCtrl := handlers.NewWebhookHandler(db)
	scimCtrl := handlers.NewSCIMHandler(db)
	auditFwdCtrl := handlers.NewAuditForwarderHandler(db)
	dispatcher := webhooks.NewDispatcher(db)
	dispatcher.Start(ctx)
	go handlers.RunForwarder(ctx, db)
	utils.SetObsSink(db)
	setupCtrl := handlers.NewSetupHandler(db, filepath.Join(secretsDir, "kms.toml"), func(newProvider kms.Provider) {
		// All secrets written during setup (wg0_config, sync_hmac_key, hub_hmac_key,
		// etc.) were sealed with the pre-setup KMS. Read what we need before swapping,
		// swap the provider, then re-wrap every DEK so subsequent reads use the new key.
		hubKey, hubKeyErr := db.GetSecret(ctx, "hub_hmac_key")
		oldProvider := db.CurrentKMS()
		db.SwapKMS(newProvider)
		if err := db.RotateKeys(ctx, oldProvider); err != nil {
			logger.Warn("key rotation during setup failed — secrets may be unreadable after restart", "error", err)
		}
		logger.Info("kms provider switched", "provider", newProvider.Name())
		if hubKeyErr == nil && len(hubKey) > 0 {
			auth.SetHubHMACKey(hubKey)
			logger.Info("hub HMAC key activated")
		}
	})

	// Auth (public)
	apiMux.Handle("POST /auth/register", rateLimitMW(http.HandlerFunc(authCtrl.HandleRegister)))
	apiMux.Handle("POST /auth/login", rateLimitMW(http.HandlerFunc(authCtrl.HandleLogin)))
	apiMux.Handle("POST /auth/keys", rateLimitMW(http.HandlerFunc(authCtrl.HandleCreateAPIKey)))
	apiMux.Handle("POST /auth/forgot-password", rateLimitMW(http.HandlerFunc(authCtrl.HandleForgotPassword)))

	// Auth (authenticated)
	apiMux.Handle("GET /auth/me", authMW(http.HandlerFunc(userCtrl.HandleMe)))
	apiMux.Handle("GET /auth/tokens", authMW(http.HandlerFunc(userCtrl.HandleListTokens)))
	apiMux.Handle("DELETE /auth/tokens/{id}", authMW(http.HandlerFunc(userCtrl.HandleDeleteToken)))

	// Recovery codes (authenticated)
	apiMux.Handle("GET /auth/recovery-codes", authMW(http.HandlerFunc(authCtrl.HandleRecoveryCodeStatus)))
	apiMux.Handle("POST /auth/recovery-codes/regenerate", authMW(http.HandlerFunc(authCtrl.HandleRegenerateRecoveryCodes)))

	// MFA / TOTP
	apiMux.Handle("GET /auth/mfa/status", authMW(http.HandlerFunc(authCtrl.HandleMFAStatus)))
	apiMux.Handle("POST /auth/mfa/setup", authMW(http.HandlerFunc(authCtrl.HandleMFASetup)))
	apiMux.Handle("POST /auth/mfa/enable", authMW(http.HandlerFunc(authCtrl.HandleMFAEnable)))
	apiMux.Handle("POST /auth/mfa/disable", authMW(http.HandlerFunc(authCtrl.HandleMFADisable)))

	// SSO (public)
	ssoCtrl := handlers.NewSSOHandler(db, jwtSecret)
	apiMux.Handle("GET /auth/sso/providers", http.HandlerFunc(ssoCtrl.HandleProviders))
	apiMux.Handle("GET /auth/sso/{provider}", http.HandlerFunc(ssoCtrl.HandleLogin))
	apiMux.Handle("GET /auth/sso/{provider}/callback", http.HandlerFunc(ssoCtrl.HandleCallback))

	// Health + version (public)
	apiMux.HandleFunc("GET /health", handlers.HealthHandler)
	apiMux.HandleFunc("GET /version", handlers.VersionHandler)
	apiMux.Handle("GET /metrics", promhttp.Handler())

	// Setup (public — no auth before setup is complete)
	apiMux.HandleFunc("GET /setup/status", setupCtrl.HandleStatus)
	apiMux.HandleFunc("POST /setup", setupCtrl.HandleSetup)
	apiMux.Handle("POST /setup/test-kms", require("admin", "admin", setupCtrl.HandleTestKMS))

	// Nodes
	apiMux.Handle("GET /nodes", require("nodes", "read", nodeCtrl.HandleList))
	apiMux.Handle("GET /nodes/{id}", require("nodes", "read", nodeCtrl.HandleGet))
	apiMux.Handle("POST /nodes", require("nodes", "write", nodeCtrl.HandleCreate))
	apiMux.Handle("DELETE /nodes/{id}", require("nodes", "admin", nodeCtrl.HandleDelete))

	// Users (admin)
	apiMux.Handle("GET /users", require("admin", "admin", userCtrl.HandleList))
	apiMux.Handle("GET /users/{id}", require("admin", "admin", userCtrl.HandleGet))
	apiMux.Handle("POST /users", require("admin", "admin", userCtrl.HandleCreate))
	apiMux.Handle("PUT /users/{id}", require("admin", "admin", userCtrl.HandleUpdate))
	apiMux.Handle("DELETE /users/{id}", require("admin", "admin", userCtrl.HandleDelete))

	// Roles (admin)
	apiMux.Handle("GET /roles", require("admin", "admin", roleCtrl.HandleList))
	apiMux.Handle("POST /roles", require("admin", "admin", roleCtrl.HandleCreate))
	apiMux.Handle("PUT /roles/{id}", require("admin", "admin", roleCtrl.HandleUpdate))
	apiMux.Handle("DELETE /roles/{id}", require("admin", "admin", roleCtrl.HandleDelete))

	// Docker
	apiMux.Handle("GET /docker/containers", requireDocker("docker", "read", dockerCtrl.HandleListContainers))
	apiMux.Handle("POST /docker/deploy", requireDocker("docker", "write", dockerCtrl.PostDeploy))
	apiMux.Handle("POST /docker/deploy-compose", requireDocker("docker", "write", dockerCtrl.HandleDeployCompose))
	apiMux.Handle("GET /docker/containers/{id}", requireDocker("docker", "read", dockerCtrl.HandleInspect))
	apiMux.Handle("GET /docker/containers/{id}/logs-json", requireDocker("docker", "read", dockerCtrl.HandleLogsJSON))
	apiMux.Handle("POST /docker/containers/{id}/start", requireDocker("docker", "write", dockerCtrl.HandleStart))
	apiMux.Handle("POST /docker/containers/{id}/stop", requireDocker("docker", "write", dockerCtrl.HandleStop))
	apiMux.Handle("POST /docker/containers/{id}/restart", requireDocker("docker", "write", dockerCtrl.HandleRestart))
	apiMux.Handle("DELETE /docker/containers/{id}", requireDocker("docker", "delete", dockerCtrl.HandleDelete))
	apiMux.Handle("GET /docker/containers/{id}/stats", requireDocker("docker", "read", dockerCtrl.HandleStats))
	apiMux.Handle("GET /docker/containers/{id}/stats-snapshot", requireDocker("docker", "read", dockerCtrl.HandleStatsSnapshot))
	apiMux.Handle("GET /docker/containers/{id}/logs", requireDocker("docker", "read", dockerCtrl.HandleLogs))
	apiMux.Handle("GET /docker/containers/{id}/terminal", requireDocker("docker", "write", dockerCtrl.HandleTerminal))

	// Kubernetes
	apiMux.Handle("GET /kubernetes/summary", require("kubernetes", "read", kubernetesCtrl.HandleK8sSummary))
	apiMux.Handle("GET /kubernetes/pods", require("kubernetes", "read", kubernetesCtrl.HandleListAllPods))
	apiMux.Handle("GET /kubernetes/events", require("kubernetes", "read", kubernetesCtrl.HandleWatchEvents))
	apiMux.Handle("GET /kubernetes/{ns}/pods/{name}", require("kubernetes", "read", kubernetesCtrl.HandleGetPod))
	apiMux.Handle("GET /kubernetes/{ns}/pods/{name}/logs-json", require("kubernetes", "read", kubernetesCtrl.HandlePodLogsJSON))
	apiMux.Handle("DELETE /kubernetes/{ns}/pods/{name}", require("kubernetes", "delete", kubernetesCtrl.HandleDeletePod))
	apiMux.Handle("POST /kubernetes/apply", require("kubernetes", "write", kubernetesCtrl.HandleApplyYAML))
	apiMux.Handle("POST /kubernetes/{ns}/deploy", require("kubernetes", "write", kubernetesCtrl.HandleDeploy))
	apiMux.Handle("POST /kubernetes/{ns}/deployments/{name}/scale", require("kubernetes", "write", kubernetesCtrl.HandleScale))
	apiMux.Handle("POST /kubernetes/{ns}/deployments/{name}/restart", require("kubernetes", "write", kubernetesCtrl.HandleRestart))
	apiMux.Handle("GET /k8s/{namespace}/{pod}/terminal", require("kubernetes", "write", kubernetesCtrl.HandleTerminal))

	// Git
	apiMux.Handle("POST /git/sync", require("git", "write", gitCtrl.HandleGitSync))

	// S3 / Storage
	apiMux.Handle("POST /storage/s3/upload", require("storage", "write", s3Ctrl.HandleUpload))
	apiMux.Handle("POST /storage/s3/download", require("storage", "read", s3Ctrl.HandleDownload))
	apiMux.Handle("POST /storage/s3/list", require("storage", "read", s3Ctrl.HandleList))
	apiMux.Handle("DELETE /storage/s3/delete", require("storage", "admin", s3Ctrl.HandleDelete))

	// Storage backends (registered credentials)
	apiMux.Handle("GET /storage/backends", require("storage", "read", storageCtrl.HandleListBackends))
	apiMux.Handle("POST /storage/backends", require("storage", "write", storageCtrl.HandleCreateBackend))
	apiMux.Handle("DELETE /storage/backends/{id}", require("storage", "admin", storageCtrl.HandleDeleteBackend))
	apiMux.Handle("POST /storage/backends/{id}/test", require("storage", "read", storageCtrl.HandleTestBackend))
	apiMux.Handle("GET /storage/backends/{id}/buckets", require("storage", "read", storageCtrl.HandleListBuckets))

	// WireGuard
	apiMux.Handle("POST /network/peer", require("wireguard", "write", wgCtrl.HandleAddPeer))
	apiMux.Handle("POST /network/register-endpoint", require("wireguard", "write", wgCtrl.HandleRegisterEndpoint))
	apiMux.Handle("GET /network/status", require("wireguard", "read", wgCtrl.HandleGetStatus))
	apiMux.Handle("GET /network/mesh-summary", require("wireguard", "read", wgCtrl.HandleMeshSummary))
	apiMux.Handle("GET /network/peers", require("wireguard", "read", wgCtrl.HandlePeerStatus))
	apiMux.Handle("GET /network/hub-key", require("admin", "admin", wgCtrl.HandleGetHubKey))

	// Plugin management
	apiMux.Handle("POST /plugins/load", require("plugins", "admin", pluginCtrl.HandleLoad))
	apiMux.Handle("POST /plugins/upload", require("plugins", "admin", pluginCtrl.HandleUpload))
	apiMux.Handle("DELETE /plugins/{name}", require("plugins", "admin", pluginCtrl.HandleUnload))
	apiMux.Handle("GET /plugins", require("plugins", "read", pluginCtrl.HandleList))

	// Observability + Audit
	apiMux.Handle("GET /observability/logs", require("admin", "read", obsCtrl.HandleLogs))
	apiMux.Handle("GET /observability/traces", require("admin", "read", obsCtrl.HandleTraces))
	apiMux.Handle("GET /observability/metrics", require("admin", "read", otelCtrl.HandleListMetrics))
	apiMux.Handle("GET /audit/logs", require("admin", "admin", obsCtrl.HandleAuditLogs))
	apiMux.Handle("GET /audit/logs/export", require("admin", "admin", obsCtrl.HandleExportAuditLogs))
	apiMux.Handle("GET /admin/export", require("admin", "admin", exportCtrl.HandleExport))

	// OTLP receivers — authenticated so only enrolled services can push telemetry
	apiMux.Handle("POST /otlp/v1/traces", require("admin", "read", otelCtrl.HandleOTLPTraces))
	apiMux.Handle("POST /otlp/v1/logs", require("admin", "read", otelCtrl.HandleOTLPLogs))
	apiMux.Handle("POST /otlp/v1/metrics", require("admin", "read", otelCtrl.HandleOTLPMetrics))

	// Container / pod telemetry scraping
	apiMux.Handle("GET /docker/containers/{id}/traces", requireDocker("docker", "read", dockerCtrl.HandleContainerTraces))
	apiMux.Handle("GET /docker/containers/{id}/metrics-scrape", requireDocker("docker", "read", dockerCtrl.HandleContainerMetricsScrape))
	apiMux.Handle("GET /kubernetes/{ns}/pods/{name}/traces", require("kubernetes", "read", kubernetesCtrl.HandlePodTraces))
	apiMux.Handle("GET /kubernetes/{ns}/pods/{name}/metrics-scrape", require("kubernetes", "read", kubernetesCtrl.HandlePodMetricsScrape))

	// Sync (push config to edges)
	syncCtrl := handlers.NewSyncHandler(db, pusher, clientTLSConfig)
	apiMux.Handle("POST /sync/push", require("sync", "write", syncCtrl.HandlePush))
	apiMux.Handle("POST /sync/register-edge", require("sync", "admin", syncCtrl.HandleRegisterEdge))

	// Operator bootstrap (admin only — used by the Kubernetes operator)
	apiMux.Handle("GET /operator/bootstrap", require("admin", "admin", operatorCtrl.HandleBootstrap))

	// Webhooks (admin only)
	apiMux.Handle("GET /webhooks", require("admin", "admin", webhookCtrl.HandleList))
	apiMux.Handle("POST /webhooks", require("admin", "admin", webhookCtrl.HandleCreate))
	apiMux.Handle("GET /webhooks/{id}", require("admin", "admin", webhookCtrl.HandleGet))
	apiMux.Handle("PUT /webhooks/{id}", require("admin", "admin", webhookCtrl.HandleUpdate))
	apiMux.Handle("DELETE /webhooks/{id}", require("admin", "admin", webhookCtrl.HandleDelete))
	apiMux.Handle("POST /webhooks/{id}/test", require("admin", "admin", webhookCtrl.HandleTest))

	// Audit log forwarders (admin only)
	apiMux.Handle("GET /audit/forwarders", require("admin", "admin", auditFwdCtrl.HandleList))
	apiMux.Handle("POST /audit/forwarders", require("admin", "admin", auditFwdCtrl.HandleCreate))
	apiMux.Handle("GET /audit/forwarders/{id}", require("admin", "admin", auditFwdCtrl.HandleGet))
	apiMux.Handle("PUT /audit/forwarders/{id}", require("admin", "admin", auditFwdCtrl.HandleUpdate))
	apiMux.Handle("DELETE /audit/forwarders/{id}", require("admin", "admin", auditFwdCtrl.HandleDelete))

	// SCIM token management (admin only, regular JWT auth)
	apiMux.Handle("GET /scim/tokens", require("admin", "admin", scimCtrl.HandleListTokens))
	apiMux.Handle("POST /scim/tokens", require("admin", "admin", scimCtrl.HandleCreateToken))
	apiMux.Handle("DELETE /scim/tokens/{id}", require("admin", "admin", scimCtrl.HandleDeleteToken))

	// Recovery (emergency key recovery)
	recoveryCtrl := handlers.NewRecoveryHandler(db, kmsProvider)
	apiMux.Handle("POST /recovery/generate-shares", require("admin", "admin", recoveryCtrl.HandleGenerateShares))
	apiMux.Handle("POST /recovery/recover", require("admin", "admin", recoveryCtrl.HandleRecover))
	apiMux.Handle("POST /recovery/reissue-shares", require("admin", "admin", recoveryCtrl.HandleReissueShares))

	// System info (public)
	systemCtrl := handlers.NewSystemHandler()
	apiMux.Handle("GET /system/tls-mode", http.HandlerFunc(systemCtrl.HandleTLSMode))

	// API docs (public)
	docsCtrl := handlers.NewDocsHandler(openAPISpec)
	apiMux.HandleFunc("GET /openapi.yaml", docsCtrl.HandleSpec)
	apiMux.HandleFunc("GET /docs", docsCtrl.HandleDocs)

	// --- Final Mux with Prefixing and Static Assets ---
	mainMux := http.NewServeMux()
	// Auth is scoped to /api/ only; frontend assets are served without authentication
	// so the browser can load the login page before any credentials exist.
	// We remove authMW from the global wrapper so that apiMux can manage its own public/private routes.
	mainMux.Handle("/api/", http.StripPrefix("/api", metricsMiddleware(tracingMiddleware(logger, apiMux))))

	// SCIM 2.0 — mounted at /scim/v2/ with its own token auth
	scimMux := http.NewServeMux()
	scimMux.HandleFunc("GET /ServiceProviderConfig", scimCtrl.HandleServiceProviderConfig)
	scimMux.Handle("GET /Users", scimCtrl.AuthMiddleware(http.HandlerFunc(scimCtrl.HandleListUsers)))
	scimMux.Handle("POST /Users", scimCtrl.AuthMiddleware(http.HandlerFunc(scimCtrl.HandleCreateUser)))
	scimMux.Handle("GET /Users/{id}", scimCtrl.AuthMiddleware(http.HandlerFunc(scimCtrl.HandleGetUser)))
	scimMux.Handle("PUT /Users/{id}", scimCtrl.AuthMiddleware(http.HandlerFunc(scimCtrl.HandleReplaceUser)))
	scimMux.Handle("PATCH /Users/{id}", scimCtrl.AuthMiddleware(http.HandlerFunc(scimCtrl.HandlePatchUser)))
	scimMux.Handle("DELETE /Users/{id}", scimCtrl.AuthMiddleware(http.HandlerFunc(scimCtrl.HandleDeleteUser)))
	mainMux.Handle("/scim/v2/", http.StripPrefix("/scim/v2", scimMux))

	// Nuxt generates hashed filenames under /_nuxt/ so they can be cached forever.
	sub, _ := fs.Sub(frontendFS, "dist")
	nuxtFileServer := http.FileServer(http.FS(sub))
	staticHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/_nuxt/") {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		}
		nuxtFileServer.ServeHTTP(w, r)
	})

	mainMux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		urlPath := r.URL.Path

		// For paths without a trailing slash, check if a directory index exists
		// and serve it directly — avoids the file server's 301 redirect which
		// causes an infinite loop with Nuxt's client-side router normalisation.
		if !strings.HasSuffix(urlPath, "/") {
			if data, err := frontendFS.ReadFile("dist" + urlPath + "/index.html"); err == nil {
				w.Header().Set("Content-Type", "text/html; charset=utf-8")
				w.Write(data)
				return
			}
		}

		// Serve exact file matches (JS, CSS, images, etc.).
		if _, err := frontendFS.Open("dist" + urlPath); err == nil {
			staticHandler.ServeHTTP(w, r)
			return
		}

		// Unknown paths fall back to index.html so client-side routing works.
		index, err := frontendFS.ReadFile("dist/index.html")
		if err != nil {
			http.Error(w, "Not Found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(index)
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	if !strings.HasPrefix(port, ":") {
		port = ":" + port
	}
	setupCtrl.APIPort = port

	server := &http.Server{
		Addr:         port,
		Handler:      mainMux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
		TLSConfig:    tlsConfig,
	}

	acmeCfg := scutumacme.FromEnv(secretsDir)
	if acmeCfg.Enabled() {
		acmeMgr := scutumacme.New(acmeCfg)
		server.TLSConfig = acmeMgr.TLSConfig()
		go func() {
			logger.Info("ACME HTTP-01 challenge server starting", "addr", ":80")
			if err := http.ListenAndServe(":80", acmeMgr.ChallengeHandler(acmeCfg.Domain)); !errors.Is(err, http.ErrServerClosed) { //nolint:gosec
				logger.Error("acme http server failed", "error", err)
			}
		}()
		logger.Info("scutum API starting (HTTPS/ACME)", "addr", port, "domain", acmeCfg.Domain)
		go func() {
			ln, err := tls.Listen("tcp", port, acmeMgr.TLSConfig())
			if err != nil {
				logger.Fatal("acme tls listener failed", "error", err)
			}
			if err := server.Serve(ln); !errors.Is(err, http.ErrServerClosed) {
				logger.Fatal("https server failed", "error", err)
			}
		}()
	} else if useTLS {
		logger.Info("scutum API starting (HTTPS)", "addr", port, "cert", certFile)
		go func() {
			if err := server.ListenAndServeTLS(certFile, keyFile); !errors.Is(err, http.ErrServerClosed) {
				logger.Fatal("https server failed", "error", err)
			}
		}()
	} else {
		logger.Info("scutum API starting (HTTP)", "addr", port)
		go func() {
			if err := server.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
				logger.Fatal("http server failed", "error", err)
			}
		}()
	}

	// Keep main alive until context is cancelled
	<-ctx.Done()
	logger.Info("shutting down gracefully...")

	// Release the hub lease immediately on SIGTERM — before server.Shutdown —
	// so a restarted instance (with a new container hostname) can steal it right
	// away. Doing this after server.Shutdown risks Docker's stop timeout killing
	// the process before we get here.
	_ = db.ReleaseHubLease(context.Background(), holderID)

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error("server shutdown error", "error", err)
	}
	healer.Stop()
	pusher.Stop()
	dispatcher.Stop()
	if err := db.Close(); err != nil {
		logger.Error("db close error", "error", err)
	}
	logger.Info("shutdown complete")
}

func initLogger() *utils.Logger {
	level := slog.LevelInfo
	if os.Getenv("LOG_LEVEL") == "debug" {
		level = slog.LevelDebug
	}
	auditEnabled := os.Getenv("AUDIT_ENABLED") == "true"
	return utils.InitLogger(level, auditEnabled)
}

func getDataDir() string {
	if dir := os.Getenv("DATA_DIR"); dir != "" {
		return dir
	}
	return "../data"
}

func getSecretsDir() string {
	if dir := os.Getenv("SECRETS_DIR"); dir != "" {
		return dir
	}
	return "../secrets"
}

func initKMS(ctx context.Context, secretsDir string) (kms.Provider, error) {
	kmsConfigPath := filepath.Join(secretsDir, "kms.toml")

	cfg, err := kms.LoadConfig(kmsConfigPath)
	if err != nil {
		if os.IsNotExist(err) {
			// If config doesn't exist, create local KMS with auto-generated key
			keyFile := filepath.Join(secretsDir, "master.key")
			provider, err := kms.NewLocalKeyProvider(keyFile)
			if err != nil {
				return nil, fmt.Errorf("create default kms: %w", err)
			}
			logger.Info("no KMS config found; using default local KMS", "key_file", keyFile)
			return provider, nil
		}
		return nil, fmt.Errorf("load kms config: %w", err)
	}

	provider, err := kms.FromConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("init kms from config: %w", err)
	}

	return provider, nil
}

func loadOrGenerateJWTSecret(ctx context.Context, db *store.Store, secretsDir string) ([]byte, error) {
	secret, err := db.GetSecret(ctx, "jwt_secret")
	if err == nil {
		return secret, nil
	}

	// Generate new secret
	secretPath := filepath.Join(secretsDir, "jwt.key")
	secretData, err := os.ReadFile(secretPath)
	if err == nil {
		return secretData, nil
	}

	// Generate and save
	raw := make([]byte, 48)
	if _, err := rand.Read(raw); err != nil {
		return nil, fmt.Errorf("generate jwt secret: %w", err)
	}

	if err := os.WriteFile(secretPath, raw, 0600); err != nil {
		return nil, fmt.Errorf("store jwt secret to file: %w", err)
	}

	// Also store in DB
	if err := db.SetSecret(ctx, "jwt_secret", raw); err != nil {
		logger.Warn("failed to store jwt secret in DB", "error", err)
	}

	logger.Info("generated and stored new JWT secret")
	return raw, nil
}

func loadPlugins(ctx context.Context, rt *plugin.Runtime, db *store.Store, rr *plugin.RouteRegistrar) error {
	ps, err := db.ListEnabledPlugins(ctx)
	if err != nil {
		return err
	}
	for _, p := range ps {
		// FIX: Pass the registrar to the Load method
		if err := rt.Load(ctx, p.Name, p.Path, rr); err != nil {
			logger.Warn("failed to load plugin", "name", p.Name, "error", err)
		} else {
			logger.Info("plugin loaded", "name", p.Name)
		}
	}
	return nil
}

func loadOrGenerateHMACKey(ctx context.Context, db *store.Store, secretsDir string) ([]byte, error) {
	secret, err := db.GetSecret(ctx, "sync_hmac_key")
	if err == nil {
		return secret, nil
	}

	key, err := sync.NewHMACKey()
	if err != nil {
		return nil, fmt.Errorf("generate hmac key: %w", err)
	}

	secretPath := filepath.Join(secretsDir, "sync_hmac.key")
	if err := os.WriteFile(secretPath, key, 0600); err != nil {
		return nil, fmt.Errorf("store hmac key to file: %w", err)
	}

	if err := db.SetSecret(ctx, "sync_hmac_key", key); err != nil {
		logger.Warn("failed to store sync hmac key in DB", "error", err)
	}

	logger.Info("generated and stored new sync HMAC key")
	return key, nil
}

func registerEdges(ctx context.Context, db *store.Store, pusher *sync.Pusher, healer *sync.Healer, clientTLSConfig *tls.Config) error {
	peers, err := db.ListWGPeers(ctx)
	if err != nil {
		return fmt.Errorf("list peers: %w", err)
	}

	nodes, err := db.ListNodes(ctx)
	if err != nil {
		return fmt.Errorf("list nodes: %w", err)
	}

	for _, node := range nodes {
		if node.Type != "remote" && node.Type != "combined" {
			continue
		}
		apiBase := nodeAPIBase(node.Address)
		if apiBase == "" {
			continue
		}
		token, _ := db.GetSecret(ctx, "edge_token_"+node.ID)
		sink := sync.NewHTTPEdgeSink(node.ID, apiBase+"/sync", string(token), clientTLSConfig)
		pusher.Register(sink)
		logger.Info("registered edge", "node_id", node.ID)

		// Register with healer, wiring FreshEndpoint so the healer re-adds the
		// peer using the latest DB endpoint if the edge registers a new one.
		for _, p := range peers {
			if p.NodeID == node.ID {
				nodeID := node.ID // capture for closure
				healer.AddPeer(sync.WGPeer{
					IfaceName:  "wg0",
					PublicKey:  node.PublicKey,
					Endpoint:   p.Endpoint,
					AllowedIPs: p.AllowedIPs,
					FreshEndpoint: func(ctx context.Context) (string, error) {
						fresh, err := db.GetWGPeer(ctx, nodeID)
						if err != nil {
							return "", err
						}
						return fresh.Endpoint, nil
					},
				})
				break
			}
		}
	}
	return nil
}

// endpointRefreshInterval controls how often an edge node re-registers its
// WireGuard endpoint with the hub. Keeping this short means the hub picks up
// a new NAT mapping within one interval when the node changes networks (e.g.
// laptop roaming between WiFi and mobile hotspot).
const endpointRefreshInterval = 2 * time.Minute

// registerOwnEndpoint runs on edge (remote/combined) nodes. It pushes the
// node's current WireGuard listen port to the hub; the hub derives the full
// endpoint as "observed-source-IP:listen_port" and stores it in wg_peers.
// After the initial registration it loops, re-registering every
// endpointRefreshInterval so the hub always has the current NAT mapping.
func registerOwnEndpoint(ctx context.Context, db *store.Store, logger *utils.Logger, tlsConfig *tls.Config) {
	installType, err := db.GetInstallType(ctx)
	if err != nil || (installType != store.InstallRemote && installType != store.InstallCombined) {
		return
	}

	// Find hub node address.
	nodes, err := db.ListNodes(ctx)
	if err != nil {
		logger.Warn("registerOwnEndpoint: cannot list nodes", "error", err)
		return
	}
	var hubAddr string
	for _, n := range nodes {
		if n.Type == "hub" {
			hubAddr = n.Address
			break
		}
	}
	if hubAddr == "" {
		logger.Warn("registerOwnEndpoint: no hub node in DB, skipping")
		return
	}
	hubAPIBase := nodeAPIBase(hubAddr)

	// Our WireGuard public key.
	pubKeyBytes, err := utils.DefaultCommandRunner.Output("wg", "show", "wg0", "public-key")
	if err != nil {
		logger.Warn("registerOwnEndpoint: cannot read wg public key", "error", err)
		return
	}
	pubKey := strings.TrimSpace(string(pubKeyBytes))
	if pubKey == "" {
		logger.Warn("registerOwnEndpoint: empty wg public key, skipping")
		return
	}

	// Our WireGuard listen port from stored interface config.
	listenPort := 51820
	if cfgBytes, err := db.GetSecret(ctx, "wg0_config"); err == nil {
		var cfg utils.InterfaceConfig
		if err := json.Unmarshal(cfgBytes, &cfg); err == nil && cfg.Port > 0 {
			listenPort = cfg.Port
		}
	}

	// Hub HMAC key for signing — stored during setup as "hub_hmac_key".
	hmacKey, err := db.GetSecret(ctx, "hub_hmac_key")
	if err != nil || len(hmacKey) == 0 {
		logger.Warn("registerOwnEndpoint: hub_hmac_key not set, skipping endpoint registration")
		return
	}

	// Initial registration with retry backoff.
	for attempt := 1; attempt <= 5; attempt++ {
		if err := callRegisterEndpoint(ctx, hubAPIBase, pubKey, listenPort, hmacKey, tlsConfig); err != nil {
			logger.Warn("registerOwnEndpoint: attempt failed, retrying",
				"attempt", attempt, "error", err)
			select {
			case <-ctx.Done():
				return
			case <-time.After(time.Duration(attempt*3) * time.Second):
			}
			continue
		}
		logger.Info("registerOwnEndpoint: endpoint registered with hub",
			"hub", hubAPIBase, "listen_port", listenPort)
		break
	}

	// Periodic re-registration: if the node changes networks (different NAT
	// exit IP) the hub learns the new mapping on the next tick rather than
	// waiting for a restart.
	ticker := time.NewTicker(endpointRefreshInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := callRegisterEndpoint(ctx, hubAPIBase, pubKey, listenPort, hmacKey, tlsConfig); err != nil {
				logger.Warn("registerOwnEndpoint: periodic refresh failed", "error", err)
			} else {
				logger.Debug("registerOwnEndpoint: endpoint refreshed", "hub", hubAPIBase)
			}
		}
	}
}

// callRegisterEndpoint sends a signed POST to the hub's /api/network/register-endpoint.
// It uses the same HMAC signing format as hub-proxied requests so the hub's existing
// auth middleware accepts it without additional setup.
func callRegisterEndpoint(ctx context.Context, hubAPIBase, pubKey string, listenPort int, hmacKey []byte, tlsConfig *tls.Config) error {
	body, _ := json.Marshal(map[string]interface{}{
		"public_key":  pubKey,
		"listen_port": listenPort,
	})

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		hubAPIBase+"/api/network/register-endpoint", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}

	ts := strconv.FormatInt(time.Now().Unix(), 10)
	mac := hmac.New(sha256.New, hmacKey)
	fmt.Fprintf(mac, "%s\n%s\n%s\n", ts, req.Method, "/api/network/register-endpoint")
	sig := hex.EncodeToString(mac.Sum(nil))

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Scutum-Hub-Sig", sig)
	req.Header.Set("X-Scutum-Hub-Ts", ts)

	resp, err := (&http.Client{
		Timeout:   10 * time.Second,
		Transport: &http.Transport{TLSClientConfig: tlsConfig},
	}).Do(req)
	if err != nil {
		return fmt.Errorf("http: %w", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("hub returned HTTP %d", resp.StatusCode)
	}
	return nil
}

// restoreWGPeers re-adds all WireGuard peers from the database after the
// interface is brought back up on startup. Without this, the tunnel has no
// peers after a container restart.
func restoreWGPeers(ctx context.Context, db *store.Store, logger *utils.Logger) {
	nodes, err := db.ListNodes(ctx)
	if err != nil {
		logger.Warn("restoreWGPeers: failed to list nodes", "error", err)
		return
	}
	peers, err := db.ListWGPeers(ctx)
	if err != nil {
		logger.Warn("restoreWGPeers: failed to list peers", "error", err)
		return
	}
	keyByNodeID := make(map[string]string, len(nodes))
	for _, n := range nodes {
		keyByNodeID[n.ID] = n.PublicKey
	}
	for _, p := range peers {
		pubKey, ok := keyByNodeID[p.NodeID]
		if !ok || pubKey == "" {
			continue
		}
		if err := utils.AddPeer("wg0", pubKey, p.Endpoint, p.AllowedIPs, 25); err != nil {
			logger.Warn("restoreWGPeers: failed to re-add peer", "node_id", p.NodeID, "error", err)
		} else {
			logger.Info("restoreWGPeers: re-added peer", "node_id", p.NodeID)
		}
	}
}

// nodeAPIBase converts a node's stored address (which may be a WireGuard CIDR
// like "10.x.x.x/24" or a proper "host:port") into an "https://host:port" base
// URL suitable for API sync and proxy calls. Returns "" if addr is empty.
func nodeAPIBase(addr string) string {
	if addr == "" {
		return ""
	}
	// Strip CIDR suffix (e.g. "10.0.0.2/24" → "10.0.0.2")
	if idx := strings.Index(addr, "/"); idx != -1 && !strings.Contains(addr[:idx], ":") {
		addr = addr[:idx]
	}
	// Add default port if none present
	if !strings.Contains(addr, ":") {
		addr = addr + ":8080"
	}
	return "https://" + addr
}

type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// Hijack implements the http.Hijacker interface to allow WebSocket/Terminal connections.
func (rw *responseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	h, ok := rw.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, errors.New("underlying ResponseWriter does not support hijacking")
	}
	return h.Hijack()
}

// Flush implements the http.Flusher interface for streaming responses.
func (rw *responseWriter) Flush() {
	if f, ok := rw.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// tracingMiddleware creates an OTEL-compatible span for every API request.
// Health, metrics, and OTLP ingest paths are skipped to avoid noise/recursion.
func tracingMiddleware(l *utils.Logger, next http.Handler) http.Handler {
	skip := map[string]bool{
		"/health": true, "/metrics": true,
		"/otlp/v1/traces": true, "/otlp/v1/logs": true, "/otlp/v1/metrics": true,
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if skip[r.URL.Path] {
			next.ServeHTTP(w, r)
			return
		}
		traceID := utils.NewTraceID()
		spanID := utils.NewSpanID()
		ctx := utils.WithTraceContext(r.Context(), traceID, spanID)
		start := time.Now()
		rw := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
		next.ServeHTTP(rw, r.WithContext(ctx))
		elapsed := time.Since(start)
		status := "ok"
		errMsg := ""
		if rw.statusCode >= 400 {
			status = "error"
			errMsg = fmt.Sprintf("HTTP %d", rw.statusCode)
		}
		utils.AppendSpan(utils.TraceEntry{
			TraceID:    traceID,
			SpanID:     spanID,
			Name:       r.Method + " " + r.URL.Path,
			Service:    "scutum",
			Kind:       "server",
			Time:       start,
			DurationMs: elapsed.Milliseconds(),
			Status:     status,
			Error:      errMsg,
			Source:     "internal",
			Attributes: map[string]string{
				"http.method":      r.Method,
				"http.route":       r.URL.Path,
				"http.status_code": strconv.Itoa(rw.statusCode),
			},
		})
	})
}

func metricsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rw := &responseWriter{w, http.StatusOK}
		next.ServeHTTP(rw, r)

		metrics.HttpRequestsTotal.WithLabelValues(
			r.Method,
			r.URL.Path,
			fmt.Sprintf("%d", rw.statusCode),
		).Inc()
	})
}

var (
	limiterMap = make(map[string]*visitor)
	limiterMu  stdsync.Mutex
)

type visitor struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

func getVisitor(ip string) *rate.Limiter {
	limiterMu.Lock()
	defer limiterMu.Unlock()

	v, exists := limiterMap[ip]
	if !exists {
		// Limit to 2 requests per second with a burst of 5 (sufficient for auth/setup)
		v = &visitor{limiter: rate.NewLimiter(2, 5)}
		limiterMap[ip] = v
	}
	v.lastSeen = time.Now()
	return v.limiter
}

// cleanupVisitors evicts rate-limiter state for IPs not seen in the last 5 minutes.
// Run this in a background goroutine on startup.
func cleanupVisitors(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			limiterMu.Lock()
			for ip, v := range limiterMap {
				if time.Since(v.lastSeen) > 5*time.Minute {
					delete(limiterMap, ip)
				}
			}
			limiterMu.Unlock()
		}
	}
}

func rateLimitMW(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := strings.Split(r.RemoteAddr, ":")[0]
		if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
			ip = strings.Split(forwarded, ",")[0]
		}
		limiter := getVisitor(strings.TrimSpace(ip))
		if !limiter.Allow() {
			http.Error(w, "Too many requests", http.StatusTooManyRequests)
			return
		}
		next.ServeHTTP(w, r)
	})
}
