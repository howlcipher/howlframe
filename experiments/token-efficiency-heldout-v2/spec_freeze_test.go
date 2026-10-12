package heldoutv2

import (
	"crypto/sha256"
	"fmt"
	"os"
	"testing"
)

// The frozen heldout-v2 spec files must match the hashes recorded at freeze time.
func TestHeldoutV2SpecsFrozen(t *testing.T) {
	frozen := map[string]string{
		"tasks/README.md":      "c2d9173ddf1ad4586d6bb02ed72b9197ce752f0fcd2e0ec0a3641ae8112be262",
		"tasks/build_specs.py": "a2ba3abc8c3151c6c27dd35200a5c79bd6a551de0b4e59cd66b5a35f5a773727",
		"tasks/manifest.json":  "8607a8d255115a225311208636e6563073aeace0e839a63b0e9f71aa17a467d7",
	}
	for path, want := range frozen {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if got := fmt.Sprintf("%x", sha256.Sum256(data)); got != want {
			t.Errorf("%s changed after freeze: %s", path, got)
		}
	}
}
