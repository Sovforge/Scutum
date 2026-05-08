package handlers

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"time"

	"github.com/skip2/go-qrcode"

	"scutum/cmd/internal/auth"

	"github.com/google/uuid"
)

type authStore interface {
	CreateUser(ctx context.Context, id, username, passwordHash string) error
	UserByUsername(ctx context.Context, username string) (id, passwordHash string, err error)
	UpdateUserPassword(ctx context.Context, id, passwordHash string) error
	CreateAPIKey(ctx context.Context, id, userID, name, keyHash string, expiresAt *time.Time) error
	GetUserTOTP(ctx context.Context, userID string) (secret string, enabled bool, err error)
	SetUserTOTPSecret(ctx context.Context, userID, secret string) error
	SetUserTOTPEnabled(ctx context.Context, userID string, enabled bool) error
	CreateRecoveryCodes(ctx context.Context, userID string, codeHashes []string) error
	UseRecoveryCode(ctx context.Context, userID, codeHash string) error
	CountRemainingRecoveryCodes(ctx context.Context, userID string) (int, error)
}

type AuthHandler struct {
	store     authStore
	jwtSecret []byte
}

func NewAuthHandler(store authStore, jwtSecret []byte) *AuthHandler {
	return &AuthHandler{store: store, jwtSecret: jwtSecret}
}

type registerRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
	TOTPCode string `json:"totp_code,omitempty"`
}

type createKeyRequest struct {
	Name      string `json:"name"`
	ExpiresAt string `json:"expires_at"` // RFC3339, optional
}

func (h *AuthHandler) HandleRegister(w http.ResponseWriter, r *http.Request) {
	var req registerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	if req.Username == "" || req.Password == "" {
		http.Error(w, "username and password are required", http.StatusBadRequest)
		return
	}
	if len(req.Password) < 12 {
		http.Error(w, "password must be at least 12 characters", http.StatusBadRequest)
		return
	}

	base := NewBaseHandler(nil)
	base.Audit("USER_REGISTER", r, "username", req.Username)

	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		http.Error(w, "failed to hash password", http.StatusInternalServerError)
		return
	}

	id := uuid.New().String()
	if err := h.store.CreateUser(r.Context(), id, req.Username, hash); err != nil {
		http.Error(w, "username already exists", http.StatusConflict)
		return
	}

	// Generate recovery codes and store their hashes.
	plainCodes := make([]string, auth.RecoveryCodeCount)
	hashes := make([]string, auth.RecoveryCodeCount)
	for i := range plainCodes {
		plain, hashed, err := auth.GenerateRecoveryCode()
		if err != nil {
			http.Error(w, "failed to generate recovery codes", http.StatusInternalServerError)
			return
		}
		plainCodes[i] = plain
		hashes[i] = hashed
	}
	if err := h.store.CreateRecoveryCodes(r.Context(), id, hashes); err != nil {
		http.Error(w, "failed to store recovery codes", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"id":             id,
		"username":       req.Username,
		"recovery_codes": plainCodes,
	})
}

func (h *AuthHandler) HandleLogin(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	if req.Username == "" || req.Password == "" {
		http.Error(w, "username and password are required", http.StatusBadRequest)
		return
	}

	id, hash, err := h.store.UserByUsername(r.Context(), req.Username)
	if err != nil {
		base := NewBaseHandler(nil)
		base.Audit("LOGIN_FAILED", r, "username", req.Username, "reason", "user_not_found")
		http.Error(w, "invalid credentials", http.StatusUnauthorized)
		return
	}

	ok, err := auth.VerifyPassword(req.Password, hash)
	if err != nil || !ok {
		base := NewBaseHandler(nil)
		base.Audit("LOGIN_FAILED", r, "username", req.Username, "reason", "invalid_password")
		http.Error(w, "invalid credentials", http.StatusUnauthorized)
		return
	}

	// Check TOTP if enabled for this user.
	totpSecret, totpEnabled, _ := h.store.GetUserTOTP(r.Context(), id)
	if totpEnabled {
		if req.TOTPCode == "" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"error":         "totp_required",
				"totp_required": true,
			})
			return
		}
		if !auth.VerifyTOTP(totpSecret, req.TOTPCode) {
			base := NewBaseHandler(nil)
			base.Audit("LOGIN_FAILED", r, "username", req.Username, "reason", "invalid_totp")
			http.Error(w, "invalid credentials", http.StatusUnauthorized)
			return
		}
	}

	token, err := auth.IssueJWT(id, req.Username, h.jwtSecret, 24*time.Hour)
	if err != nil {
		http.Error(w, "failed to issue token", http.StatusInternalServerError)
		return
	}

	base := NewBaseHandler(nil)
	base.Audit("LOGIN_SUCCESS", r, "username", req.Username, "user_id", id)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"token": token})
}

