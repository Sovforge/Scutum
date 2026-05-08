package kms

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

const erkDigestLabel = "scutum-erk-digest-v1"

type LocalKeyProvider struct {
	mu        sync.RWMutex
	masterKey []byte
}

// NewLocalKeyProvider loads a 32-byte hex-encoded master key from a file.
// If the file does not exist, a new key is generated and saved.
func NewLocalKeyProvider(keyFilePath string) (*LocalKeyProvider, error) {
	data, err := os.ReadFile(keyFilePath)
	if os.IsNotExist(err) {
		return generateLocalKey(keyFilePath)
	}
	if err != nil {
		return nil, fmt.Errorf("read key file: %w", err)
	}
	key, err := hex.DecodeString(strings.TrimSpace(string(data)))
	if err != nil {
		return nil, fmt.Errorf("decode master key: %w", err)
	}
	if len(key) != 32 {
		return nil, fmt.Errorf("master key must be 32 bytes, got %d", len(key))
	}
	return &LocalKeyProvider{masterKey: key}, nil
}

func generateLocalKey(path string) (*LocalKeyProvider, error) {
	key := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		return nil, fmt.Errorf("generate master key: %w", err)
	}
	if dir := filepath.Dir(path); dir != "." {
		if err := os.MkdirAll(dir, 0700); err != nil {
			return nil, fmt.Errorf("create key directory: %w", err)
		}
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0600)
	if err != nil {
		return nil, fmt.Errorf("create key file: %w", err)
	}
	defer f.Close()
	if _, err := fmt.Fprintf(f, "%s\n", hex.EncodeToString(key)); err != nil {
		return nil, fmt.Errorf("write master key: %w", err)
	}
	return &LocalKeyProvider{masterKey: key}, nil
}

// Encrypt wraps plaintext (a DEK) using AES-256-GCM with the master key.
func (p *LocalKeyProvider) Encrypt(_ context.Context, plaintext []byte) ([]byte, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if p.masterKey == nil {
		return nil, fmt.Errorf("localkey: provider has been wiped")
	}
	return aesGCMEncrypt(p.masterKey, plaintext)
}

// Decrypt unwraps ciphertext (a wrapped DEK) using the master key.
func (p *LocalKeyProvider) Decrypt(_ context.Context, ciphertext []byte) ([]byte, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if p.masterKey == nil {
		return nil, fmt.Errorf("localkey: provider has been wiped")
	}
	return aesGCMDecrypt(p.masterKey, ciphertext)
}

func (p *LocalKeyProvider) Name() string { return "localkey" }

// Wipe zeros and clears the in-memory master key immediately.
// After this call, Encrypt and Decrypt will return errors until
// LoadMasterKey is called with a new key.
func (p *LocalKeyProvider) Wipe() {
	p.mu.Lock()
	defer p.mu.Unlock()
	for i := range p.masterKey {
		p.masterKey[i] = 0
	}
	p.masterKey = nil
}

// LoadMasterKey installs key as the active master key.
// key must be exactly 32 bytes. The previous key is zeroed before replacement.
func (p *LocalKeyProvider) LoadMasterKey(key []byte) error {
	if len(key) != 32 {
		return fmt.Errorf("localkey: master key must be 32 bytes, got %d", len(key))
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	// Zero the old key before replacing it.
	for i := range p.masterKey {
		p.masterKey[i] = 0
	}
	p.masterKey = make([]byte, 32)
	copy(p.masterKey, key)
	return nil
}

// ExportMasterKey returns a copy of the raw master key for share-generation purposes.
// The caller must zero the returned slice after use.
func (p *LocalKeyProvider) ExportMasterKey() ([]byte, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if p.masterKey == nil {
		return nil, fmt.Errorf("localkey: provider has been wiped")
	}
	out := make([]byte, len(p.masterKey))
	copy(out, p.masterKey)
	return out, nil
}

// MasterKeyDigest returns a 16-byte HMAC-SHA256 digest of the master key.
// This is used by recovery to verify a reconstructed key without ever
// exposing the key itself. The digest is deterministic for a given key.
func (p *LocalKeyProvider) MasterKeyDigest() ([]byte, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if p.masterKey == nil {
		return nil, fmt.Errorf("localkey: provider has been wiped")
	}
	mac := hmac.New(sha256.New, p.masterKey)
	mac.Write([]byte(erkDigestLabel))
	return mac.Sum(nil)[:16], nil
}
