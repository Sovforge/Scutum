package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

const RecoveryCodeCount = 10

// GenerateRecoveryCode returns a human-readable plaintext code and its SHA-256 hash.
// Format: xxxx-xxxx-xxxx-xxxx (16 random hex chars in 4 groups).
func GenerateRecoveryCode() (plain, hash string, err error) {
	raw := make([]byte, 8)
	if _, err = rand.Read(raw); err != nil {
		return "", "", fmt.Errorf("generate recovery code: %w", err)
	}
	encoded := hex.EncodeToString(raw)
	plain = encoded[0:4] + "-" + encoded[4:8] + "-" + encoded[8:12] + "-" + encoded[12:16]
	sum := sha256.Sum256([]byte(plain))
	hash = hex.EncodeToString(sum[:])
	return
}

// HashRecoveryCode returns the SHA-256 hash of a plaintext recovery code.
// Used to look up a user-submitted code in the database.
func HashRecoveryCode(plain string) string {
	sum := sha256.Sum256([]byte(plain))
	return hex.EncodeToString(sum[:])
}
