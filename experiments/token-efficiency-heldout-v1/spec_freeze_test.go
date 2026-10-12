package heldoutv1

import (
	"crypto/sha256"
	"fmt"
	"os"
	"testing"
)

// The frozen heldout-v1 spec files must match the hashes recorded at freeze
// time (docs/journals/2026-10-12_core_library_heldout_v1.md).
func TestHeldoutV1SpecsFrozen(t *testing.T) {
	frozen := map[string]string{
		"tasks/README.md":      "c7837434a99bac3f3289342ac77505a2aed74ec34b6316a19e74e5ad8eb2dd29",
		"tasks/build_specs.py": "7eb654f478aadebb316769d7757986352d96821828d78e8d93b58475245badd4",
		"tasks/manifest.json":  "b10c646bf5ed04f552b1ef28b5f29adb9a693501a66595cfc400043c6887af29",
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
