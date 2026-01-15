package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"net/http"
	"strings"
)

// GenerateAPIKey generates a new random API key
// Returns the plaintext key (to show user once) and its hash (to store)
func GenerateAPIKey() (plaintext, hash string, err error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", "", err
	}

	plaintext = base64.RawURLEncoding.EncodeToString(buf)
	hash = HashAPIKey(plaintext)
	return plaintext, hash, nil
}

// HashAPIKey creates a SHA-256 hash of an API key for storage
func HashAPIKey(key string) string {
	h := sha256.Sum256([]byte(key))
	return hex.EncodeToString(h[:])
}

// GetAPIKeyFromRequest extracts API key from Authorization header
// Expects format: "Bearer <key>"
func GetAPIKeyFromRequest(r *http.Request) string {
	auth := r.Header.Get("Authorization")
	if auth == "" {
		return ""
	}

	parts := strings.SplitN(auth, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return ""
	}

	return strings.TrimSpace(parts[1])
}
