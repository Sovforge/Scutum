package handlers

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sync"
	"time"

	gooidc "github.com/coreos/go-oidc/v3/oidc"
	"github.com/google/uuid"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/github"

	"scutum/cmd/internal/auth"
)

type ssoStore interface {
	UserByID(ctx context.Context, id string) (username string, err error)
	UserByEmail(ctx context.Context, email string) (id, username string, err error)
	UserBySSOIdentity(ctx context.Context, provider, subject string) (userID string, err error)
	CreateUserWithEmail(ctx context.Context, id, username, email string) error
	UpsertSSOIdentity(ctx context.Context, id, userID, provider, subject, email string) error
}

type SSOHandler struct {
	store     ssoStore
	jwtSecret []byte

	mu        sync.Mutex
	states    map[string]stateEntry
	providers []ssoProvider
}

type stateEntry struct {
	provider string
	expiry   time.Time
}

type ssoProvider struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Icon string `json:"icon"`
}

func NewSSOHandler(store ssoStore, jwtSecret []byte) *SSOHandler {
	h := &SSOHandler{
		store:     store,
		jwtSecret: jwtSecret,
		states:    make(map[string]stateEntry),
	}
	h.providers = buildProviders()
	return h
}

func buildProviders() []ssoProvider {
	var out []ssoProvider
	if os.Getenv("SSO_MICROSOFT_CLIENT_ID") != "" && os.Getenv("SSO_MICROSOFT_CLIENT_SECRET") != "" {
		out = append(out, ssoProvider{ID: "microsoft", Name: "Microsoft", Icon: "microsoft"})
	}
	if os.Getenv("SSO_GITHUB_CLIENT_ID") != "" && os.Getenv("SSO_GITHUB_CLIENT_SECRET") != "" {
		out = append(out, ssoProvider{ID: "github", Name: "GitHub", Icon: "github"})
	}
	if os.Getenv("SSO_AUTHENTIK_CLIENT_ID") != "" && os.Getenv("SSO_AUTHENTIK_CLIENT_SECRET") != "" {
		out = append(out, ssoProvider{ID: "authentik", Name: "Authentik", Icon: "authentik"})
	}
	if os.Getenv("SSO_KEYCLOAK_CLIENT_ID") != "" && os.Getenv("SSO_KEYCLOAK_CLIENT_SECRET") != "" {
		out = append(out, ssoProvider{ID: "keycloak", Name: "Keycloak", Icon: "keycloak"})
	}
	if os.Getenv("SSO_OIDC_CLIENT_ID") != "" && os.Getenv("SSO_OIDC_CLIENT_SECRET") != "" {
		name := os.Getenv("SSO_OIDC_NAME")
		if name == "" {
			name = "OIDC"
		}
		out = append(out, ssoProvider{ID: "oidc", Name: name, Icon: "oidc"})
	}
	return out
}

func (h *SSOHandler) HandleProviders(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if h.providers == nil {
		json.NewEncoder(w).Encode([]ssoProvider{})
		return
	}
	json.NewEncoder(w).Encode(h.providers)
}

func (h *SSOHandler) HandleLogin(w http.ResponseWriter, r *http.Request) {
	provider := r.PathValue("provider")
	cfg, err := h.oauth2Config(r.Context(), provider, r)
	if err != nil {
		http.Error(w, "unknown provider", http.StatusBadRequest)
		return
	}

	state := h.newState(provider)
	http.Redirect(w, r, cfg.AuthCodeURL(state, oauth2.AccessTypeOnline), http.StatusFound)
}

