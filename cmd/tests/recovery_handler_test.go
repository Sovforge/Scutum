package tests

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"scutum/cmd/internal/handlers"
	"scutum/cmd/internal/kms"
	"scutum/cmd/internal/store"
)

// newRecoveryDeps creates a LocalKeyProvider and SQLite store for recovery tests.
// The provider's master key is exactly masterKey (32 bytes).
func newRecoveryDeps(t *testing.T, masterKey []byte) (*store.Store, *kms.LocalKeyProvider) {
	t.Helper()
	dir := t.TempDir()
	keyFile := filepath.Join(dir, "master.key")
	if err := os.WriteFile(keyFile, []byte(hex.EncodeToString(masterKey)+"\n"), 0600); err != nil {
		t.Fatalf("write key file: %v", err)
	}
	provider, err := kms.NewLocalKeyProvider(keyFile)
	if err != nil {
		t.Fatalf("NewLocalKeyProvider: %v", err)
	}
	s, err := store.New(context.Background(), filepath.Join(dir, "test.db"), provider)
	if err != nil {
		t.Fatalf("store.New: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s, provider
}

func knownMasterKey(seed byte) []byte {
	key := make([]byte, 32)
	for i := range key {
		key[i] = seed + byte(i)
	}
	return key
}

// makeShares splits masterKey into testN shares with testT threshold.
func makeShares(t *testing.T, masterKey []byte) []string {
	t.Helper()
	shares, err := kms.EmergencySetup(masterKey, testN, testT)
	if err != nil {
		t.Fatalf("EmergencySetup: %v", err)
	}
	out := make([]string, len(shares))
	for i, s := range shares {
		out[i] = s.String()
	}
	return out
}

// --- HandleGenerateShares ---

func TestHandleGenerateSharesInvalidJSON(t *testing.T) {
	s, p := newRecoveryDeps(t, knownMasterKey(1))
	h := handlers.NewRecoveryHandler(s, p)
	req := httptest.NewRequest(http.MethodPost, "/recovery/generate-shares", bytes.NewReader([]byte("{")))
	w := httptest.NewRecorder()
	h.HandleGenerateShares(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestHandleGenerateSharesSuccess(t *testing.T) {
	masterKey := knownMasterKey(4)
	s, p := newRecoveryDeps(t, masterKey)
	h := handlers.NewRecoveryHandler(s, p)

	body, _ := json.Marshal(map[string]int{"n_shares": testN, "threshold": testT})
	req := httptest.NewRequest(http.MethodPost, "/recovery/generate-shares", bytes.NewReader(body))
	w := httptest.NewRecorder()
	h.HandleGenerateShares(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp handlers.GenerateSharesResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(resp.Shares) != testN {
		t.Fatalf("expected %d shares, got %d", testN, len(resp.Shares))
	}
	for i, sh := range resp.Shares {
		if _, err := kms.ParseShare(sh); err != nil {
			t.Errorf("share[%d] not parseable: %v", i, err)
		}
	}
}

func TestHandleGenerateSharesDefaultParams(t *testing.T) {
	masterKey := knownMasterKey(5)
	s, p := newRecoveryDeps(t, masterKey)
	h := handlers.NewRecoveryHandler(s, p)

	// Empty body → handler should apply defaults (5 shares, threshold 3).
	body, _ := json.Marshal(map[string]int{})
	req := httptest.NewRequest(http.MethodPost, "/recovery/generate-shares", bytes.NewReader(body))
	w := httptest.NewRecorder()
	h.HandleGenerateShares(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp handlers.GenerateSharesResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if len(resp.Shares) != 5 {
		t.Errorf("expected 5 default shares, got %d", len(resp.Shares))
	}
}

func TestHandleGenerateSharesRandomised(t *testing.T) {
	masterKey := knownMasterKey(6)
	s, p := newRecoveryDeps(t, masterKey)
	h := handlers.NewRecoveryHandler(s, p)

	callShares := func() []string {
		body, _ := json.Marshal(map[string]int{"n_shares": testN, "threshold": testT})
		req := httptest.NewRequest(http.MethodPost, "/recovery/generate-shares", bytes.NewReader(body))
		w := httptest.NewRecorder()
		h.HandleGenerateShares(w, req)
		var resp handlers.GenerateSharesResponse
		json.NewDecoder(w.Body).Decode(&resp)
		return resp.Shares
	}

	a, b := callShares(), callShares()
	for i := range a {
		if a[i] != b[i] {
			return // good — at least one share differs
		}
	}
	t.Error("two calls with same key produced identical shares (shares should be randomised)")
}

// --- HandleReissueShares ---

func TestHandleReissueSharesInvalidJSON(t *testing.T) {
	s, p := newRecoveryDeps(t, knownMasterKey(7))
	h := handlers.NewRecoveryHandler(s, p)
	req := httptest.NewRequest(http.MethodPost, "/recovery/reissue-shares", bytes.NewReader([]byte("{")))
	w := httptest.NewRecorder()
	h.HandleReissueShares(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestHandleReissueSharesInsufficientShares(t *testing.T) {
	s, p := newRecoveryDeps(t, knownMasterKey(8))
	h := handlers.NewRecoveryHandler(s, p)
	// Only 1 share — below the minimum of 2.
	body, _ := json.Marshal(map[string][]string{"shares": {"a"}})
	req := httptest.NewRequest(http.MethodPost, "/recovery/reissue-shares", bytes.NewReader(body))
	w := httptest.NewRecorder()
	h.HandleReissueShares(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestHandleReissueSharesInvalidFormat(t *testing.T) {
	s, p := newRecoveryDeps(t, knownMasterKey(9))
	h := handlers.NewRecoveryHandler(s, p)
	body, _ := json.Marshal(map[string][]string{
		"shares": {"bad-1", "bad-2", "bad-3", "bad-4"},
	})
	req := httptest.NewRequest(http.MethodPost, "/recovery/reissue-shares", bytes.NewReader(body))
	w := httptest.NewRecorder()
	h.HandleReissueShares(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestHandleReissueSharesSuccess(t *testing.T) {
	masterKey := knownMasterKey(10)
	s, p := newRecoveryDeps(t, masterKey)
	h := handlers.NewRecoveryHandler(s, p)

	shareStrings := makeShares(t, masterKey)

	body, _ := json.Marshal(map[string]any{
		"shares":    shareStrings,
		"n_shares":  testN,
		"threshold": testT,
	})
	req := httptest.NewRequest(http.MethodPost, "/recovery/reissue-shares", bytes.NewReader(body))
	w := httptest.NewRecorder()
	h.HandleReissueShares(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp handlers.ReissueResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Status != "success" {
		t.Errorf("expected status=success, got %q", resp.Status)
	}
	if len(resp.NewShares) != testN {
		t.Errorf("expected %d new shares, got %d", testN, len(resp.NewShares))
	}
	for i, sh := range resp.NewShares {
		if _, err := kms.ParseShare(sh); err != nil {
			t.Errorf("new share[%d] not parseable: %v", i, err)
		}
	}
}

// --- HandleRecover ---

func TestHandleRecoverInvalidJSON(t *testing.T) {
	s, p := newRecoveryDeps(t, knownMasterKey(11))
	h := handlers.NewRecoveryHandler(s, p)
	req := httptest.NewRequest(http.MethodPost, "/recovery/recover", bytes.NewReader([]byte("{")))
	w := httptest.NewRecorder()
	h.HandleRecover(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestHandleRecoverInsufficientShares(t *testing.T) {
	s, p := newRecoveryDeps(t, knownMasterKey(12))
	h := handlers.NewRecoveryHandler(s, p)
	// Only 1 share — below the minimum of 2.
	body, _ := json.Marshal(map[string][]string{"shares": {"a"}})
	req := httptest.NewRequest(http.MethodPost, "/recovery/recover", bytes.NewReader(body))
	w := httptest.NewRecorder()
	h.HandleRecover(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestHandleRecoverInvalidShareFormat(t *testing.T) {
	s, p := newRecoveryDeps(t, knownMasterKey(13))
	h := handlers.NewRecoveryHandler(s, p)
	body, _ := json.Marshal(map[string][]string{
		"shares": {"bad-1", "bad-2", "bad-3", "bad-4"},
	})
	req := httptest.NewRequest(http.MethodPost, "/recovery/recover", bytes.NewReader(body))
	w := httptest.NewRecorder()
	h.HandleRecover(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestHandleRecoverSuccess(t *testing.T) {
	masterKey := knownMasterKey(14)
	s, p := newRecoveryDeps(t, masterKey)
	h := handlers.NewRecoveryHandler(s, p)

	shareStrings := makeShares(t, masterKey)

	body, _ := json.Marshal(map[string]any{
		"shares":    shareStrings,
		"n_shares":  testN,
		"threshold": testT,
	})
	req := httptest.NewRequest(http.MethodPost, "/recovery/recover", bytes.NewReader(body))
	w := httptest.NewRecorder()
	h.HandleRecover(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp handlers.RecoverResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Status != "success" {
		t.Errorf("expected status=success, got %q", resp.Status)
	}
	if len(resp.NewShares) != testN {
		t.Errorf("expected %d new shares, got %d", testN, len(resp.NewShares))
	}
}
