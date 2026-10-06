package harness

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const MaxPlanSteps = 64
const MaxPlanDepth = 8

type Diagnostic struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (d *Diagnostic) Error() string { b, _ := json.Marshal(d); return string(b) }
func invalid(format string, args ...interface{}) error {
	return &Diagnostic{Code: "PLAN_INVALID", Message: fmt.Sprintf(format, args...)}
}

type Plan struct {
	Schema string `json:"schema"`
	Steps  []Step `json:"steps"`
}
type Predicate struct {
	Field string      `json:"field"`
	Op    string      `json:"op"`
	Value interface{} `json:"value"`
}

// Conditions address variables by name or a single dotted object field.
// all/any compare record field to value; count compares list length to value;
// equals compares a variable to value; semver_gt compares two strict x.y.z versions.
type Condition struct {
	Op    string      `json:"op"`
	Var   string      `json:"var"`
	Field string      `json:"field,omitempty"`
	Value interface{} `json:"value"`
}
type Step struct {
	Action  string                 `json:"action"`
	Path    string                 `json:"path,omitempty"`
	Format  string                 `json:"format,omitempty"`
	As      string                 `json:"as,omitempty"`
	From    string                 `json:"from,omitempty"`
	Where   *Predicate             `json:"where,omitempty"`
	Cond    *Condition             `json:"cond,omitempty"`
	Then    []Step                 `json:"then,omitempty"`
	Else    []Step                 `json:"else,omitempty"`
	Record  map[string]interface{} `json:"record,omitempty"`
	Reason  string                 `json:"reason,omitempty"`
	Summary map[string]interface{} `json:"summary,omitempty"`
}

