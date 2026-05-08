package tests

import (
	"context"
	"testing"
	"scutum/cmd/internal/store"
)

func TestStoreRecoveryCodes(t *testing.T) {
	st, err := store.New(context.Background(), ":memory:", &mockKMS{})
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer st.Close()

	ctx := context.Background()
	userID := "user-123"

	t.Run("CreateAndUse", func(t *testing.T) {
		// Create user first because of FK
		err := st.CreateUser(ctx, userID, "testuser", "hash")
		if err != nil {
			t.Fatalf("failed to create user: %v", err)
		}

		codes := []string{"code1", "code2", "code3"}
		err = st.CreateRecoveryCodes(ctx, userID, codes)
		if err != nil {
			t.Fatalf("CreateRecoveryCodes failed: %v", err)
		}



		count, _ := st.CountRemainingRecoveryCodes(ctx, userID)
		if count != 3 {
			t.Errorf("expected 3 codes, got %d", count)
		}

		// Use a code
		err = st.UseRecoveryCode(ctx, userID, "code1")
		if err != nil {
			t.Fatalf("UseRecoveryCode failed: %v", err)
		}

		count, _ = st.CountRemainingRecoveryCodes(ctx, userID)
		if count != 2 {
			t.Errorf("expected 2 codes, got %d", count)
		}

		// Use same code again
		err = st.UseRecoveryCode(ctx, userID, "code1")
		if err == nil {
			t.Error("expected error when using already used code")
		}

		// Use invalid code
		err = st.UseRecoveryCode(ctx, userID, "invalid")
		if err == nil {
			t.Error("expected error when using invalid code")
		}
	})
}
