package vm

import (
	"strings"

	"github.com/howlcipher/howlframe/internal/capability"

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

// capabilityGranted reports whether the runner granted a capability by its
// coarse name. A scoped grant ("filesystem:read=/x") counts for its family,
// except that "network:fetch=..." never counts as serving "network".
func capabilityGranted(caps []capability.Capability, name string) bool {
	// Scoped filesystem queries ask the same canonical containment question as
	// an actual read/write, without opening or creating the requested file.
	// This lets applications preflight several destinations before any mutation.
	if strings.HasPrefix(name, "filesystem:read=") {
		path := strings.TrimPrefix(name, "filesystem:read=")
		return path != "" && capability.Grants(caps).AllowsRead(path)
	}
	if strings.HasPrefix(name, "filesystem:write=") {
		path := strings.TrimPrefix(name, "filesystem:write=")
		return path != "" && capability.Grants(caps).AllowsWrite(path)
	}
	for _, c := range caps {
		s := string(c)
		if s == name {
			return true
		}
		if name != "network" && strings.HasPrefix(s, name+":") {
			return true
		}
	}
	return false
}