func (s *Step) UnmarshalJSON(b []byte) error {
	type plain Step
	var x plain
	if e := strict(b, &x); e != nil {
		return e
	}
	allowed := map[string][]string{"read_file": {"action", "path", "format", "as"}, "filter": {"action", "from", "where", "as"}, "if": {"action", "cond", "then", "else"}, "write_record": {"action", "path", "record"}, "request_approval": {"action", "reason", "summary"}}
	keys, ok := allowed[x.Action]
	if !ok {
		return invalid("unknown action %q", x.Action)
	}
	var fields map[string]json.RawMessage
	if e := json.Unmarshal(b, &fields); e != nil {
		return e
	}
	for _, k := range keys {
		if _, ok := fields[k]; !ok {
			return invalid("missing field %q for %s", k, x.Action)
		}
	}
	for k := range fields {
		if !has(keys, k) {
			return invalid("field %q not valid for %s", k, x.Action)
		}
	}
	*s = Step(x)
	return nil
}
func DecodePlan(b []byte) (Plan, error) {
	var p Plan
	if e := strict(b, &p); e != nil {
		return p, invalid("%v", e)
	}
	if p.Schema != "howlframe.exp.plan/v0" || len(p.Steps) == 0 {
		return p, invalid("invalid plan header/empty steps")
	}
	return p, nil
}
func confinedExisting(root, p string) (string, error) {
	if !cleanRelative(p) {
		return "", invalid("unclean relative path %q", p)
	}
	base, e := filepath.EvalSymlinks(root)
	if e != nil {
		return "", e
	}
	full, e := filepath.EvalSymlinks(filepath.Join(base, p))
	if e != nil {
		return "", e
	}
	rel, e := filepath.Rel(base, full)
	if e != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", invalid("symlink escape %q", p)
	}
	return full, nil
}
func readAllowed(t Task, p string) bool {
	for _, g := range t.AllowedEffects.Reads {
		if g == "evidence/**" && strings.HasPrefix(p, "evidence/") {
			return true
		}
		if ok, _ := filepath.Match(g, p); ok {
			return true
		}
	}
	return false
}
func readPath(root string, t Task, p string) (string, error) {
	if !readAllowed(t, p) {
		return "", invalid("read not allowed %q", p)
	}
	full, e := confinedExisting(root, p)
	if e != nil {
		return "", e
	}
	base, e := filepath.EvalSymlinks(root)
	if e != nil {
		return "", e
	}
	rel, e := filepath.Rel(base, full)
	if e != nil || !readAllowed(t, filepath.ToSlash(rel)) {
		return "", invalid("read target not allowed %q", p)
	}
	info, e := os.Stat(full)
	if e != nil {
		return "", e
	}
	if !info.Mode().IsRegular() {
		return "", invalid("read requires a regular file")
	}
	return full, nil
}
func writePath(root string, t Task, p string) (string, error) {
	if !cleanRelative(p) || !has(t.AllowedEffects.Writes, p) {
		return "", invalid("write not allowed %q", p)
	}
	// Check every existing ancestor (and leaf); broken symlinks fail closed.
	for q := p; q != "."; q = filepath.Dir(q) {
		info, e := os.Lstat(filepath.Join(root, q))
		if e == nil {
			if info.Mode()&os.ModeSymlink != 0 {
				return "", invalid("write symlink forbidden %q", q)
			}
			if _, e = confinedExisting(root, q); e != nil {
				return "", e
			}
		} else if !os.IsNotExist(e) {
			return "", e
		} else if q != p && !has(t.AllowedEffects.Writes, q) {
			return "", invalid("creating output parent not allowed %q", q)
		}
	}
	return filepath.Join(root, p), nil
}
func validName(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range s {
		if !(c == '_' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9') {
			return false
		}
	}
	return true
}
func ValidatePlan(p Plan, root string, t Task) error {
	if !has(t.AllowedCaps, "filesystem") {
		return invalid("filesystem capability not granted")
	}
	if p.Schema != "howlframe.exp.plan/v0" || len(p.Steps) == 0 {
		return invalid("invalid plan header")
	}
	count := 0
	var walk func([]Step, int, map[string]bool) error
	walk = func(steps []Step, depth int, vars map[string]bool) error {
		if depth > MaxPlanDepth {
			return invalid("depth exceeds %d", MaxPlanDepth)
		}
		for _, s := range steps {
			count++
			if count > MaxPlanSteps {
				return invalid("too many steps")
			}
			switch s.Action {
			case "read_file":
				if !has([]string{"text", "lines", "json", "kv"}, s.Format) || !validName(s.As) || !readAllowed(t, s.Path) {
					return invalid("invalid read")
				}
				if _, e := readPath(root, t, s.Path); e != nil {
					return invalid("%v", e)
				}
				vars[s.As] = true
			case "filter":
				if !vars[s.From] || !validName(s.As) || s.Where == nil || !has([]string{"eq", "ne", "gt", "lt", "ge", "le", "contains", "semver_gt", "in"}, s.Where.Op) {
					return invalid("invalid filter")
				}
				if e := validateRefs(s.Where.Value, vars); e != nil {
					return e
				}
				vars[s.As] = true
			case "if":
				if s.Cond == nil || !vars[strings.Split(s.Cond.Var, ".")[0]] || !has([]string{"all", "any", "count", "equals", "semver_gt"}, s.Cond.Op) || s.Then == nil || s.Else == nil {
					return invalid("invalid condition/branches")
				}
				if e := validateRefs(s.Cond.Value, vars); e != nil {
					return e
				}
				for _, branch := range [][]Step{s.Then, s.Else} {
					copy := map[string]bool{}
					for k, v := range vars {
						copy[k] = v
					}
					if e := walk(branch, depth+1, copy); e != nil {
						return e
					}
				}
			case "write_record":
				if s.Record == nil {
					return invalid("record must be object")
				}
				if _, e := writePath(root, t, s.Path); e != nil {
					return e
				}
				if e := validateRefs(s.Record, vars); e != nil {
					return e
				}
			case "request_approval":
				if s.Reason == "" || s.Summary == nil {
					return invalid("approval requires reason and summary")
				}
				if _, e := writePath(root, t, "out/approval_request.json"); e != nil {
					return e
				}
				if e := validateRefs(s.Summary, vars); e != nil {
					return e
				}
			default:
				return invalid("unknown action %q", s.Action)
			}
		}
		return nil
	}
	return walk(p.Steps, 1, map[string]bool{})
}

