package tests

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"scutum/cmd/internal/certs"
)

// genCert creates a self-signed cert/key PEM pair for testing.
func genCert(t *testing.T, cn string, notAfter time.Time, dnsNames []string) (certPEM, keyPEM []byte) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(time.Now().UnixNano()),
		Subject:               pkix.Name{CommonName: cn},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              notAfter,
		DNSNames:              dnsNames,
		IPAddresses:           []net.IP{net.ParseIP("127.0.0.1")},
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		IsCA:                  true,
		BasicConstraintsValid: true,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("create certificate: %v", err)
	}
	certPEM = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatalf("marshal key: %v", err)
	}
	keyPEM = pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})
	return certPEM, keyPEM
}

func TestCertsStoreNoFiles(t *testing.T) {
	dir := t.TempDir()
	store, err := certs.NewStore(filepath.Join(dir, "server.crt"), filepath.Join(dir, "server.key"), "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if store.HasServerCert() {
		t.Fatal("expected no server cert loaded")
	}
	if info := store.Info(30); len(info) != 0 {
		t.Fatalf("expected empty info, got %v", info)
	}
	if _, err := store.GetConfigForClient(nil); err == nil {
		t.Fatal("expected error from GetConfigForClient with no cert loaded")
	}
}

func TestCertsStoreLoadsExistingFiles(t *testing.T) {
	dir := t.TempDir()
	certPath := filepath.Join(dir, "server.crt")
	keyPath := filepath.Join(dir, "server.key")

	certPEM, keyPEM := genCert(t, "hub.local", time.Now().AddDate(1, 0, 0), []string{"hub.local"})
	os.WriteFile(certPath, certPEM, 0644)
	os.WriteFile(keyPath, keyPEM, 0600)

	store, err := certs.NewStore(certPath, keyPath, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !store.HasServerCert() {
		t.Fatal("expected server cert loaded")
	}

	info := store.Info(30)
	if len(info) != 1 {
		t.Fatalf("expected 1 cert, got %d", len(info))
	}
	if info[0].Role != "server" {
		t.Fatalf("role = %q, want server", info[0].Role)
	}
	if info[0].ExpiringSoon {
		t.Fatal("cert expiring in a year should not be ExpiringSoon at warnDays=30")
	}
	if len(info[0].SANs) == 0 {
		t.Fatal("expected SANs to be populated")
	}

	cfg, err := store.GetConfigForClient(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.Certificates) != 1 {
		t.Fatalf("expected 1 certificate in returned config, got %d", len(cfg.Certificates))
	}
	if cfg.ClientAuth != tls.NoClientCert {
		t.Fatalf("ClientAuth = %v, want NoClientCert (no CA configured)", cfg.ClientAuth)
	}
}

func TestCertsStoreRotateServerCert(t *testing.T) {
	dir := t.TempDir()
	certPath := filepath.Join(dir, "server.crt")
	keyPath := filepath.Join(dir, "server.key")

	oldCertPEM, oldKeyPEM := genCert(t, "old.local", time.Now().AddDate(1, 0, 0), nil)
	os.WriteFile(certPath, oldCertPEM, 0644)
	os.WriteFile(keyPath, oldKeyPEM, 0600)

	store, err := certs.NewStore(certPath, keyPath, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	newCertPEM, newKeyPEM := genCert(t, "new.local", time.Now().AddDate(2, 0, 0), []string{"new.local"})
	info, err := store.RotateServerCert(newCertPEM, newKeyPEM)
	if err != nil {
		t.Fatalf("rotate failed: %v", err)
	}
	if info.Subject == "" || info.Role != "server" {
		t.Fatalf("unexpected info: %+v", info)
	}

	// Persisted to disk...
	onDisk, _ := os.ReadFile(certPath)
	if !bytes.Equal(onDisk, newCertPEM) {
		t.Fatal("rotated cert was not persisted to certPath")
	}

	// ...and live in the store.
	cfg, err := store.GetConfigForClient(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	leaf, err := x509.ParseCertificate(cfg.Certificates[0].Certificate[0])
	if err != nil {
		t.Fatalf("parse rotated leaf: %v", err)
	}
	if leaf.Subject.CommonName != "new.local" {
		t.Fatalf("CommonName = %q, want new.local", leaf.Subject.CommonName)
	}
}

func TestCertsStoreRotateServerCertRejectsMismatchedKey(t *testing.T) {
	dir := t.TempDir()
	certPath := filepath.Join(dir, "server.crt")
	keyPath := filepath.Join(dir, "server.key")
	certPEM, keyPEM := genCert(t, "a.local", time.Now().AddDate(1, 0, 0), nil)
	os.WriteFile(certPath, certPEM, 0644)
	os.WriteFile(keyPath, keyPEM, 0600)

	store, err := certs.NewStore(certPath, keyPath, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	otherCertPEM, _ := genCert(t, "b.local", time.Now().AddDate(1, 0, 0), nil)
	_, otherKeyPEM := genCert(t, "c.local", time.Now().AddDate(1, 0, 0), nil)
	if _, err := store.RotateServerCert(otherCertPEM, otherKeyPEM); err == nil {
		t.Fatal("expected error rotating in a cert/key pair that don't match")
	}
}

func TestCertsStoreRotateCA(t *testing.T) {
	dir := t.TempDir()
	certPath := filepath.Join(dir, "server.crt")
	keyPath := filepath.Join(dir, "server.key")
	caPath := filepath.Join(dir, "ca.crt")

	certPEM, keyPEM := genCert(t, "hub.local", time.Now().AddDate(1, 0, 0), nil)
	os.WriteFile(certPath, certPEM, 0644)
	os.WriteFile(keyPath, keyPEM, 0600)
	initialCAPEM, _ := genCert(t, "initial-ca", time.Now().AddDate(5, 0, 0), nil)
	os.WriteFile(caPath, initialCAPEM, 0644)

	store, err := certs.NewStore(certPath, keyPath, caPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(store.Info(30)) != 2 {
		t.Fatal("expected both the server and the initially-configured CA cert")
	}

	caCertPEM, _ := genCert(t, "test-ca", time.Now().AddDate(5, 0, 0), nil)
	info, err := store.RotateCA(caCertPEM)
	if err != nil {
		t.Fatalf("rotate CA failed: %v", err)
	}
	if info.Role != "ca" {
		t.Fatalf("role = %q, want ca", info.Role)
	}

	onDisk, _ := os.ReadFile(caPath)
	if !bytes.Equal(onDisk, caCertPEM) {
		t.Fatal("rotated CA was not persisted to caPath")
	}

	if len(store.Info(30)) != 2 {
		t.Fatal("expected both server and ca certs after RotateCA")
	}

	cfg, err := store.GetConfigForClient(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.ClientAuth != tls.RequireAndVerifyClientCert {
		t.Fatalf("ClientAuth = %v, want RequireAndVerifyClientCert once a CA is set", cfg.ClientAuth)
	}
}

func TestCertsStoreRotateCAWithoutPathConfigured(t *testing.T) {
	dir := t.TempDir()
	certPath := filepath.Join(dir, "server.crt")
	keyPath := filepath.Join(dir, "server.key")
	certPEM, keyPEM := genCert(t, "hub.local", time.Now().AddDate(1, 0, 0), nil)
	os.WriteFile(certPath, certPEM, 0644)
	os.WriteFile(keyPath, keyPEM, 0600)

	store, err := certs.NewStore(certPath, keyPath, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	caCertPEM, _ := genCert(t, "test-ca", time.Now().AddDate(5, 0, 0), nil)
	if _, err := store.RotateCA(caCertPEM); err == nil {
		t.Fatal("expected error rotating a CA when no CA_CERT_FILE path was configured")
	}
}

func TestCertsStoreExpiringSoon(t *testing.T) {
	dir := t.TempDir()
	certPath := filepath.Join(dir, "server.crt")
	keyPath := filepath.Join(dir, "server.key")
	certPEM, keyPEM := genCert(t, "hub.local", time.Now().AddDate(0, 0, 5), nil)
	os.WriteFile(certPath, certPEM, 0644)
	os.WriteFile(keyPath, keyPEM, 0600)

	store, err := certs.NewStore(certPath, keyPath, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !store.Info(30)[0].ExpiringSoon {
		t.Fatal("cert expiring in 5 days should be ExpiringSoon at warnDays=30")
	}
	if store.Info(1)[0].ExpiringSoon {
		t.Fatal("cert expiring in 5 days should not be ExpiringSoon at warnDays=1")
	}
}