func (h *AuthHandler) HandleCreateAPIKey(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var req createKeyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	if req.Name == "" {
		http.Error(w, "name is required", http.StatusBadRequest)
		return
	}

	var expiresAt *time.Time
	if req.ExpiresAt != "" {
		t, err := time.Parse(time.RFC3339, req.ExpiresAt)
		if err != nil {
			http.Error(w, "invalid expires_at format, use RFC3339", http.StatusBadRequest)
			return
		}
		expiresAt = &t
	}

	rawKey, err := auth.GenerateAPIKey()
	if err != nil {
		http.Error(w, "failed to generate key", http.StatusInternalServerError)
		return
	}

	id := uuid.New().String()
	if err := h.store.CreateAPIKey(r.Context(), id, claims.UserID, req.Name,
		auth.HashAPIKey(rawKey), expiresAt); err != nil {
		http.Error(w, "failed to store key", http.StatusInternalServerError)
		return
	}

	base := NewBaseHandler(nil)
	base.Audit("API_KEY_CREATED", r,
		"key_id", id,
		"key_name", req.Name,
		"user_id", claims.UserID)

	// Return the raw key once — it cannot be retrieved again
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{
		"id":  id,
		"key": rawKey,
	})
}

// HandleMFAStatus returns whether TOTP is enabled for the current user.
func (h *AuthHandler) HandleMFAStatus(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	_, enabled, err := h.store.GetUserTOTP(r.Context(), claims.UserID)
	if err != nil {
		http.Error(w, "failed to get mfa status", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"enabled": enabled})
}

// HandleMFASetup generates a new TOTP secret and returns the secret and URI.
// The secret is stored but MFA is NOT yet enabled — the user must confirm with
// a valid code via HandleMFAEnable first.
func (h *AuthHandler) HandleMFASetup(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	secret, err := auth.GenerateTOTPSecret()
	if err != nil {
		http.Error(w, "failed to generate secret", http.StatusInternalServerError)
		return
	}

	if err := h.store.SetUserTOTPSecret(r.Context(), claims.UserID, secret); err != nil {
		http.Error(w, "failed to store secret", http.StatusInternalServerError)
		return
	}

	uri := auth.TOTPUri(secret, claims.Username, "Scutum")

	var pngBytes []byte
	pngBytes, err = qrcode.Encode(uri, qrcode.Medium, 256)
	if err != nil {
		http.Error(w, "Failed to generate QR code", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"secret":  secret,
		"uri":     uri,
		"qr_code": base64.StdEncoding.EncodeToString(pngBytes),
	})
}

// HandleMFAEnable verifies a TOTP code against the pending secret and enables MFA.
func (h *AuthHandler) HandleMFAEnable(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var req struct {
		Code string `json:"code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Code == "" {
		http.Error(w, "code is required", http.StatusBadRequest)
		return
	}

	secret, _, err := h.store.GetUserTOTP(r.Context(), claims.UserID)
	if err != nil || secret == "" {
		http.Error(w, "no pending mfa setup — call /auth/mfa/setup first", http.StatusBadRequest)
		return
	}

	if !auth.VerifyTOTP(secret, req.Code) {
		http.Error(w, "invalid code", http.StatusUnprocessableEntity)
		return
	}

	if err := h.store.SetUserTOTPEnabled(r.Context(), claims.UserID, true); err != nil {
		http.Error(w, "failed to enable mfa", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"enabled": true})
}

// HandleMFADisable verifies a TOTP code then disables MFA.
func (h *AuthHandler) HandleMFADisable(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var req struct {
		Code string `json:"code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Code == "" {
		http.Error(w, "code is required", http.StatusBadRequest)
		return
	}

	secret, enabled, err := h.store.GetUserTOTP(r.Context(), claims.UserID)
	if err != nil {
		http.Error(w, "failed to get mfa status", http.StatusInternalServerError)
		return
	}
	if !enabled {
		http.Error(w, "mfa is not enabled", http.StatusBadRequest)
		return
	}

	if !auth.VerifyTOTP(secret, req.Code) {
		http.Error(w, "invalid code", http.StatusUnprocessableEntity)
		return
	}

	if err := h.store.SetUserTOTPEnabled(r.Context(), claims.UserID, false); err != nil {
		http.Error(w, "failed to disable mfa", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"enabled": false})
}

