package heldoutv3

import (
	"crypto/sha256"
	"fmt"
	"os"
	"testing"
)

// The heldout-v3 baseline (HowlFrame sources using only features on main
// a6bb53b6, before the records slice) is frozen: the recorded gaps and
// results must match the hashes pinned here. Re-running run.sh reproduces
// these bytes.
func TestHeldoutV3BaselineResultsFrozen(t *testing.T) {
	frozen := map[string]string{
		"gaps.json":                 "37517e5f3dc66819f61f5a7bb8ace1921282415f10551d2defa5c3eb0985b570",
		"results/summary.json":      "a7cc74b4e1c454e66e3e0c24adc833599e0380d72cc9fc391253c14406f1950c",
		"results/tokens.csv":        "5204cbd4588c15aa2edd5ad484636bbbf3fff35c6bda44c5c07ba6e7cd6720e2",
		"results/tokens.json":       "507cd2686015598ec2578711292e07999b1af0ff59b06ef6a99ef7371c8fcd1e",
		"results/verification.json": "b050ab05d4672623f279c7d0f0c9f61838f2be65883bc8867c680333e54248a1",
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
