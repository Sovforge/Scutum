package kms

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"fmt"
	"io"
)

const dekSize = 32

// DEK holds an encrypted data encryption key and the ciphertext it produced.
type DEK struct {
	EncryptedKey []byte
	Ciphertext   []byte
}

// Seal generates a fresh DEK, encrypts plaintext with it using AES-256-GCM,
// then wraps the DEK with the master key via the provider.
func Seal(ctx context.Context, provider Provider, plaintext []byte) (DEK, error) {
	// Generate a fresh DEK
	dek := make([]byte, dekSize)
	if _, err := io.ReadFull(rand.Reader, dek); err != nil {
		return DEK{}, fmt.Errorf("generate dek: %w", err)
	}

	// Encrypt plaintext with DEK using AES-256-GCM
	ciphertext, err := aesGCMEncrypt(dek, plaintext)
	if err != nil {
		return DEK{}, fmt.Errorf("encrypt with dek: %w", err)
	}

	// Wrap the DEK with the master key
	encryptedKey, err := provider.Encrypt(ctx, dek)
	if err != nil {
		return DEK{}, fmt.Errorf("wrap dek: %w", err)
	}

	return DEK{EncryptedKey: encryptedKey, Ciphertext: ciphertext}, nil
}

// Open unwraps the DEK using the provider then decrypts the ciphertext.
func Open(ctx context.Context, provider Provider, d DEK) ([]byte, error) {
	dek, err := provider.Decrypt(ctx, d.EncryptedKey)
	if err != nil {
		return nil, fmt.Errorf("unwrap dek: %w", err)
	}
	plaintext, err := aesGCMDecrypt(dek, d.Ciphertext)
	if err != nil {
		return nil, fmt.Errorf("decrypt with dek: %w", err)
	}
	return plaintext, nil
}

// ReWrap decrypts the DEK with the old provider and re-encrypts it with the new one.
// The ciphertext is unchanged — only the key wrapper rotates.
func ReWrap(ctx context.Context, oldProvider, newProvider Provider, encryptedKey []byte) ([]byte, error) {
	dek, err := oldProvider.Decrypt(ctx, encryptedKey)
	if err != nil {
		return nil, fmt.Errorf("unwrap dek for rotation: %w", err)
	}
	newEncryptedKey, err := newProvider.Encrypt(ctx, dek)
	if err != nil {
		return nil, fmt.Errorf("re-wrap dek: %w", err)
	}
	return newEncryptedKey, nil
}

func aesGCMEncrypt(key, plaintext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}
	return gcm.Seal(nonce, nonce, plaintext, nil), nil
}

func aesGCMDecrypt(key, ciphertext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	if len(ciphertext) < gcm.NonceSize() {
		return nil, fmt.Errorf("ciphertext too short")
	}
	nonce, ciphertext := ciphertext[:gcm.NonceSize()], ciphertext[gcm.NonceSize():]
	return gcm.Open(nil, nonce, ciphertext, nil)
}
