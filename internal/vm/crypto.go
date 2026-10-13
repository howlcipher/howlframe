package vm

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

func sha256Hex(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

// randHex returns n (1..64) secure random bytes as lowercase hex.
func randHex(nAny any) (string, error) {
	var n int64
	switch v := nAny.(type) {
	case int64:
		n = v
	case int:
		n = int64(v)
	case float64:
		n = int64(v)
	default:
		return "", fmt.Errorf("n must be an integer, got %T", nAny)
	}
	if n < 1 || n > 64 {
		return "", fmt.Errorf("n must be 1..64, got %d", n)
	}
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}
