package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/base32"
	"encoding/binary"
	"fmt"
	"math"
	"net/url"
	"strings"
	"sync"
	"time"
)

// GenerateTOTPSecret creates a random 20-byte base32-encoded TOTP secret.
func GenerateTOTPSecret() (string, error) {
	raw := make([]byte, 20)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("generate totp secret: %w", err)
	}
	return strings.TrimRight(base32.StdEncoding.EncodeToString(raw), "="), nil
}

// TOTPUri returns the otpauth:// URI for QR code generation or manual entry.
func TOTPUri(secret, username, issuer string) string {
	label := url.PathEscape(issuer + ":" + username)
	return fmt.Sprintf(
		"otpauth://totp/%s?secret=%s&issuer=%s&algorithm=SHA1&digits=6&period=30",
		label, secret, url.QueryEscape(issuer),
	)
}

// usedTOTP tracks consumed (secret-hash, window) pairs to prevent replay attacks.
var (
	usedTOTPMu sync.Mutex
	usedTOTP   = make(map[string]time.Time) // key → expiry
)

// VerifyTOTP returns true if code matches the current or adjacent TOTP window
// for the given base32 secret. Each window is accepted only once per secret.
func VerifyTOTP(secret, code string) bool {
	if len(code) != 6 {
		return false
	}
	padded := secret
	for len(padded)%8 != 0 {
		padded += "="
	}
	key, err := base32.StdEncoding.DecodeString(strings.ToUpper(padded))
	if err != nil {
		return false
	}
	h := sha256.Sum256(key)
	now := time.Now().Unix() / 30
	for _, t := range []int64{now - 1, now, now + 1} {
		if totpCode(key, uint64(t)) == code {
			cacheKey := fmt.Sprintf("%x:%d", h, t)
			usedTOTPMu.Lock()
			// Purge expired entries while holding the lock.
			now2 := time.Now()
			for k, exp := range usedTOTP {
				if now2.After(exp) {
					delete(usedTOTP, k)
				}
			}
			_, replay := usedTOTP[cacheKey]
			if !replay {
				usedTOTP[cacheKey] = time.Unix((t+2)*30, 0) // expires after window + buffer
			}
			usedTOTPMu.Unlock()
			return !replay
		}
	}
	return false
}

func totpCode(key []byte, counter uint64) string {
	msg := make([]byte, 8)
	binary.BigEndian.PutUint64(msg, counter)
	mac := hmac.New(sha1.New, key)
	mac.Write(msg)
	h := mac.Sum(nil)
	offset := h[len(h)-1] & 0x0f
	code := int(h[offset]&0x7f)<<24 |
		int(h[offset+1])<<16 |
		int(h[offset+2])<<8 |
		int(h[offset+3])
	return fmt.Sprintf("%06d", code%int(math.Pow10(6)))
}