func (h *SSOHandler) HandleCallback(w http.ResponseWriter, r *http.Request) {
	provider := r.PathValue("provider")
	state := r.URL.Query().Get("state")
	code := r.URL.Query().Get("code")

	if !h.validateState(state, provider) {
		http.Error(w, "invalid state", http.StatusBadRequest)
		return
	}

	cfg, err := h.oauth2Config(r.Context(), provider, r)
	if err != nil {
		http.Error(w, "unknown provider", http.StatusBadRequest)
		return
	}

	token, err := cfg.Exchange(r.Context(), code)
	if err != nil {
		http.Error(w, "code exchange failed", http.StatusBadRequest)
		return
	}

	subject, email, name, err := h.userInfo(r.Context(), provider, cfg, token)
	if err != nil {
		http.Error(w, "failed to get user info", http.StatusInternalServerError)
		return
	}

	userID, username, err := h.resolveUser(r.Context(), provider, subject, email, name)
	if err != nil {
		http.Error(w, "failed to resolve user", http.StatusInternalServerError)
		return
	}

	jwt, err := auth.IssueJWT(userID, username, h.jwtSecret, 24*time.Hour)
	if err != nil {
		http.Error(w, "failed to issue token", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, fmt.Sprintf("/#sso-token=%s", jwt), http.StatusFound)
}

func (h *SSOHandler) resolveUser(ctx context.Context, provider, subject, email, name string) (userID, username string, err error) {
	userID, lookupErr := h.store.UserBySSOIdentity(ctx, provider, subject)
	if lookupErr == nil {
		username, err = h.store.UserByID(ctx, userID)
		return
	}
	if lookupErr != sql.ErrNoRows {
		err = lookupErr
		return
	}

	if email != "" {
		var existingID string
		existingID, username, lookupErr = h.store.UserByEmail(ctx, email)
		if lookupErr == nil {
			userID = existingID
			err = h.store.UpsertSSOIdentity(ctx, uuid.New().String(), userID, provider, subject, email)
			return
		}
		if lookupErr != sql.ErrNoRows {
			err = lookupErr
			return
		}
	}

	userID = uuid.New().String()
	username = usernameFromNameEmail(name, email, provider, subject)
	if err = h.store.CreateUserWithEmail(ctx, userID, username, email); err != nil {
		return
	}
	err = h.store.UpsertSSOIdentity(ctx, uuid.New().String(), userID, provider, subject, email)
	return
}

func usernameFromNameEmail(name, email, provider, subject string) string {
	if name != "" {
		return sanitizeUsername(name)
	}
	if email != "" {
		at := 0
		for i, c := range email {
			if c == '@' {
				at = i
				break
			}
		}
		if at > 0 {
			return sanitizeUsername(email[:at])
		}
		return sanitizeUsername(email)
	}
	l := len(subject)
	if l > 8 {
		l = 8
	}
	return provider + "_" + subject[:l]
}

func sanitizeUsername(s string) string {
	out := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_' || c == '-' || c == '.' {
			out = append(out, c)
		}
	}
	if len(out) == 0 {
		return "sso_user"
	}
	return string(out)
}

func (h *SSOHandler) userInfo(ctx context.Context, provider string, cfg *oauth2.Config, token *oauth2.Token) (subject, email, name string, err error) {
	if provider == "github" {
		return h.githubUserInfo(ctx, cfg, token)
	}
	return h.oidcUserInfo(ctx, provider, cfg, token)
}

func (h *SSOHandler) githubUserInfo(ctx context.Context, cfg *oauth2.Config, token *oauth2.Token) (subject, email, name string, err error) {
	client := cfg.Client(ctx, token)

	var userResp struct {
		ID    int64  `json:"id"`
		Login string `json:"login"`
		Name  string `json:"name"`
		Email string `json:"email"`
	}
	r, err := client.Get("https://api.github.com/user")
	if err != nil {
		return
	}
	defer r.Body.Close()
	if err = json.NewDecoder(r.Body).Decode(&userResp); err != nil {
		return
	}
	subject = fmt.Sprintf("%d", userResp.ID)
	name = userResp.Name
	if name == "" {
		name = userResp.Login
	}
	email = userResp.Email

	if email == "" {
		var emails []struct {
			Email   string `json:"email"`
			Primary bool   `json:"primary"`
		}
		er, err2 := client.Get("https://api.github.com/user/emails")
		if err2 == nil {
			defer er.Body.Close()
			if json.NewDecoder(er.Body).Decode(&emails) == nil {
				for _, e := range emails {
					if e.Primary {
						email = e.Email
						break
					}
				}
			}
		}
	}
	return
}

func (h *SSOHandler) oidcUserInfo(ctx context.Context, provider string, cfg *oauth2.Config, token *oauth2.Token) (subject, email, name string, err error) {
	issuerURL := issuerURLForProvider(provider)
	oidcProvider, err := gooidc.NewProvider(ctx, issuerURL)
	if err != nil {
		return
	}

	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok {
		err = fmt.Errorf("no id_token in response")
		return
	}

	verifier := oidcProvider.Verifier(&gooidc.Config{ClientID: cfg.ClientID})
	idToken, err := verifier.Verify(ctx, rawIDToken)
	if err != nil {
		return
	}

	var claims struct {
		Sub   string `json:"sub"`
		Email string `json:"email"`
		Name  string `json:"name"`
	}
	if err = idToken.Claims(&claims); err != nil {
		return
	}
	subject = claims.Sub
	email = claims.Email
	name = claims.Name
	return
}

