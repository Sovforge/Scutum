package tests

import (
	"encoding/base32"
	"strings"
	"testing"
	"scutum/cmd/internal/auth"
)

func TestTOTP(t *testing.T) {
	secret, err := auth.GenerateTOTPSecret()
	if err != nil {
		t.Fatalf("GenerateTOTPSecret failed: %v", err)
	}

	if secret == "" {
		t.Fatal("expected non-empty secret")
	}

	// Verify it's valid base32
	padded := secret
	for len(padded)%8 != 0 {
		padded += "="
	}
	_, err = base32.StdEncoding.DecodeString(strings.ToUpper(padded))
	if err != nil {
		t.Fatalf("invalid base32 secret: %v", err)
	}

	// Test VerifyTOTP with invalid codes
	if auth.VerifyTOTP(secret, "000000") {
		t.Error("expected VerifyTOTP to fail for random code")
	}
}

func TestTOTPUri(t *testing.T) {
	secret := "JBSWY3DPEHPK3PXP"
	username := "alice@example.com"
	issuer := "Scutum"
	uri := auth.TOTPUri(secret, username, issuer)
	
	if !strings.Contains(uri, "otpauth://totp/") {
		t.Error("missing prefix")
	}
	if !strings.Contains(uri, "secret="+secret) {
		t.Error("missing secret")
	}
}
