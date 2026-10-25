// Package certs manages the hub's own TLS server certificate and the
// optional CA used to verify inbound client certs (mTLS via CA_CERT_FILE).
// It exists so both can be inspected and rotated at runtime, without a
// restart — previously the only way to replace either was to overwrite the
// file on disk and restart the process.
//
// This does not model per-node certificates: node identity in this mesh is
// a WireGuard public key plus HMAC-signed requests, not x509 — there is no
// per-node cert material to inventory or rotate.
//
// ACME-issued certificates are out of scope here too: autocert.Manager
// already renews those automatically on its own schedule.
package certs

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Info is the metadata exposed for one certificate — never the private key.
type Info struct {
	Role          string    `json:"role"` // "server" | "ca"
	Subject       string    `json:"subject"`
	Issuer        string    `json:"issuer"`
	SerialNumber  string    `json:"serial_number"`
	NotBefore     time.Time `json:"not_before"`
	NotAfter      time.Time `json:"not_after"`
	DaysRemaining int       `json:"days_remaining"`
	ExpiringSoon  bool      `json:"expiring_soon"`
	SANs          []string  `json:"sans"`
}

// Store holds the hub's current server certificate and optional client-CA
// pool, guarded by a lock so a rotation can swap them while the TLS
// listener keeps serving with whatever was current at handshake time.
type Store struct {
	mu sync.RWMutex

	certPath string
	keyPath  string
	cert     *tls.Certificate
	certLeaf *x509.Certificate

	caPath string
	caPool *x509.CertPool
	caLeaf *x509.Certificate
}

// NewStore loads the initial server cert/key and, if caFile is non-empty,
// the CA cert used for inbound client-cert verification. certFile/keyFile
// may point at files that don't exist yet — the store just starts with no
// server cert loaded (TLS disabled), matching the existing startup check
// in cmd/api/main.go.
func NewStore(certFile, keyFile, caFile string) (*Store, error) {
	s := &Store{certPath: certFile, keyPath: keyFile, caPath: caFile}

	if _, err := os.Stat(certFile); err == nil {
		if _, err := os.Stat(keyFile); err == nil {
			if err := s.loadServerCert(); err != nil {
				return nil, err
			}
		}
	}

	if caFile != "" {
		if err := s.loadCA(); err != nil {
			return nil, err
		}
	}

	return s, nil
}

func (s *Store) loadServerCert() error {
	certPEM, err := os.ReadFile(s.certPath)
	if err != nil {
		return fmt.Errorf("read cert file: %w", err)
	}
	keyPEM, err := os.ReadFile(s.keyPath)
	if err != nil {
		return fmt.Errorf("read key file: %w", err)
	}
	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return fmt.Errorf("parse cert/key: %w", err)
	}
	leaf, err := x509.ParseCertificate(cert.Certificate[0])
	if err != nil {
		return fmt.Errorf("parse leaf cert: %w", err)
	}
	s.cert = &cert
	s.certLeaf = leaf
	return nil
}

func (s *Store) loadCA() error {
	caPEM, err := os.ReadFile(s.caPath)
	if err != nil {
		return fmt.Errorf("read CA cert file: %w", err)
	}
	pool := x509.NewCertPool()
	if ok := pool.AppendCertsFromPEM(caPEM); !ok {
		return fmt.Errorf("no valid certificates found in CA file")
	}
	block, _ := pem.Decode(caPEM)
	if block == nil {
		return fmt.Errorf("no PEM block found in CA file")
	}
	leaf, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return fmt.Errorf("parse CA cert: %w", err)
	}
	s.caPool = pool
	s.caLeaf = leaf
	return nil
}

// HasServerCert reports whether a server cert/key pair is currently loaded.
func (s *Store) HasServerCert() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.cert != nil
}

// GetConfigForClient is set as tls.Config.GetConfigForClient so every new
// handshake picks up whatever cert/CA is current, without restarting the
// listener. It supersedes the base tls.Config's static Certificates/
// ClientCAs/ClientAuth for that connection.
func (s *Store) GetConfigForClient(_ *tls.ClientHelloInfo) (*tls.Config, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.cert == nil {
		return nil, fmt.Errorf("no server certificate loaded")
	}
	cfg := &tls.Config{
		MinVersion:   tls.VersionTLS12,
		Certificates: []tls.Certificate{*s.cert},
	}
	if s.caPool != nil {
		cfg.ClientCAs = s.caPool
		cfg.ClientAuth = tls.RequireAndVerifyClientCert
	}
	return cfg, nil
}

