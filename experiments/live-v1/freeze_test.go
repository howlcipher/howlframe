// Package livev1 holds the pre-registered live-model comparison protocol.
package livev1

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"sort"
	"testing"
	"time"

	"github.com/howlcipher/howlframe/experiments/falsifiable-v0/harness"
)

// Every protocol input (tasks, oracles, schema, prompts, rendered prompts)
// must match pins.json, recorded at freeze time. Do not regenerate pins after
// any live run has started.
func TestLiveV1ProtocolFrozen(t *testing.T) {
	got, err := harness.ComputeLivePins(".", "../falsifiable-v0")
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile("pins.json")
	if err != nil {
		t.Fatal(err)
	}
	var want map[string]string
	if err := json.Unmarshal(b, &want); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		for k, v := range got {
			if want[k] != v {
				t.Errorf("changed after freeze: %s", k)
			}
		}
		for k := range want {
			if _, ok := got[k]; !ok {
				t.Errorf("missing after freeze: %s", k)
			}
		}
	}
	if len(want) < 425 {
		t.Errorf("pin count %d, want at least 425", len(want))
	}
}

func TestLiveV1FrozenParameters(t *testing.T) {
	m, err := harness.LoadManifest("../falsifiable-v0")
	if err != nil {
		t.Fatal(err)
	}
	ids := harness.LiveFrozenTaskIDs(m)
	sort.Strings(ids)
	if len(ids) != 60 || ids[0] != "a01" || ids[59] != "t30" {
		t.Fatalf("frozen task list changed: %v", ids)
	}
	for _, id := range ids {
		task, _ := m.Task(id)
		if !reflect.DeepEqual(task.AllowedCaps, []string{"filesystem"}) || task.Oracle.Status != "reference" {
			t.Fatalf("%s: round 1 admits only filesystem-only tasks with reference oracles", id)
		}
	}
	if harness.LiveMaxRepairs != 2 || harness.LiveDefaultDeadline != 20*time.Second || harness.LiveMaxDeadline != 30*time.Second {
		t.Fatal("repair budget or deadline changed after pre-registration")
	}
	if harness.LiveClaudeModel != "claude-opus-5-5" || harness.LiveClaudeCLIVersion != "2.1.286 (Claude Code)" || len(harness.LiveAllowedModelIDs) != 1 {
		t.Fatal("model pins changed after pre-registration")
	}
}

// Heinrich re-review: the Arm B v1 language and every prompt are frozen by
// literal hash, independent of pins.json, because the designer had seen the
// blind set (PREREGISTRATION.md §2). Never update these hashes.
func TestLiveV1LanguageAndPromptsFrozen(t *testing.T) {
	want := map[string]string{
		"../falsifiable-v0/harness/live_planv1.go":  "742b33e7e2e7b1b95a045d4ac587f3aabbc019497e3a74c2acd3c54534c52b09",
		"../falsifiable-v0/harness/live_planarm.go": "3f50ef887ee690f5898e3eaa888211032c16e9b1f25f5789fe62cc356eed7aee",
		"prompts/arm_a.md":                          "0b95f9ba34c32bb0027823c9f6f1fe1ddab33f06aacb10756ed41ae252d66e20",
		"prompts/arm_b.md":                          "50f15a262c1fb8054f81de6bce8ec81e9fb2546d7d7ec147693b861fb0215a57",
		"prompts/repair.md":                         "bd54b4f6f5087021f32c87fe67ae5a2aeafd2f17479cdfb14affa92156d774b8",
	}
	for p, h := range want {
		b, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		if got := fmt.Sprintf("%x", sha256.Sum256(b)); got != h {
			t.Errorf("%s changed after the v1 freeze: %s", p, got)
		}
	}
}
