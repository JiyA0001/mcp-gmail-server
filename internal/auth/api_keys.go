package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
)

func GenerateAPIKey() (raw string, hash string, err error) {
	b := make([]byte, 32)
	_, err = rand.Read(b)
	if err != nil {
		return "", "", err
	}

	raw = "mcp_live_" + hex.EncodeToString(b)

	h := sha256.Sum256([]byte(raw))
	hash = hex.EncodeToString(h[:])

	return raw, hash, nil
}
