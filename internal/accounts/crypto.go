package accounts

import (
	"crypto/hmac"
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// randomToken returns 32 random bytes as URL-safe text.
func randomToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("random token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// hashToken is how a token is stored: the database never holds a usable one.
func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// hashPassword returns "pbkdf2-sha256$iterations$salt$hash".
func (s *Service) hashPassword(password string) (string, error) {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("salt: %w", err)
	}
	key, err := pbkdf2.Key(sha256.New, password, salt, s.iterations, 32)
	if err != nil {
		return "", fmt.Errorf("hash password: %w", err)
	}
	return fmt.Sprintf("pbkdf2-sha256$%d$%s$%s", s.iterations, hex.EncodeToString(salt), hex.EncodeToString(key)), nil
}

// verifyPassword compares in constant time and reads the cost from the stored hash.
func verifyPassword(password, stored string) bool {
	parts := strings.Split(stored, "$")
	if len(parts) != 4 || parts[0] != "pbkdf2-sha256" {
		return false
	}
	iterations, err := strconv.Atoi(parts[1])
	if err != nil || iterations <= 0 {
		return false
	}
	salt, err := hex.DecodeString(parts[2])
	if err != nil {
		return false
	}
	want, err := hex.DecodeString(parts[3])
	if err != nil {
		return false
	}
	got, err := pbkdf2.Key(sha256.New, password, salt, iterations, len(want))
	if err != nil {
		return false
	}
	return subtle.ConstantTimeCompare(got, want) == 1
}

type accessClaims struct {
	UserID    string `json:"uid"`
	SessionID string `json:"sid"`
	Workspace string `json:"ws"`
	ExpiresAt int64  `json:"exp"`
}

func (s *Service) signAccess(c accessClaims) (string, error) {
	payload, err := json.Marshal(c)
	if err != nil {
		return "", err
	}
	body := base64.RawURLEncoding.EncodeToString(payload)
	return body + "." + s.mac(body), nil
}

func (s *Service) verifyAccess(token string) (accessClaims, error) {
	body, sig, ok := strings.Cut(token, ".")
	if !ok || !hmac.Equal([]byte(sig), []byte(s.mac(body))) {
		return accessClaims{}, ErrTokenInvalid
	}
	payload, err := base64.RawURLEncoding.DecodeString(body)
	if err != nil {
		return accessClaims{}, ErrTokenInvalid
	}
	var c accessClaims
	if err := json.Unmarshal(payload, &c); err != nil {
		return accessClaims{}, ErrTokenInvalid
	}
	if c.ExpiresAt <= s.nowMillis() {
		return accessClaims{}, ErrTokenInvalid
	}
	return c, nil
}

func (s *Service) mac(body string) string {
	m := hmac.New(sha256.New, s.key)
	m.Write([]byte(body))
	return base64.RawURLEncoding.EncodeToString(m.Sum(nil))
}
