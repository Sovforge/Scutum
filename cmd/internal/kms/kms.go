package kms

import (
	"context"
	"crypto/cipher"
	"crypto/aes"
	"crypto/rand"
	"errors"
)

// Provider is the interface both age and Vault must satisfy.
type Provider interface {
	// Encrypt encrypts plaintext and returns ciphertext.
	Encrypt(ctx context.Context, plaintext []byte) ([]byte, error)
	// Decrypt decrypts ciphertext and returns plaintext.
	Decrypt(ctx context.Context, ciphertext []byte) ([]byte, error)
	// Name returns the provider identifier used to tag stored secrets.
	Name() string

	// Wipe zeros all in-memory key material immediately.
	Wipe()
	// LoadMasterKey installs a new master key into the provider.
	LoadMasterKey(key []byte) error
	// MasterKeyDigest returns a short digest of the current master key
	// used only to verify a reconstructed key without exposing the key itself.
	MasterKeyDigest() ([]byte, error)
}

// SecretStore is the subset of store.Store needed by recovery.
type SecretStore interface {
	// ReEncryptAllDEKs decrypts every DEK with oldMasterKey and re-encrypts
	// with newMasterKey in a single transaction.
	ReEncryptAllDEKs(ctx context.Context, oldMasterKey, newMasterKey []byte) error
}

func ReEncryptDEK(ctx context.Context, oldKey, newKey, encryptedDEK []byte) ([]byte, error) {
	block, err := aes.NewCipher(oldKey)
	if err != nil {
		return nil, err
	}
	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonceSize := aesgcm.NonceSize()
	if len(encryptedDEK) < nonceSize {
		return nil, errors.New("encrypted DEK too short")
	}
	nonce, ciphertext := encryptedDEK[:nonceSize], encryptedDEK[nonceSize:]
	plaintext, err := aesgcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, err
	}
	block2, err := aes.NewCipher(newKey)
	if err != nil {
		return nil, err
	}
	aesgcm2, err := cipher.NewGCM(block2)
	if err != nil {
		return nil, err
	}
	newNonce := make([]byte, nonceSize)
	if _, err := rand.Read(newNonce); err != nil {
		return nil, err
	}
	newCiphertext := aesgcm2.Seal(nil, newNonce, plaintext, nil)
	result := make([]byte, nonceSize+len(newCiphertext))
	copy(result[:nonceSize], newNonce)
	copy(result[nonceSize:], newCiphertext)
	return result, nil
}