// HandleForgotPassword resets a user's password using either a recovery code or
// (if MFA is enabled) a valid TOTP code.
// POST /auth/forgot-password
// Body: { username, new_password, recovery_code? | totp_code? }
func (h *AuthHandler) HandleForgotPassword(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username     string `json:"username"`
		NewPassword  string `json:"new_password"`
		RecoveryCode string `json:"recovery_code,omitempty"`
		TOTPCode     string `json:"totp_code,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	if req.Username == "" || req.NewPassword == "" {
		http.Error(w, "username and new_password are required", http.StatusBadRequest)
		return
	}
	if len(req.NewPassword) < 12 {
		http.Error(w, "password must be at least 12 characters", http.StatusBadRequest)
		return
	}
	if req.RecoveryCode == "" && req.TOTPCode == "" {
		http.Error(w, "recovery_code or totp_code is required", http.StatusBadRequest)
		return
	}

	userID, _, err := h.store.UserByUsername(r.Context(), req.Username)
	if err != nil {
		// Return the same error regardless to avoid username enumeration.
		http.Error(w, "invalid credentials", http.StatusUnauthorized)
		return
	}

	if req.RecoveryCode != "" {
		codeHash := auth.HashRecoveryCode(req.RecoveryCode)
		if err := h.store.UseRecoveryCode(r.Context(), userID, codeHash); err != nil {
			base := NewBaseHandler(nil)
			base.Audit("PASSWORD_RESET_FAILED", r, "username", req.Username, "reason", "invalid_recovery_code")
			http.Error(w, "invalid credentials", http.StatusUnauthorized)
			return
		}
	} else {
		totpSecret, totpEnabled, _ := h.store.GetUserTOTP(r.Context(), userID)
		if !totpEnabled {
			http.Error(w, "MFA is not enabled for this account; use a recovery code", http.StatusBadRequest)
			return
		}
		if !auth.VerifyTOTP(totpSecret, req.TOTPCode) {
			base := NewBaseHandler(nil)
			base.Audit("PASSWORD_RESET_FAILED", r, "username", req.Username, "reason", "invalid_totp")
			http.Error(w, "invalid credentials", http.StatusUnauthorized)
			return
		}
	}

	newHash, err := auth.HashPassword(req.NewPassword)
	if err != nil {
		http.Error(w, "failed to hash password", http.StatusInternalServerError)
		return
	}
	if err := h.store.UpdateUserPassword(r.Context(), userID, newHash); err != nil {
		http.Error(w, "failed to update password", http.StatusInternalServerError)
		return
	}

	base := NewBaseHandler(nil)
	base.Audit("PASSWORD_RESET", r, "username", req.Username)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"message": "password updated"})
}

// HandleRecoveryCodeStatus returns the number of unused recovery codes for the current user.
// GET /auth/recovery-codes
func (h *AuthHandler) HandleRecoveryCodeStatus(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	count, err := h.store.CountRemainingRecoveryCodes(r.Context(), claims.UserID)
	if err != nil {
		http.Error(w, "failed to count codes", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]int{"remaining": count})
}

// HandleRegenerateRecoveryCodes generates a fresh set of recovery codes,
// invalidating all existing ones. Returns the new plaintext codes once.
// POST /auth/recovery-codes/regenerate
func (h *AuthHandler) HandleRegenerateRecoveryCodes(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	plainCodes := make([]string, auth.RecoveryCodeCount)
	hashes := make([]string, auth.RecoveryCodeCount)
	for i := range plainCodes {
		plain, hashed, err := auth.GenerateRecoveryCode()
		if err != nil {
			http.Error(w, "failed to generate recovery codes", http.StatusInternalServerError)
			return
		}
		plainCodes[i] = plain
		hashes[i] = hashed
	}
	if err := h.store.CreateRecoveryCodes(r.Context(), claims.UserID, hashes); err != nil {
		http.Error(w, "failed to store recovery codes", http.StatusInternalServerError)
		return
	}

	base := NewBaseHandler(nil)
	base.Audit("RECOVERY_CODES_REGENERATED", r, "user_id", claims.UserID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"recovery_codes": plainCodes})
}