type BrokerResult struct {
	ExecutedSteps int      `json:"executed_steps"`
	Effects       []Effect `json:"effects"`
}

func ExecutePlan(p Plan, root string, t Task) (BrokerResult, error) {
	r := BrokerResult{Effects: []Effect{}}
	if e := ValidatePlan(p, root, t); e != nil {
		return r, e
	}
	vars := map[string]interface{}{}
	var run func([]Step, map[string]interface{}) error
	run = func(steps []Step, vars map[string]interface{}) error {
		for _, s := range steps {
			r.ExecutedSteps++
			switch s.Action {
			case "read_file":
				full, e := readPath(root, t, s.Path)
				if e != nil {
					return e
				}
				b, e := os.ReadFile(full)
				if e != nil {
					return e
				}
				var v interface{}
				switch s.Format {
				case "text":
					v = string(b)
				case "json":
					if e := strict(b, &v); e != nil {
						return e
					}
				case "lines":
					xs := []interface{}{}
					for _, line := range strings.Split(strings.TrimSuffix(string(b), "\n"), "\n") {
						if line != "" {
							xs = append(xs, line)
						}
					}
					v = xs
				case "kv":
					m := map[string]interface{}{}
					for _, line := range strings.Split(string(b), "\n") {
						if line == "" {
							continue
						}
						pair := strings.SplitN(line, "=", 2)
						if len(pair) != 2 {
							return invalid("invalid kv line")
						}
						if _, ok := m[pair[0]]; ok {
							return invalid("duplicate kv key")
						}
						m[pair[0]] = pair[1]
					}
					v = m
				}
				vars[s.As] = v
				r.Effects = append(r.Effects, Effect{Kind: "read", Path: s.Path})
			case "filter":
				xs, e := records(vars[s.From])
				if e != nil {
					return e
				}
				value, e := resolve(s.Where.Value, vars)
				if e != nil {
					return e
				}
				out := []interface{}{}
				for _, x := range xs {
					v, e := field(x, s.Where.Field)
					if e != nil {
						return e
					}
					ok, e := compare(v, value, s.Where.Op)
					if e != nil {
						return e
					}
					if ok {
						out = append(out, x)
					}
				}
				vars[s.As] = out
			case "if":
				ok, e := condition(*s.Cond, vars)
				if e != nil {
					return e
				}
				branch := s.Else
				if ok {
					branch = s.Then
				}
				local := map[string]interface{}{}
				for k, v := range vars {
					local[k] = v
				}
				if e := run(branch, local); e != nil {
					return e
				}
			case "write_record", "request_approval":
				path := s.Path
				var record interface{} = s.Record
				if s.Action == "request_approval" {
					path = "out/approval_request.json"
					record = map[string]interface{}{"reason": s.Reason, "summary": s.Summary}
				}
				v, e := resolve(record, vars)
				if e != nil {
					return e
				}
				b, e := json.MarshalIndent(v, "", "  ")
				if e != nil {
					return e
				}
				full, e := writePath(root, t, path)
				if e != nil {
					return e
				}
				kind := "created"
				if _, e := os.Stat(full); e == nil {
					kind = "modified"
				}
				if e := os.MkdirAll(filepath.Dir(full), 0700); e != nil {
					return e
				}
				if e := os.WriteFile(full, append(b, '\n'), 0600); e != nil {
					return e
				}
				r.Effects = append(r.Effects, Effect{Kind: kind, Path: path})
			}
		}
		return nil
	}
	e := run(p.Steps, vars)
	return r, e
}