// RotateServerCert validates the given PEM cert/key pair, persists them to
// the configured cert/key file paths, and swaps them into the live store.
// New connections pick up the new cert immediately; connections already in
// flight keep whatever they negotiated.
func (s *Store) RotateServerCert(certPEM, keyPEM []byte) (Info, error) {
	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return Info{}, fmt.Errorf("invalid certificate/key pair: %w", err)
	}
	leaf, err := x509.ParseCertificate(cert.Certificate[0])
	if err != nil {
		return Info{}, fmt.Errorf("parse leaf cert: %w", err)
	}

	if err := writeFileAtomic(s.certPath, certPEM, 0644); err != nil {
		return Info{}, fmt.Errorf("write cert file: %w", err)
	}
	if err := writeFileAtomic(s.keyPath, keyPEM, 0600); err != nil {
		return Info{}, fmt.Errorf("write key file: %w", err)
	}

	s.mu.Lock()
	s.cert = &cert
	s.certLeaf = leaf
	s.mu.Unlock()

	return infoFor("server", leaf, 30), nil
}

// RotateCA validates the given PEM CA cert, persists it to the configured
// CA file path, and swaps it into the live store. Once set, inbound
// connections to the hub's HTTPS server must present a client cert signed
// by this CA (tls.RequireAndVerifyClientCert) from that point on.
func (s *Store) RotateCA(caPEM []byte) (Info, error) {
	pool := x509.NewCertPool()
	if ok := pool.AppendCertsFromPEM(caPEM); !ok {
		return Info{}, fmt.Errorf("no valid certificates found in CA PEM")
	}
	block, _ := pem.Decode(caPEM)
	if block == nil {
		return Info{}, fmt.Errorf("no PEM block found in CA PEM")
	}
	leaf, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return Info{}, fmt.Errorf("parse CA cert: %w", err)
	}

	caPath := s.caPath
	if caPath == "" {
		return Info{}, fmt.Errorf("no CA_CERT_FILE path configured for this hub")
	}
	if err := writeFileAtomic(caPath, caPEM, 0644); err != nil {
		return Info{}, fmt.Errorf("write CA file: %w", err)
	}

	s.mu.Lock()
	s.caPool = pool
	s.caLeaf = leaf
	s.mu.Unlock()

	return infoFor("ca", leaf, 30), nil
}

// Info returns metadata for every certificate currently loaded (the server
// cert, and the CA cert if mTLS is configured) — never key material.
// warnDays sets the ExpiringSoon threshold.
func (s *Store) Info(warnDays int) []Info {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var out []Info
	if s.certLeaf != nil {
		out = append(out, infoFor("server", s.certLeaf, warnDays))
	}
	if s.caLeaf != nil {
		out = append(out, infoFor("ca", s.caLeaf, warnDays))
	}
	return out
}

func infoFor(role string, leaf *x509.Certificate, warnDays int) Info {
	days := int(time.Until(leaf.NotAfter).Hours() / 24)
	sans := make([]string, 0, len(leaf.DNSNames)+len(leaf.IPAddresses))
	sans = append(sans, leaf.DNSNames...)
	for _, ip := range leaf.IPAddresses {
		sans = append(sans, ip.String())
	}
	return Info{
		Role:          role,
		Subject:       leaf.Subject.String(),
		Issuer:        leaf.Issuer.String(),
		SerialNumber:  leaf.SerialNumber.String(),
		NotBefore:     leaf.NotBefore,
		NotAfter:      leaf.NotAfter,
		DaysRemaining: days,
		ExpiringSoon:  days < warnDays,
		SANs:          sans,
	}
}

// writeFileAtomic writes to a temp file in the same directory and renames
// it into place, so a crash mid-write can never leave a half-written cert
// or key file for the next load to trip over.
func writeFileAtomic(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath) //nolint:errcheck

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmpPath, perm); err != nil {
		return err
	}
	return os.Rename(tmpPath, path)
}
