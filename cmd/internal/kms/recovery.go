package kms

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
)

// ---------------------------------------------------------------------------
// GF(256) arithmetic — AES field, reduction polynomial x^8+x^4+x^3+x+1
// ---------------------------------------------------------------------------

const gfPoly = 0x1b // low 8 bits of 0x11b

var (
	gfExp [512]byte // anti-log table, doubled to avoid mod
	gfLog [256]byte // log table
)

func init() {
	x := byte(1)
	for i := 0; i < 255; i++ {
		gfExp[i] = x
		gfExp[i+255] = x
		gfLog[x] = byte(i)
		// multiply by 3 (primitive root of GF(256) AES): x*3 = xtime(x) XOR x
		t := x
		if x&0x80 != 0 {
			x = (x << 1) ^ gfPoly
		} else {
			x <<= 1
		}
		x ^= t
	}
}

func gfMul(a, b byte) byte {
	if a == 0 || b == 0 {
		return 0
	}
	return gfExp[int(gfLog[a])+int(gfLog[b])]
}

func gfDiv(a, b byte) byte {
	if b == 0 {
		panic("kms/recovery: division by zero in GF(256)")
	}
	if a == 0 {
		return 0
	}
	return gfExp[int(gfLog[a])+255-int(gfLog[b])]
}

func gfPow(a, b byte) byte {
	if b == 0 {
		return 1
	}
	return gfExp[int(gfLog[a])*int(b)%255]
}

// ---------------------------------------------------------------------------
// Polynomial evaluation over GF(256)
// ---------------------------------------------------------------------------

// polyEval evaluates a polynomial at x.
// coeffs[0] is the constant term (the secret byte).
func polyEval(coeffs []byte, x byte) byte {
	result := byte(0)
	for i := len(coeffs) - 1; i >= 0; i-- {
		result = gfMul(result, x) ^ coeffs[i]
	}
	return result
}

// ---------------------------------------------------------------------------
// Share encoding
// ---------------------------------------------------------------------------

const (
	sharePrefix  = "scutum-erk"
	shareVersion = "v1"
)

// Share is a single Shamir share, safe to print and store offline.
// Format: scutum-erk-v1-<x>-<base64(bytes)>
type Share struct {
	X     byte   // x coordinate (1-255), unique per share
	Bytes []byte // y values, one per secret byte
}

// String encodes the share as a printable string the operator can write down.
func (s Share) String() string {
	return fmt.Sprintf("%s-%s-%d-%s",
		sharePrefix, shareVersion,
		s.X,
		base64.RawURLEncoding.EncodeToString(s.Bytes),
	)
}

// ParseShare decodes a share from its printed string form.
func ParseShare(raw string) (Share, error) {
	parts := strings.SplitN(raw, "-", 5)
	// expected: "scutum", "erk", "v1", "<x>", "<base64>"
	if len(parts) != 5 {
		return Share{}, errors.New("invalid share format")
	}
	if parts[0]+"-"+parts[1] != sharePrefix {
		return Share{}, errors.New("invalid share prefix")
	}
	if parts[2] != shareVersion {
		return Share{}, fmt.Errorf("unsupported share version %q", parts[2])
	}
	var x uint
	if _, err := fmt.Sscanf(parts[3], "%d", &x); err != nil || x == 0 || x > 255 {
		return Share{}, errors.New("invalid share x coordinate")
	}
	b, err := base64.RawURLEncoding.DecodeString(parts[4])
	if err != nil {
		return Share{}, fmt.Errorf("invalid share bytes: %w", err)
	}
	return Share{X: byte(x), Bytes: b}, nil
}

// ---------------------------------------------------------------------------
// Core split / combine
// ---------------------------------------------------------------------------

// splitSecret splits a secret byte slice into n shares with threshold t.
// Each share byte is a point on an independent degree-(t-1) polynomial
// whose constant term is the secret byte.
func splitSecret(secret []byte, n, t int) ([]Share, error) {
	if t < 2 || t > n || n > 255 {
		return nil, fmt.Errorf("invalid parameters: t=%d n=%d", t, n)
	}

	// Pick n distinct non-zero x coordinates at random.
	xs := make([]byte, n)
	used := make(map[byte]bool)
	buf := make([]byte, 1)
	for i := 0; i < n; {
		if _, err := rand.Read(buf); err != nil {
			return nil, fmt.Errorf("random x: %w", err)
		}
		if buf[0] == 0 || used[buf[0]] {
			continue
		}
		xs[i] = buf[0]
		used[buf[0]] = true
		i++
	}

	shares := make([]Share, n)
	for i := range shares {
		shares[i] = Share{X: xs[i], Bytes: make([]byte, len(secret))}
	}

	coeffs := make([]byte, t)
	for byteIdx := range secret {
		coeffs[0] = secret[byteIdx]
		// random coefficients for degrees 1..t-1
		if _, err := rand.Read(coeffs[1:]); err != nil {
			return nil, fmt.Errorf("random coefficients: %w", err)
		}
		for i, x := range xs {
			shares[i].Bytes[byteIdx] = polyEval(coeffs, x)
		}
	}

	// Zero coefficients before returning.
	for i := range coeffs {
		coeffs[i] = 0
	}
	return shares, nil
}

