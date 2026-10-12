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
	// HowlFrame references were re-authored in current shorthand on 2026-10-12
	// (#134; sugar from #117, #118, #120, #124; compact built-ins later); they intentionally no longer copy reference/arm_a
	// verbatim. Guard against regressing to the expanded pre-shorthand forms.
	for _, task := range m.All() {
		data, err := os.ReadFile(filepath.Join("solutions/howl", task.ID+".howl"))
		if err != nil {
			t.Fatal(err)
		}
		if bytes.Contains(data, []byte("bytes_to_string")) {
			t.Errorf("%s uses expanded bytes_to_string; use read_text/lines shorthand", task.ID)
		}
	}
}
