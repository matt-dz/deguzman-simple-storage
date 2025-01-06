package api

import (
	"crypto/rand"
	"encoding/base64"
)

func generateRandomString(n int) (string, error) {
	// Calculate the number of bytes required to encode `n` characters in base64
	// Base64 encoding expands every 3 bytes into 4 characters.
	byteLength := (n*3 + 3) / 4

	randomBytes := make([]byte, byteLength)
	if _, err := rand.Read(randomBytes); err != nil {
		return "", err
	}
	encoded := base64.RawURLEncoding.EncodeToString(randomBytes)

	return encoded[:n], nil
}
