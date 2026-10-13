package livev1blind

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

// The frozen live-v1-blind spec files must match the hashes recorded at freeze time.
func TestLiveV1BlindSpecsFrozen(t *testing.T) {
	frozen := map[string]string{
		"README.md":      "ae77030f6474e7db611ce91405d735844967a2fcf19d715b5c0050600088cfd6",
		"build_specs.py": "b7bf3ccf185a8c12856e205605b3d6672489ca8205cec02dc403dcb6d3e6fc64",
		"manifest.json":  "7686087c2c7a5fb7f11fd4f3c93173d69f4593e87ced1b4e6dd6d248f3d73cf7",
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
	raw, err := os.ReadFile("manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	var m struct {
		TaskCount int               `json:"task_count"`
		Files     map[string]string `json:"files"`
	}
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	if m.TaskCount != 27 {
		t.Errorf("task_count = %d, want 27", m.TaskCount)
	}
	var extra []string
	err = filepath.WalkDir("tasks", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			if _, ok := m.Files[filepath.ToSlash(p)]; !ok {
				extra = append(extra, p)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range extra {
		t.Errorf("%s is under tasks/ but not in manifest.json", p)
	}
	for path, want := range m.Files {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if got := fmt.Sprintf("%x", sha256.Sum256(data)); got != want {
			t.Errorf("%s changed after freeze: %s", path, got)
		}
	}
}
