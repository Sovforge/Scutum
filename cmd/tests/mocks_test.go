package tests

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
)

const (
	testN = 5
	testT = 3
)

type mockKMS struct {
	masterKey []byte
}

func (m *mockKMS) Encrypt(ctx context.Context, plaintext []byte) ([]byte, error) {
	return append([]byte("enc:"), plaintext...), nil
}
func (m *mockKMS) Decrypt(ctx context.Context, ciphertext []byte) ([]byte, error) {
	return ciphertext[4:], nil
}
func (m *mockKMS) Name() string { return "mock" }
func (m *mockKMS) Wipe()        { m.masterKey = nil }
func (m *mockKMS) LoadMasterKey(key []byte) error {
	m.masterKey = make([]byte, len(key))
	copy(m.masterKey, key)
	return nil
}

func (m *mockKMS) MasterKeyDigest() ([]byte, error) {
	mac := hmac.New(sha256.New, m.masterKey)
	mac.Write([]byte("scutum-erk-digest-v1"))
	return mac.Sum(nil), nil
}


type mockSecretStore struct {
	reEncrypted bool
}

func (m *mockSecretStore) ReEncryptAllDEKs(ctx context.Context, oldKey, newKey []byte) error {
	m.reEncrypted = true
	return nil
}
