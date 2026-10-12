package tokenefficiency

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/howlcipher/howlframe/experiments/falsifiable-v0/harness"
)

// results/verification.json must cover every task × language exactly once,
// every entry must pass, and each recorded hash must match the committed source.
func TestVerificationResultsCoverAndMatchSources(t *testing.T) {
	m, err := harness.LoadManifest("../falsifiable-v0")
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile("results/verification.json")
	if err != nil {
		t.Fatal(err)
	}
	var results []struct {
		Task   string `json:"task"`
		Lang   string `json:"lang"`
		Pass   bool   `json:"pass"`
		SHA256 string `json:"sha256"`
	}
	if err := json.Unmarshal(data, &results); err != nil {
		t.Fatal(err)
	}
	ext := map[string]string{"howl": "howl", "python": "py", "js": "js", "go": "go"}
	seen := map[string]bool{}
	for _, r := range results {
		key := r.Task + "/" + r.Lang
		if seen[key] {
			t.Errorf("duplicate entry %s", key)
		}
		seen[key] = true
		if !r.Pass {
			t.Errorf("%s did not pass", key)
		}
		e, ok := ext[r.Lang]
		if !ok {
			t.Errorf("unknown lang %s", key)
			continue
		}
		src, err := os.ReadFile(filepath.Join("solutions", r.Lang, r.Task+"."+e))
		if err != nil {
			t.Errorf("%s: %v", key, err)
			continue
		}
		if got := fmt.Sprintf("%x", sha256.Sum256(src)); got != r.SHA256 {
			t.Errorf("%s: stale hash %s, source is %s", key, r.SHA256, got)
		}
	}
	tasks := m.All()
	if want := len(tasks) * len(ext); len(results) != want || want != 240 {
		t.Errorf("verification has %d entries, want %d (60 tasks × 4 languages)", len(results), want)
	}
	for _, task := range tasks {
		for lang := range ext {
			if !seen[task.ID+"/"+lang] {
				t.Errorf("missing %s/%s", task.ID, lang)
			}
		}
	}
}
