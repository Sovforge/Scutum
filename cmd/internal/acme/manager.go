package acme

import (
	"crypto/tls"
	"net/http"
	"os"
	"path/filepath"

	goacme "golang.org/x/crypto/acme"
	"golang.org/x/crypto/acme/autocert"
)

// Config holds ACME configuration read from environment variables.
// ACME is only active when Domain and Email are both non-empty.
type Config struct {
	Domain   string // ACME_DOMAIN
	Email    string // ACME_EMAIL
	Staging  bool   // ACME_STAGING
	CacheDir string // ACME_CACHE_DIR (default: <secretsDir>/acme)
}

// FromEnv builds a Config from environment variables.
func FromEnv(secretsDir string) Config {
	cacheDir := os.Getenv("ACME_CACHE_DIR")
	if cacheDir == "" {
		cacheDir = filepath.Join(secretsDir, "acme")
	}
	return Config{
		Domain:   os.Getenv("ACME_DOMAIN"),
		Email:    os.Getenv("ACME_EMAIL"),
		Staging:  os.Getenv("ACME_STAGING") == "true",
		CacheDir: cacheDir,
	}
}

// Enabled returns true when both Domain and Email are set.
func (c Config) Enabled() bool {
	return c.Domain != "" && c.Email != ""
}

// Manager wraps autocert.Manager for automatic certificate provisioning.
type Manager struct {
	m *autocert.Manager
}

// New creates a Manager. Call only when cfg.Enabled() is true.
func New(cfg Config) *Manager {
	m := &autocert.Manager{
		Prompt:     autocert.AcceptTOS,
		HostPolicy: autocert.HostWhitelist(cfg.Domain),
		Cache:      autocert.DirCache(cfg.CacheDir),
		Email:      cfg.Email,
	}
	if cfg.Staging {
		m.Client = &goacme.Client{
			DirectoryURL: "https://acme-staging-v02.api.letsencrypt.org/directory",
		}
	}
	return &Manager{m: m}
}

// TLSConfig returns a *tls.Config that serves ACME-provisioned certificates.
func (m *Manager) TLSConfig() *tls.Config {
	return m.m.TLSConfig()
}

// ChallengeHandler wraps next with the ACME HTTP-01 challenge responder.
// Mount this on port 80; all non-challenge requests are redirected to HTTPS.
func (m *Manager) ChallengeHandler(domain string) http.Handler {
	return m.m.HTTPHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		target := "https://" + domain + r.RequestURI
		http.Redirect(w, r, target, http.StatusMovedPermanently)
	}))
}
