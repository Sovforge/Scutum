package tests

import (
	"context"
	"testing"
	"scutum/cmd/internal/kms"
)

func TestKMSRecovery(t *testing.T) {
	masterKey := []byte("this-is-a-32-byte-master-key-!!!")
	n, threshold := 5, 3

	t.Run("SplitAndCombine", func(t *testing.T) {
		shares, err := kms.EmergencySetup(masterKey, n, threshold)
		if err != nil {
			t.Fatalf("EmergencySetup failed: %v", err)
		}
		if len(shares) != n {
			t.Errorf("expected %d shares, got %d", n, len(shares))
		}

		// Reconstruct with exactly threshold
		reconstructed, err := kms.ReconstructMasterKey(shares[:threshold])
		if err != nil {
			t.Fatalf("Reconstruct failed: %v", err)
		}
		if string(reconstructed) != string(masterKey) {
			t.Error("reconstructed key mismatch")
		}

		// Reconstruct with more than threshold
		reconstructed2, err := kms.ReconstructMasterKey(shares)
		if err != nil {
			t.Fatalf("Reconstruct (all) failed: %v", err)
		}
		if string(reconstructed2) != string(masterKey) {
			t.Error("reconstructed (all) key mismatch")
		}
	})

	t.Run("InvalidParameters", func(t *testing.T) {
		_, err := kms.EmergencySetup(masterKey, 2, 3) // t > n
		if err == nil {
			t.Error("expected error for t > n")
		}
		_, err = kms.EmergencySetup(nil, 5, 3)
		if err == nil {
			t.Error("expected error for empty key")
		}
	})

	t.Run("ParseShare", func(t *testing.T) {
		shares, _ := kms.EmergencySetup(masterKey, 1, 1) // wait, n >= 2 is enforced in splitSecret?
		// Actually EmergencySetup uses splitSecret which requires t >= 2.
		shares, _ = kms.EmergencySetup(masterKey, 2, 2)
		s := shares[0]
		str := s.String()
		parsed, err := kms.ParseShare(str)
		if err != nil {
			t.Fatalf("ParseShare failed: %v", err)
		}
		if parsed.X != s.X || string(parsed.Bytes) != string(s.Bytes) {
			t.Error("parsed share mismatch")
		}

		// Invalid formats
		if _, err := kms.ParseShare("invalid"); err == nil {
			t.Error("expected error for invalid format")
		}
		if _, err := kms.ParseShare("scutum-erk-v2-1-base64"); err == nil {
			t.Error("expected error for wrong version")
		}
	})

	t.Run("VerifyMasterKey", func(t *testing.T) {
		mkms := &mockKMS{masterKey: masterKey}
		if err := kms.VerifyMasterKey(mkms, masterKey); err != nil {
			t.Fatalf("VerifyMasterKey failed: %v", err)
		}
		if err := kms.VerifyMasterKey(mkms, []byte("wrong-key")); err == nil {
			t.Error("expected error for wrong key")
		}
	})

	t.Run("EmergencyRecover", func(t *testing.T) {
		mkms := &mockKMS{masterKey: masterKey}
		mstore := &mockSecretStore{}
		shares, _ := kms.EmergencySetup(masterKey, 3, 2)

		newShares, err := kms.EmergencyRecover(context.Background(), mstore, mkms, shares[:2], 3, 2)
		if err != nil {
			t.Fatalf("EmergencyRecover failed: %v", err)
		}
		if !mstore.reEncrypted {
			t.Error("expected DEKs to be re-encrypted")
		}
		if len(newShares) != 3 {
			t.Errorf("expected 3 new shares, got %d", len(newShares))
		}
		// Verify we can reconstruct with new shares
		newKey, err := kms.ReconstructMasterKey(newShares[:2])
		if err != nil {
			t.Fatalf("Reconstruct new key failed: %v", err)
		}
		if string(newKey) == string(masterKey) {
			t.Error("expected new key to be different from old key")
		}
		if err := kms.VerifyMasterKey(mkms, newKey); err != nil {
			t.Errorf("Verify new key failed: %v", err)
		}
	})
}

