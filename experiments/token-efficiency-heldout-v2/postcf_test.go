package heldoutv2

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// The post-control-flow HowlFrame set (postcf/) is a separate results set:
// every task x language is verified with a current hash, the Python/JS/Go
// references are the baseline files, and the results are pinned below.
func TestHeldoutV2PostCFVerificationCoverage(t *testing.T) {
	var manifest struct {
		Tasks []struct {
			ID string `json:"id"`
		} `json:"tasks"`
	}
	readJSON(t, "tasks/manifest.json", &manifest)
	var results []struct {
		Task, Lang, SHA256 string
		Pass               bool
	}
	readJSON(t, "postcf/results/verification.json", &results)
	seen := map[string]bool{}
	for _, r := range results {
		key := r.Task + "/" + r.Lang
		if seen[key] {
			t.Errorf("duplicate %s", key)
		}
		seen[key] = true
		if !r.Pass {
			t.Errorf("%s did not pass", key)
		}
		path := filepath.Join("solutions", r.Lang, r.Task+"."+exts[r.Lang])
		if r.Lang == "howl" {
			path = filepath.Join("postcf", "solutions", "howl", r.Task+".howl")
		}
		src, err := os.ReadFile(path)
		if err != nil {
			t.Errorf("%s: %v", key, err)
			continue
		}
		if got := fmt.Sprintf("%x", sha256.Sum256(src)); got != r.SHA256 {
			t.Errorf("%s: stale hash", key)
		}
	}
	if want := len(manifest.Tasks) * len(exts); len(results) != want {
		t.Errorf("%d results, want %d (postcf has no gaps)", len(results), want)
	}
}

func TestHeldoutV2PostCFResultsFrozen(t *testing.T) {
	frozen := map[string]string{
		"postcf/results/compare.md":        "03020dcb9beb29976460b670a5331e1df009f5115c24699cd4503bf927078cbb",
		"postcf/results/summary.json":      "4553229c7ce800311b854394017b97b55cb62dbfb79aa39c82be3690930f03e1",
		"postcf/results/tokens.csv":        "acf707796bc89132c88a31918ec152d8e77843aec4ba818b7ccd8889c9bfcfee",
		"postcf/results/tokens.json":       "e10b5b43497db767c4fd4e77b6ddf97976854e4ce11c2a57b63899e5f210c08e",
		"postcf/results/verification.json": "11b1844f42e57b818c9cb3b31672a8ef8df72feddf0eaa7a723d9f570f5dff16",
	}
	for path, want := range frozen {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if got := fmt.Sprintf("%x", sha256.Sum256(data)); got != want {
			t.Errorf("%s changed after post-control-flow freeze: %s", path, got)
		}
	}
}
