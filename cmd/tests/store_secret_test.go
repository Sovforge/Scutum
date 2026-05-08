package tests

import (
	"context"
	"testing"
	"scutum/cmd/internal/store"
)

func TestStoreSecret(t *testing.T) {
	ctx := context.Background()

	t.Run("SetAndGet", func(t *testing.T) {
		st, _ := store.New(ctx, ":memory:", &mockKMS{})
		defer st.Close()

		key := "my-secret"
		val := []byte("top-secret-value")
		err := st.SetSecret(ctx, key, val)
		if err != nil {
			t.Fatalf("SetSecret failed: %v", err)
		}

		got, err := st.GetSecret(ctx, key)
		if err != nil {
			t.Fatalf("GetSecret failed: %v", err)
		}
		if string(got) != string(val) {
			t.Errorf("got %s, want %s", string(got), string(val))
		}
	})

	t.Run("ReEncryptAllDEKs", func(t *testing.T) {
		st, _ := store.New(ctx, ":memory:", &mockKMS{})
		defer st.Close()

		oldKey := make([]byte, 32)
		newKey := make([]byte, 32)
		for i := range oldKey { oldKey[i] = byte(i) }
		for i := range newKey { newKey[i] = byte(i + 1) }

		// ReEncryptAllDEKs on empty store should just work
		err := st.ReEncryptAllDEKs(ctx, oldKey, newKey)
		if err != nil {
			t.Fatalf("ReEncryptAllDEKs failed: %v", err)
		}
	})
}
