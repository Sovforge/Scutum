package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"golang.org/x/crypto/argon2"
)

const (
	argonTime    = 1
	argonMemory  = 64 * 1024
	argonThreads = 4
	argonKeyLen  = 32
	saltLen      = 16
)

type Claims struct {
	UserID   string
	Username string
	Expires  time.Time
}

// HashPassword hashes a plaintext password using argon2id.
func HashPassword(password string) (string, error) {
	salt := make([]byte, saltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("generate salt: %w", err)
	}
	hash := argon2.IDKey([]byte(password), salt, argonTime, argonMemory, argonThreads, argonKeyLen)
	encoded := fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, argonMemory, argonTime, argonThreads,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(hash),
	)
	return encoded, nil
}

// VerifyPassword checks a plaintext password against an argon2id hash.
func VerifyPassword(password, encoded string) (bool, error) {
	var version, memory, iterations, parallelism uint32
	var saltB64, hashB64 string

	// The hash format is: $argon2id$v=19$m=65536,t=1,p=4$<salt>$<hash>
	// We need to parse it carefully since the salt and hash contain base64 which
	// may have = signs or / characters.
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[0] != "" || parts[1] != "argon2id" {
		return false, fmt.Errorf("parse hash: invalid format")
	}

	// Parse parts[2]: "v=19"
	if !strings.HasPrefix(parts[2], "v=") {
		return false, fmt.Errorf("parse hash: missing version")
	}
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil {
		return false, fmt.Errorf("parse hash: %w", err)
	}

	// Parse parts[3]: "m=65536,t=1,p=4"
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &memory, &iterations, &parallelism); err != nil {
		return false, fmt.Errorf("parse hash: %w", err)
	}

	saltB64 = parts[4]
	hashB64 = parts[5]

	salt, err := base64.RawStdEncoding.DecodeString(saltB64)
	if err != nil {
		return false, fmt.Errorf("decode salt: %w", err)
	}
	expectedHash, err := base64.RawStdEncoding.DecodeString(hashB64)
	if err != nil {
		return false, fmt.Errorf("decode hash: %w", err)
	}

	actualHash := argon2.IDKey([]byte(password), salt, iterations, memory, uint8(parallelism), uint32(len(expectedHash)))

	// Constant-time comparison
	if len(actualHash) != len(expectedHash) {
		return false, nil
	}
	var diff byte
	for i := range actualHash {
		diff |= actualHash[i] ^ expectedHash[i]
	}
	return diff == 0, nil
}

// HashAPIKey hashes an API key for storage using SHA-256.
// API keys are long random strings so SHA-256 is sufficient — no salt needed.
func HashAPIKey(key string) string {
	h := sha256.Sum256([]byte(key))
	return hex.EncodeToString(h[:])
}

// GenerateAPIKey creates a cryptographically secure random API key.
func GenerateAPIKey() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate api key: %w", err)
	}
	return "orch_" + base64.RawURLEncoding.EncodeToString(b), nil
}

var ErrInvalidToken = errors.New("invalid token")
var ErrExpiredToken = errors.New("token expired")

type contextKey struct{}

// WithClaims stores claims in the request context.
func WithClaims(ctx context.Context, claims Claims) context.Context {
	return context.WithValue(ctx, contextKey{}, claims)
}

// ClaimsFromContext retrieves claims from the request context.
func ClaimsFromContext(ctx context.Context) (Claims, bool) {
	c, ok := ctx.Value(contextKey{}).(Claims)
	return c, ok
}
