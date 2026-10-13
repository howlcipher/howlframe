// Package livev1 holds the pre-registered live-model comparison protocol.
package livev1

import (
	"encoding/json"
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
