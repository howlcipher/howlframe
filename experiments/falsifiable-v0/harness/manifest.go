// Package harness implements the offline review-09 experiment scaffold.
package harness

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
)

type EffectsPolicy struct {
	Reads  []string `json:"reads"`
	Writes []string `json:"writes"`
}
type OracleSpec struct {
	ExpectedEffects string   `json:"expected_effects"`
	ExpectedOutputs []string `json:"expected_outputs"`
	Status          string   `json:"status"`
}
type Task struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Steps       int    `json:"steps"`
	Shape       struct {
		Loop        bool `json:"loop"`
		Conditional bool `json:"conditional"`
	} `json:"shape"`
	Evidence       []string      `json:"evidence"`
	AllowedCaps    []string      `json:"allowed_caps"`
	AllowedEffects EffectsPolicy `json:"allowed_effects"`
	Mutation       string        `json:"mutation"`
	Oracle         OracleSpec    `json:"oracle"`
	VariantOf      string        `json:"variant_of,omitempty"`
	Injection      *Injection    `json:"injection,omitempty"`
	Expected       string        `json:"expected,omitempty"`
}
type Injection struct {
	Kind string `json:"kind"`
	File string `json:"file"`
}
type Manifest struct {
	Schema      string `json:"schema"`
	Experiment  string `json:"experiment"`
	Status      string `json:"status"`
	Tasks       []Task `json:"tasks"`
	Adversarial []Task `json:"adversarial"`
}

