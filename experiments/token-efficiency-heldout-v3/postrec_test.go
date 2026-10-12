package heldoutv3

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// The post-records HowlFrame set (postrec/) is a separate results set:
// every task x language is verified with a current hash, the Python/JS/Go
// references are the baseline files, and the results are pinned below.
func TestHeldoutV3PostRecVerificationCoverage(t *testing.T) {
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
	readJSON(t, "postrec/results/verification.json", &results)
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
			path = filepath.Join("postrec", "solutions", "howl", r.Task+".howl")
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
		t.Errorf("%d results, want %d (postrec has no gaps)", len(results), want)
	}
}

func TestHeldoutV3PostRecResultsFrozen(t *testing.T) {
	frozen := map[string]string{
		"postrec/results/compare.md":        "5f817fa42130ccabe0d71c00dc232c09dd83dfd104039ea0d3d595c797fad0e1",
		"postrec/results/summary.json":      "fa2d3ea8db027043b6bfd3ab1cb08c8f21070be9c091385094d37a3c91d11273",
		"postrec/results/tokens.csv":        "b11929b5ef4b19bf7a0fb6b2b9790d6e0b95d3aaf8f642a812df52dfc8dfc108",
		"postrec/results/tokens.json":       "6b5f82a76e65408281b2ebc6b2f4acbcb23bbfede37c646fac3d7d52ad334c6d",
		"postrec/results/verification.json": "01de0278cd977f3cff115ac958a26104ceb3cf8c24cf9521aa9d5b04c545156c",
	}
	for path, want := range frozen {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if got := fmt.Sprintf("%x", sha256.Sum256(data)); got != want {
			t.Errorf("%s changed after post-records freeze: %s", path, got)
		}
	}
}
