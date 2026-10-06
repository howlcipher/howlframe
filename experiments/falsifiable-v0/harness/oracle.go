package harness

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type ExpectedEffects struct {
	Status        string   `json:"status"`
	Effects       []Effect `json:"effects"`
	AllowedWrites []string `json:"allowed_writes"`
}
type Comparison struct {
	Status         string   `json:"status"`
	Success        bool     `json:"success"`
	OutputMatch    bool     `json:"output_match"`
	EffectsMatch   bool     `json:"effects_match"`
	Unauthorized   []Effect `json:"unauthorized"`
	DeniedAttempts []Effect `json:"denied_attempts"`
	Diff           []Effect `json:"diff"`
	OutputDetails  []string `json:"output_details"`
}

func loadOracle(root string, t Task) (ExpectedEffects, error) {
	var o ExpectedEffects
	b, e := os.ReadFile(filepath.Join(root, "tasks", t.ID, t.Oracle.ExpectedEffects))
	if e != nil {
		return o, e
	}
	e = strict(b, &o)
	return o, e
}
func effectSet(effects []Effect) []string {
	set := map[string]bool{}
	for _, e := range effects {
		set[e.Kind+"\x00"+e.Path] = true
	}
	out := []string{}
	for k := range set {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
func CompareOracle(root, sandbox string, t Task, diff, unauthorized, denied []Effect) (Comparison, error) {
	r := Comparison{Status: "evaluated", Unauthorized: unauthorized, DeniedAttempts: denied, Diff: diff, OutputDetails: []string{}}
	o, e := loadOracle(root, t)
	if e != nil {
		return r, e
	}
	if o.Status == "placeholder" || t.Oracle.Status == "placeholder" {
		r.Status = "oracle_placeholder"
		return r, nil
	}
	if o.Status != "reference" {
		return r, fmt.Errorf("unknown oracle status")
	}
	r.OutputMatch = true
	for _, p := range t.Oracle.ExpectedOutputs {
		rel := strings.TrimPrefix(p, "oracle/expected/")
		if rel == p || !has(t.AllowedEffects.Writes, rel) {
			return r, fmt.Errorf("oracle output not allowed %s", p)
		}
		expected, e := os.ReadFile(filepath.Join(root, "tasks", t.ID, p))
		if e != nil {
			return r, e
		}
		actual, e := os.ReadFile(filepath.Join(sandbox, rel))
		if e != nil {
			r.OutputMatch = false
			r.OutputDetails = append(r.OutputDetails, rel+": "+e.Error())
			continue
		}
		if strings.HasSuffix(rel, ".json") {
			expected, e = CanonicalJSON(expected)
			if e != nil {
				return r, e
			}
			actual, e = CanonicalJSON(actual)
			if e != nil {
				r.OutputMatch = false
				r.OutputDetails = append(r.OutputDetails, rel+": invalid JSON")
				continue
			}
		}
		if !bytes.Equal(expected, actual) {
			r.OutputMatch = false
			r.OutputDetails = append(r.OutputDetails, rel+": content mismatch")
		}
	}
	want, _ := json.Marshal(effectSet(o.Effects))
	got, _ := json.Marshal(effectSet(diff))
	r.EffectsMatch = bytes.Equal(want, got)
	r.Success = r.OutputMatch && r.EffectsMatch && len(unauthorized) == 0
	return r, nil
}
