package controlplane

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

const (
	hashEncodingScheme = "pbkdf2-sha256"
	passwordIterations = 600_000
	passwordSaltBytes  = 16
	passwordKeyBytes   = 32
	minPasswordBytes   = 12
	maxPasswordBytes   = 256
)

var errInvalidPassword = errors.New("invalid email or password")

func hashPassword(password string) (string, error) {
	if len(password) < minPasswordBytes || len(password) > maxPasswordBytes {
		return "", fmt.Errorf("password must contain between %d and %d bytes", minPasswordBytes, maxPasswordBytes)
	}
	salt := make([]byte, passwordSaltBytes)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("generate password salt: %w", err)
	}
	key := derivePBKDF2([]byte(password), salt, passwordIterations, passwordKeyBytes)
	return strings.Join([]string{
		hashEncodingScheme,
		strconv.Itoa(passwordIterations),
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(key),
	}, "$"), nil
}

func verifyPassword(encoded, password string) bool {
	parts := strings.Split(encoded, "$")
	if len(parts) != 4 || parts[0] != hashEncodingScheme {
		return false
	}
	iterations, err := strconv.Atoi(parts[1])
	if err != nil || iterations < 100_000 || iterations > 2_000_000 {
		return false
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[2])
	if err != nil || len(salt) < 16 || len(salt) > 64 {
		return false
	}
	want, err := base64.RawStdEncoding.DecodeString(parts[3])
	if err != nil || len(want) != passwordKeyBytes {
		return false
	}
	got := derivePBKDF2([]byte(password), salt, iterations, len(want))
	return subtle.ConstantTimeCompare(got, want) == 1
}

func derivePBKDF2(password, salt []byte, iterations, keyBytes int) []byte {
	hashBytes := sha256.Size
	blocks := (keyBytes + hashBytes - 1) / hashBytes
	key := make([]byte, 0, blocks*hashBytes)
	for block := 1; block <= blocks; block++ {
		mac := hmac.New(sha256.New, password)
		_, _ = mac.Write(salt)
		_, _ = mac.Write([]byte{byte(block >> 24), byte(block >> 16), byte(block >> 8), byte(block)})
		u := mac.Sum(nil)
		result := append([]byte(nil), u...)
		for iteration := 1; iteration < iterations; iteration++ {
			mac.Reset()
			_, _ = mac.Write(u)
			u = mac.Sum(nil)
			for index := range result {
				result[index] ^= u[index]
			}
		}
		key = append(key, result...)
	}
	return key[:keyBytes]
}
