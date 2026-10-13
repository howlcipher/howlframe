package harness

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

func sha(b []byte) string { return fmt.Sprintf("%x", sha256.Sum256(b)) }

// LiveFrozenTaskIDs returns the live-v1 round-1 task list: every manifest task
// whose only grant is filesystem (no network, no process:env).
func LiveFrozenTaskIDs(m Manifest) []string {
	ids := []string{}
	for _, t := range m.All() {
		if len(t.AllowedCaps) == 1 && t.AllowedCaps[0] == "filesystem" {
			ids = append(ids, t.ID)
		}
	}
	return ids
}

// ComputeLivePins hashes every input that defines the protocol: the task
// manifest, every task file (evidence and hidden oracles), the action schema,
// the live prompts and every rendered initial prompt.
func ComputeLivePins(liveRoot, root string) (map[string]string, error) {
	pins := map[string]string{}
	add := func(label, path string) error {
		b, e := os.ReadFile(path)
		if e != nil {
			return e
		}
		pins[label] = sha(b)
		return nil
	}
	for _, p := range []string{"tasks/manifest.json", "schema/action_catalog.schema.json"} {
		if e := add("falsifiable-v0/"+p, filepath.Join(root, p)); e != nil {
			return nil, e
		}
	}
	for _, p := range []string{"prompts/arm_a.md", "prompts/arm_b.md", "prompts/repair.md"} {
		if e := add("live-v1/"+p, filepath.Join(liveRoot, p)); e != nil {
			return nil, e
		}
	}
	m, e := LoadManifest(root)
	if e != nil {
		return nil, e
	}
	ps, e := LoadPromptSet(liveRoot, root)
	if e != nil {
		return nil, e
	}
	ids := LiveFrozenTaskIDs(m)
	sort.Strings(ids)
	for _, id := range ids {
		dir := filepath.Join(root, "tasks", id)
		e := filepath.Walk(dir, func(p string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() {
				return err
			}
			rel, _ := filepath.Rel(root, p)
			return add("falsifiable-v0/"+filepath.ToSlash(rel), p)
		})
		if e != nil {
			return nil, e
		}
		t, _ := m.Task(id)
		for _, arm := range []string{"a", "b"} {
			pins["rendered/"+id+"/arm_"+arm] = sha([]byte(ps.RenderInitial(arm, t)))
		}
	}
	return pins, nil
}
