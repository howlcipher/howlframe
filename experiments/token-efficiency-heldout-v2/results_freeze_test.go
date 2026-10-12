package heldoutv2

import (
	"crypto/sha256"
	"fmt"
	"os"
	"testing"
)

// The heldout-v2 baseline on main 576c87c1 (before any control-flow change)
// is frozen: the recorded gaps and results must match the hashes pinned here.
// Re-running run.sh on the same commit reproduces these bytes.
func TestHeldoutV2BaselineResultsFrozen(t *testing.T) {
	frozen := map[string]string{
		"gaps.json":                 "37517e5f3dc66819f61f5a7bb8ace1921282415f10551d2defa5c3eb0985b570",
		"results/summary.json":      "bc27b0e6f6ae10285e2423d8fac0386b10ca0cd26421a397691b538714930370",
		"results/tokens.csv":        "a892a9018132e0f1d8acc3e2892c089259c906965a18bb4abce192ed9136fd3e",
		"results/tokens.json":       "499578ce5b2cef3923a3681a2ecbe0c74c4a2816b16bc2754da6ff1b5a8cfdf8",
		"results/verification.json": "a326ae813d783989b8c5a894bf6baabb7761c23b6f9dd49c7a719948d8527594",
	}
	for path, want := range frozen {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if got := fmt.Sprintf("%x", sha256.Sum256(data)); got != want {
			t.Errorf("%s changed after baseline freeze: %s", path, got)
		}
	}
}
