package harness

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
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
	for _, p := range []string{"prompts/arm_a.md", "prompts/arm_b.md", "prompts/repair.md", "PREREGISTRATION.md", "cmd/hflive/main.go"} {
		if e := add("live-v1/"+p, filepath.Join(liveRoot, p)); e != nil {
			return nil, e
		}
	}
	code, e := LiveCodePins(root)
	if e != nil {
		return nil, e
	}
	for k, v := range code {
		pins[k] = v
	}
	pins["live-v1-blind/manifest.json(pinned)"] = LiveBlindManifestSHA256
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
			pins["rendered/"+id+"/arm_"+arm] = sha([]byte(ps.RenderInitial(arm, LiveTask{Task: t, Set: "tuning"})))
		}
	}
	return pins, nil
}

// LiveBlindManifestSHA256 pins Hange's blind set (howlframe#180) by the
// sha256 of its manifest.json, which lists the sha256 of every file.
const LiveBlindManifestSHA256 = "b4e525207b1b46b978415c92f54fdd6cc5eb23375506345512737d2f8b132fde"

// LiveCodePins hashes every non-test Go file of the harness package.
func LiveCodePins(root string) (map[string]string, error) {
	files, e := filepath.Glob(filepath.Join(root, "harness", "*.go"))
	if e != nil {
		return nil, e
	}
	out := map[string]string{}
	for _, p := range files {
		if strings.HasSuffix(p, "_test.go") {
			continue
		}
		b, e := os.ReadFile(p)
		if e != nil {
			return nil, e
		}
		out["falsifiable-v0/harness/"+filepath.Base(p)] = sha(b)
	}
	return out, nil
}
