package heldout

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

var exts = map[string]string{"howl": "howl", "python": "py", "js": "js", "go": "go"}

func readJSON(t *testing.T, path string, v any) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, v); err != nil {
		t.Fatal(err)
	}
}

// Every task × language is either verified (pass, matching hash) or an
// explicitly recorded HowlFrame gap with a reason and no solution file.
func TestHeldoutVerificationCoverage(t *testing.T) {
	var manifest struct {
		Tasks []struct {
			ID string `json:"id"`
		} `json:"tasks"`
	}
	readJSON(t, "tasks/manifest.json", &manifest)
	var gaps []struct{ Task, Lang, Reason string }
	readJSON(t, "gaps.json", &gaps)
	var results []struct {
		Task, Lang, SHA256 string
		Pass               bool
	}
	readJSON(t, "results/verification.json", &results)
	if len(manifest.Tasks) < 30 {
		t.Fatalf("held-out corpus has %d tasks, want >= 30", len(manifest.Tasks))
	}
	gapped := map[string]bool{}
	for _, g := range gaps {
		if g.Lang != "howl" || g.Reason == "" {
			t.Errorf("gap %s/%s must be a HowlFrame gap with a reason", g.Task, g.Lang)
		}
		gapped[g.Task+"/"+g.Lang] = true
		if _, err := os.Stat(filepath.Join("solutions", g.Lang, g.Task+"."+exts[g.Lang])); err == nil {
			t.Errorf("gap %s/%s has a solution file", g.Task, g.Lang)
		}
	}
	seen := map[string]bool{}
	for _, r := range results {
		key := r.Task + "/" + r.Lang
		if seen[key] || gapped[key] {
			t.Errorf("duplicate or gapped entry %s", key)
		}
		seen[key] = true
		if !r.Pass {
			t.Errorf("%s did not pass", key)
		}
		src, err := os.ReadFile(filepath.Join("solutions", r.Lang, r.Task+"."+exts[r.Lang]))
		if err != nil {
			t.Errorf("%s: %v", key, err)
			continue
		}
		if got := fmt.Sprintf("%x", sha256.Sum256(src)); got != r.SHA256 {
			t.Errorf("%s: stale hash", key)
		}
	}
	for _, task := range manifest.Tasks {
		for lang := range exts {
			key := task.ID + "/" + lang
			if !seen[key] && !gapped[key] {
				t.Errorf("missing %s", key)
			}
		}
	}
	if want := len(manifest.Tasks)*len(exts) - len(gaps); len(results) != want {
		t.Errorf("%d results, want %d", len(results), want)
	}
}

// The frozen spec files must match the hashes recorded at freeze time.
func TestHeldoutSpecsFrozen(t *testing.T) {
	frozen := map[string]string{
		"tasks/README.md":      "4441e1e5c4e906a55fe595c382a7d2ef784045e9c1b1230f38887d70bf8318f1",
		"tasks/build_specs.py": "dcea7d494d8b146ae032456b1f8822364d488c3d8596f7d829aecb21a75148f8",
		"tasks/manifest.json":  "962fa8198b11ac3ff1066abf35be6dcb87ad8a852574edfc46fee17a0ab9e67e",
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
