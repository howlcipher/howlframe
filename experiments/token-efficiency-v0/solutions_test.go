package tokenefficiency

import (
	"bytes"
	"github.com/howlcipher/howlframe/experiments/falsifiable-v0/harness"
	"os"
	"path/filepath"
	"testing"
)

func TestSolutionCoverage(t *testing.T) {
	m, err := harness.LoadManifest("../falsifiable-v0")
	if err != nil {
		t.Fatal(err)
	}
	for _, task := range m.All() {
		for lang, ext := range map[string]string{"howl": "howl", "python": "py", "js": "js", "go": "go"} {
			path := filepath.Join("solutions", lang, task.ID+"."+ext)
			data, err := os.ReadFile(path)
			if err != nil || len(bytes.TrimSpace(data)) == 0 {
				t.Errorf("missing/empty solution %s: %v", path, err)
			}
		}
	}
	for _, id := range []string{"t01", "t02", "t03", "a01", "a02", "a03"} {
		got, err := os.ReadFile(filepath.Join("solutions/howl", id+".howl"))
		if err != nil {
			t.Fatal(err)
		}
		want, err := os.ReadFile(filepath.Join("../falsifiable-v0/reference/arm_a", id+".howl"))
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(got, want) {
			t.Errorf("existing reference changed: %s", id)
		}
	}
}
