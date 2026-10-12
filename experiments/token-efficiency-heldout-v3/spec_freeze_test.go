package heldoutv3

import (
	"crypto/sha256"
	"fmt"
	"os"
	"testing"
)

// The frozen heldout-v3 spec files must match the hashes recorded at freeze time.
func TestHeldoutV3SpecsFrozen(t *testing.T) {
	frozen := map[string]string{
		"tasks/README.md":      "309081e4062d432c95d649ca4642d477d0724f482acd15c6889b6532b4f8928c",
		"tasks/build_specs.py": "199d297eefc2286fbe05a30cfbf574f461cbbd24680290b669732ccb6372e2a1",
		"tasks/manifest.json":  "85bcde28428d27ce835efaf86d153f30094c3a70a43ae2be1f2f4eae6adcaa0c",
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