// combineShares reconstructs the secret from shares using Lagrange interpolation at x=0.
// All provided shares must be valid (on the same polynomial). Providing any number
// >= threshold gives the correct result; a corrupted share will produce a wrong result
// that will be caught by VerifyMasterKey.
func combineShares(shares []Share) ([]byte, error) {
	if len(shares) < 2 {
		return nil, errors.New("need at least 2 shares")
	}
	secretLen := len(shares[0].Bytes)
	for _, s := range shares[1:] {
		if len(s.Bytes) != secretLen {
			return nil, errors.New("shares have inconsistent lengths")
		}
	}

	secret := make([]byte, secretLen)
	for byteIdx := 0; byteIdx < secretLen; byteIdx++ {
		// Lagrange interpolation at x=0
		val := byte(0)
		for i, si := range shares {
			num := byte(1)
			den := byte(1)
			for j, sj := range shares {
				if i == j {
					continue
				}
				// num *= (0 - sj.X) = sj.X in GF(256) (subtraction = XOR)
				num = gfMul(num, sj.X)
				// den *= (si.X - sj.X) = si.X XOR sj.X
				den = gfMul(den, si.X^sj.X)
			}
			val ^= gfMul(si.Bytes[byteIdx], gfDiv(num, den))
		}
		secret[byteIdx] = val
	}
	return secret, nil
}

// ---------------------------------------------------------------------------
// Public API
// ---------------------------------------------------------------------------

// EmergencySetup splits masterKey into n shares with threshold t.
// At least t shares are required to reconstruct. All shares must be printed
// and stored offline by the operator. Nothing is retained by the server.
func EmergencySetup(masterKey []byte, n, t int) ([]Share, error) {
	if len(masterKey) == 0 {
		return nil, errors.New("master key must not be empty")
	}
	shares, err := splitSecret(masterKey, n, t)
	if err != nil {
		return nil, fmt.Errorf("split master key: %w", err)
	}
	return shares, nil
}

// ReconstructMasterKey combines shares and returns the master key.
// Requires at least 2 distinct shares; in practice you must provide at least
// as many as the threshold used during EmergencySetup. The returned key
// should be verified with VerifyMasterKey before use.
func ReconstructMasterKey(shares []Share) ([]byte, error) {
	if len(shares) < 2 {
		return nil, errors.New("need at least 2 shares")
	}
	// Deduplicate by X coordinate.
	seen := make(map[byte]bool)
	var deduped []Share
	for _, s := range shares {
		if !seen[s.X] {
			seen[s.X] = true
			deduped = append(deduped, s)
		}
	}
	if len(deduped) < 2 {
		return nil, errors.New("need at least 2 distinct shares after deduplication")
	}
	return combineShares(deduped)
}

// VerifyMasterKey checks a reconstructed key against the HMAC digest stored
// in the KMS provider. Returns nil if the key is correct.
func VerifyMasterKey(provider Provider, reconstructed []byte) error {
	storedDigest, err := provider.MasterKeyDigest()
	if err != nil {
		return fmt.Errorf("get digest: %w", err)
	}
	mac := hmac.New(sha256.New, reconstructed)
	mac.Write([]byte(erkDigestLabel))
	reconstructedDigest := mac.Sum(nil)[:len(storedDigest)]
	if subtle.ConstantTimeCompare(storedDigest, reconstructedDigest) != 1 {
		return errors.New("reconstructed key does not match stored digest — wrong shares or corruption")
	}
	return nil
}

// EmergencyRecover performs a full emergency recovery:
//  1. Reconstructs the old master key from shares.
//  2. Verifies it matches the stored digest.
//  3. Wipes all in-memory KMS state.
//  4. Generates a new master key.
//  5. Re-encrypts every DEK in the secrets store under the new key.
//  6. Splits the new master key into n fresh offline shares with threshold t.
//
// The old shares are dead after this call. The caller must distribute
// the returned shares to share holders immediately.
func EmergencyRecover(ctx context.Context, store SecretStore, provider Provider, shares []Share, n, t int) ([]Share, error) {
	// Step 1 & 2: reconstruct and verify.
	oldKey, err := ReconstructMasterKey(shares)
	if err != nil {
		return nil, fmt.Errorf("reconstruct: %w", err)
	}
	if err := VerifyMasterKey(provider, oldKey); err != nil {
		return nil, err
	}

	// Step 3: wipe in-memory state.
	provider.Wipe()

	// Step 4: new master key.
	newKey := make([]byte, len(oldKey))
	if _, err := rand.Read(newKey); err != nil {
		return nil, fmt.Errorf("generate new master key: %w", err)
	}

	// Step 5: re-encrypt all DEKs.
	if err := store.ReEncryptAllDEKs(ctx, oldKey, newKey); err != nil {
		// Attempt to restore old key so the system isn't left bricked.
		provider.LoadMasterKey(oldKey)
		return nil, fmt.Errorf("re-encrypt DEKs: %w", err)
	}

	// Load the new key into the provider.
	if err := provider.LoadMasterKey(newKey); err != nil {
		return nil, fmt.Errorf("load new master key: %w", err)
	}

	// Zero old key material.
	for i := range oldKey {
		oldKey[i] = 0
	}

	// Step 6: split new key into fresh shares.
	newShares, err := EmergencySetup(newKey, n, t)
	if err != nil {
		return nil, fmt.Errorf("split new master key: %w", err)
	}

	// Zero new key now that it's split.
	for i := range newKey {
		newKey[i] = 0
	}

	return newShares, nil
}

// ReissueShares reconstructs the master key from existing shares and
// immediately re-splits it into n fresh shares with threshold t. Use this
// when a share holder leaves or a share is suspected compromised. No KMS
// state is touched and no DEKs are re-encrypted.
func ReissueShares(provider Provider, shares []Share, n, t int) ([]Share, error) {
	key, err := ReconstructMasterKey(shares)
	if err != nil {
		return nil, fmt.Errorf("reconstruct for reissue: %w", err)
	}
	if err := VerifyMasterKey(provider, key); err != nil {
		return nil, err
	}
	newShares, err := EmergencySetup(key, n, t)
	if err != nil {
		return nil, fmt.Errorf("reissue split: %w", err)
	}
	for i := range key {
		key[i] = 0
	}
	return newShares, nil
}
