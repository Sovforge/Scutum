package tests

import (
	"testing"
	"scutum/cmd/internal/utils"
)

func TestWireGuardUtils(t *testing.T) {
	t.Run("DerivePublicKey", func(t *testing.T) {
		// Valid 32-byte base64 encoded private key
		priv := "GAs/80L9fL8fL8fL8fL8fL8fL8fL8fL8fL8fL8fL8f8=" 
		pub, err := utils.DerivePublicKey(priv)
		if err != nil {
			t.Fatalf("DerivePublicKey failed: %v", err)
		}
		if pub == "" {
			t.Error("expected non-empty public key")
		}
	})

	t.Run("DerivePublicKey-Invalid", func(t *testing.T) {
		_, err := utils.DerivePublicKey("too-short")
		if err == nil {
			t.Error("expected error for invalid key")
		}
	})
}