func (h *SSOHandler) oauth2Config(ctx context.Context, provider string, r *http.Request) (*oauth2.Config, error) {
	redirectURI := redirectBase(r) + "/api/auth/sso/" + provider + "/callback"

	switch provider {
	case "microsoft":
		tenantID := os.Getenv("SSO_MICROSOFT_TENANT_ID")
		if tenantID == "" {
			tenantID = "common"
		}
		issuerURL := fmt.Sprintf("https://login.microsoftonline.com/%s/v2.0", tenantID)
		oidcProvider, err := gooidc.NewProvider(ctx, issuerURL)
		if err != nil {
			return nil, err
		}
		return &oauth2.Config{
			ClientID:     os.Getenv("SSO_MICROSOFT_CLIENT_ID"),
			ClientSecret: os.Getenv("SSO_MICROSOFT_CLIENT_SECRET"),
			Endpoint:     oidcProvider.Endpoint(),
			RedirectURL:  redirectURI,
			Scopes:       []string{gooidc.ScopeOpenID, "email", "profile"},
		}, nil

	case "github":
		return &oauth2.Config{
			ClientID:     os.Getenv("SSO_GITHUB_CLIENT_ID"),
			ClientSecret: os.Getenv("SSO_GITHUB_CLIENT_SECRET"),
			Endpoint:     github.Endpoint,
			RedirectURL:  redirectURI,
			Scopes:       []string{"read:user", "user:email"},
		}, nil

	case "authentik":
		issuerURL := os.Getenv("SSO_AUTHENTIK_ISSUER_URL")
		oidcProvider, err := gooidc.NewProvider(ctx, issuerURL)
		if err != nil {
			return nil, err
		}
		return &oauth2.Config{
			ClientID:     os.Getenv("SSO_AUTHENTIK_CLIENT_ID"),
			ClientSecret: os.Getenv("SSO_AUTHENTIK_CLIENT_SECRET"),
			Endpoint:     oidcProvider.Endpoint(),
			RedirectURL:  redirectURI,
			Scopes:       []string{gooidc.ScopeOpenID, "email", "profile"},
		}, nil

	case "keycloak":
		issuerURL := os.Getenv("SSO_KEYCLOAK_ISSUER_URL")
		oidcProvider, err := gooidc.NewProvider(ctx, issuerURL)
		if err != nil {
			return nil, err
		}
		return &oauth2.Config{
			ClientID:     os.Getenv("SSO_KEYCLOAK_CLIENT_ID"),
			ClientSecret: os.Getenv("SSO_KEYCLOAK_CLIENT_SECRET"),
			Endpoint:     oidcProvider.Endpoint(),
			RedirectURL:  redirectURI,
			Scopes:       []string{gooidc.ScopeOpenID, "email", "profile"},
		}, nil

	case "oidc":
		issuerURL := os.Getenv("SSO_OIDC_ISSUER_URL")
		oidcProvider, err := gooidc.NewProvider(ctx, issuerURL)
		if err != nil {
			return nil, err
		}
		return &oauth2.Config{
			ClientID:     os.Getenv("SSO_OIDC_CLIENT_ID"),
			ClientSecret: os.Getenv("SSO_OIDC_CLIENT_SECRET"),
			Endpoint:     oidcProvider.Endpoint(),
			RedirectURL:  redirectURI,
			Scopes:       []string{gooidc.ScopeOpenID, "email", "profile"},
		}, nil

	default:
		return nil, fmt.Errorf("unknown provider: %s", provider)
	}
}

func issuerURLForProvider(provider string) string {
	switch provider {
	case "microsoft":
		tenantID := os.Getenv("SSO_MICROSOFT_TENANT_ID")
		if tenantID == "" {
			tenantID = "common"
		}
		return fmt.Sprintf("https://login.microsoftonline.com/%s/v2.0", tenantID)
	case "authentik":
		return os.Getenv("SSO_AUTHENTIK_ISSUER_URL")
	case "keycloak":
		return os.Getenv("SSO_KEYCLOAK_ISSUER_URL")
	default:
		return os.Getenv("SSO_OIDC_ISSUER_URL")
	}
}

func redirectBase(r *http.Request) string {
	if base := os.Getenv("SSO_REDIRECT_BASE_URL"); base != "" {
		return base
	}
	scheme := "https"
	if r.TLS == nil {
		scheme = "http"
	}
	return scheme + "://" + r.Host
}

func (h *SSOHandler) newState(provider string) string {
	b := make([]byte, 32)
	rand.Read(b)
	state := base64.RawURLEncoding.EncodeToString(b)

	h.mu.Lock()
	defer h.mu.Unlock()
	now := time.Now()
	for k, v := range h.states {
		if now.After(v.expiry) {
			delete(h.states, k)
		}
	}
	h.states[state] = stateEntry{provider: provider, expiry: now.Add(10 * time.Minute)}
	return state
}

func (h *SSOHandler) validateState(state, provider string) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	entry, ok := h.states[state]
	if !ok {
		return false
	}
	delete(h.states, state)
	return entry.provider == provider && time.Now().Before(entry.expiry)
}