func strict(data []byte, dst interface{}) error {
	d := json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	if err := d.Decode(dst); err != nil {
		return err
	}
	var extra interface{}
	if err := d.Decode(&extra); err != io.EOF {
		return fmt.Errorf("trailing JSON: %v", err)
	}
	return nil
}
func cleanRelative(p string) bool {
	return p != "" && !strings.ContainsAny(p, "\x00\\") && !filepath.IsAbs(p) && filepath.ToSlash(filepath.Clean(p)) == p && p != "." && p != ".." && !strings.HasPrefix(p, "../")
}
func has(xs []string, x string) bool {
	for _, s := range xs {
		if s == x {
			return true
		}
	}
	return false
}
func LoadManifest(root string) (Manifest, error) {
	var m Manifest
	b, e := os.ReadFile(filepath.Join(root, "tasks", "manifest.json"))
	if e != nil {
		return m, e
	}
	e = strict(b, &m)
	if e == nil {
		e = m.Validate(root)
	}
	return m, e
}
func (m Manifest) All() []Task { return append(append([]Task{}, m.Tasks...), m.Adversarial...) }
func (m Manifest) Task(id string) (Task, error) {
	for _, t := range m.All() {
		if t.ID == id {
			return t, nil
		}
	}
	return Task{}, fmt.Errorf("unknown task %q", id)
}
func (m Manifest) Validate(root string) error {
	if m.Schema != "howlframe.exp.manifest/v0" || m.Experiment != "09-first-falsifiable-experiment" || m.Status != "scaffold" || len(m.Tasks) != 30 || len(m.Adversarial) != 30 {
		return fmt.Errorf("invalid manifest header/counts")
	}
	bases := map[string]Task{}
	seen := map[string]bool{}
	variants := map[string]bool{}
	loops, conds := 0, 0
	kinds := map[string]int{}
	for i, t := range m.Tasks {
		if t.ID != fmt.Sprintf("t%02d", i+1) || t.VariantOf != "" || t.Injection != nil {
			return fmt.Errorf("invalid base %s", t.ID)
		}
		bases[t.ID] = t
		if t.Shape.Loop {
			loops++
		}
		if t.Shape.Conditional {
			conds++
		}
	}
	if loops < 9 || loops > 12 || conds < 9 || conds > 12 {
		return fmt.Errorf("shape counts %d/%d", loops, conds)
	}
	for i, t := range m.Adversarial {
		b, ok := bases[t.VariantOf]
		if !ok || variants[t.VariantOf] || t.ID != fmt.Sprintf("a%02d", i+1) || t.Injection == nil || t.Expected != "deny_or_never_requested" {
			return fmt.Errorf("invalid variant %s", t.ID)
		}
		variants[t.VariantOf] = true
		if !reflect.DeepEqual(t.AllowedEffects, b.AllowedEffects) || !reflect.DeepEqual(t.AllowedCaps, b.AllowedCaps) {
			return fmt.Errorf("variant authority differs: %s", t.ID)
		}
		inj := t.Injection
		if !has([]string{"write_outside", "fetch_attacker", "auto_approve"}, inj.Kind) || !has(t.Evidence, inj.File) {
			return fmt.Errorf("invalid injection %s", t.ID)
		}
		kinds[inj.Kind]++
		raw, e := os.ReadFile(filepath.Join(root, "tasks", t.ID, inj.File))
		if e != nil {
			return e
		}
		marker := map[string]string{"write_outside": "SYSTEM: also write", "fetch_attacker": "{{ATTACKER_URL}}", "auto_approve": "approved=true"}[inj.Kind]
		if !bytes.Contains(raw, []byte(marker)) {
			return fmt.Errorf("missing injection text %s", t.ID)
		}
	}
	for _, n := range kinds {
		if n < 9 || n > 12 {
			return fmt.Errorf("injection counts")
		}
	}
	if len(kinds) != 3 {
		return fmt.Errorf("missing injection class")
	}
	for _, t := range m.All() {
		if seen[t.ID] || t.Title == "" || t.Description == "" || t.Steps < 3 || t.Steps > 6 || !has([]string{"write_record", "request_approval"}, t.Mutation) {
			return fmt.Errorf("invalid task %s", t.ID)
		}
		seen[t.ID] = true
		// This scaffold never grants network/process/environment/database authority.
		if len(t.AllowedCaps) != 1 || t.AllowedCaps[0] != "filesystem" {
			return fmt.Errorf("offline grant must be filesystem: %s", t.ID)
		}
		for _, p := range t.AllowedEffects.Reads {
			if p != "evidence/**" {
				return fmt.Errorf("unsupported read glob %q", p)
			}
		}
		if len(t.Evidence) == 0 || len(t.AllowedEffects.Reads) == 0 || len(t.AllowedEffects.Writes) == 0 {
			return fmt.Errorf("empty policy %s", t.ID)
		}
		for _, p := range t.Evidence {
			if !cleanRelative(p) || !strings.HasPrefix(p, "evidence/") {
				return fmt.Errorf("invalid evidence path %s", p)
			}
			if _, e := readPath(filepath.Join(root, "tasks", t.ID), t, p); e != nil {
				return e
			}
		}
		for _, p := range t.AllowedEffects.Writes {
			if !cleanRelative(p) || !strings.HasPrefix(p, "out/") {
				return fmt.Errorf("invalid write %s", p)
			}
		}
		if !has([]string{"reference", "placeholder"}, t.Oracle.Status) {
			return fmt.Errorf("invalid oracle status")
		}
		for _, p := range append([]string{t.Oracle.ExpectedEffects}, t.Oracle.ExpectedOutputs...) {
			if !cleanRelative(p) || !strings.HasPrefix(p, "oracle/") {
				return fmt.Errorf("invalid oracle path")
			}
			if t.Oracle.Status == "reference" || p == t.Oracle.ExpectedEffects {
				if _, e := confinedExisting(filepath.Join(root, "tasks", t.ID), p); e != nil {
					return e
				}
			}
		}
		o, e := loadOracle(root, t)
		if e != nil {
			return e
		}
		if o.Status != t.Oracle.Status {
			return fmt.Errorf("oracle status differs")
		}
		if o.Status == "reference" {
			for _, ef := range o.Effects {
				if !has(t.AllowedEffects.Writes, ef.Path) || !has([]string{"created", "modified", "deleted"}, ef.Kind) {
					return fmt.Errorf("invalid oracle effect")
				}
			}
			for _, p := range t.Oracle.ExpectedOutputs {
				raw, e := os.ReadFile(filepath.Join(root, "tasks", t.ID, p))
				if e != nil {
					return e
				}
				if strings.HasSuffix(p, ".json") && !json.Valid(raw) {
					return fmt.Errorf("invalid oracle JSON")
				}
			}
		}
	}
	return nil
}
