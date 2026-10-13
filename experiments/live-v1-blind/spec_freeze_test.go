package livev1blind

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"testing"
)

// The frozen live-v1-blind spec files must match the hashes recorded at freeze time.
func TestLiveV1BlindSpecsFrozen(t *testing.T) {
	frozen := map[string]string{
		"README.md":      "aafd89dd5bdc92bd6f5140b6a0b243b7483b20cfa2a40353f73dedf4e77990ee",
		"build_specs.py": "b2059a215777504bffa03670df698b4cf0c1587e5320040b2cad0ef9781495ad",
		"manifest.json":  "b4e525207b1b46b978415c92f54fdd6cc5eb23375506345512737d2f8b132fde",
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
