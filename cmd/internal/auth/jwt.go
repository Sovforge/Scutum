package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

type jwtHeader struct {
	Alg string `json:"alg"`
	Typ string `json:"typ"`
}

type jwtPayload struct {
	Sub      string `json:"sub"`
	Username string `json:"username"`
	Exp      int64  `json:"exp"`
	Iat      int64  `json:"iat"`
}

// IssueJWT creates a signed HS256 JWT for the given user.
func IssueJWT(userID, username string, secret []byte, ttl time.Duration) (string, error) {
	header, err := jsonBase64(jwtHeader{Alg: "HS256", Typ: "JWT"})
	if err != nil {
		return "", err
	}

	now := time.Now()
	payload, err := jsonBase64(jwtPayload{
		Sub:      userID,
		Username: username,
		Exp:      now.Add(ttl).Unix(),
		Iat:      now.Unix(),
	})
	if err != nil {
		return "", err
	}

	unsigned := header + "." + payload
	sig := signHS256([]byte(unsigned), secret)
	return unsigned + "." + sig, nil
}

// ValidateJWT parses and validates an HS256 JWT, returning its claims.
func ValidateJWT(token string, secret []byte) (Claims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return Claims{}, ErrInvalidToken
	}

	unsigned := parts[0] + "." + parts[1]
	expectedSig := signHS256([]byte(unsigned), secret)
	if !hmac.Equal([]byte(parts[2]), []byte(expectedSig)) {
		return Claims{}, ErrInvalidToken
	}

	payloadJSON, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return Claims{}, ErrInvalidToken
	}

	var payload jwtPayload
	if err := json.Unmarshal(payloadJSON, &payload); err != nil {
		return Claims{}, ErrInvalidToken
	}

	if time.Now().Unix() > payload.Exp {
		return Claims{}, ErrExpiredToken
	}

	return Claims{
		UserID:   payload.Sub,
		Username: payload.Username,
		Expires:  time.Unix(payload.Exp, 0),
	}, nil
}

func signHS256(data, secret []byte) string {
	h := hmac.New(sha256.New, secret)
	h.Write(data)
	return base64.RawURLEncoding.EncodeToString(h.Sum(nil))
}

func jsonBase64(v any) (string, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return "", fmt.Errorf("marshal jwt part: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
