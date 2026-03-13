package utils

import (
	"crypto/sha256"
	"encoding/hex"
)

// HashSHA256 returns the SHA-256 hash of the input string as a hex string.
func HashSHA256(s string) string {
	h := sha256.New()
	h.Write([]byte(s))
	return hex.EncodeToString(h.Sum(nil))
}
