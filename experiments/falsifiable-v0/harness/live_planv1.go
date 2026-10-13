package harness

// Arm B v1 action language for live-v1 (pre-registered in
// experiments/live-v1/PREREGISTRATION.md §5b and
// experiments/live-v1/schema/plan_v1.md). Designed from the generic task
// categories (config, logs, notes, inventory, files, records), not from any
// blind task. It is a superset of review-09 plan/v0: v0 plans run unchanged.
// Every file effect is checked against the same exact grant list Arm A runs
// under and recorded in a howlframe.receipt/v0 receipt.

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

const (
	PlanV1Schema   = "howlframe.live.plan/v1"
	V1MaxSteps     = 256
	V1MaxDepth     = 8
	V1MaxOps       = 200000
	V1MaxWriteSize = 1 << 20
	V1MaxRegex     = 512
)

// v1Keys is the closed key set of each action (plus "action").
var v1Keys = map[string][]string{
	"read_file":        {"path", "format", "as"},
	"list_dir":         {"path", "as", "recursive"},
	"set":              {"as", "value"},
	"append":           {"to", "value"},
	"filter":           {"from", "where", "as"},
	"for_each":         {"from", "as", "steps"},
	"if":               {"cond", "then", "else"},
	"write_text":       {"path", "value"},
	"write_record":     {"path", "record"},
	"request_approval": {"reason", "summary"},
}

// V1Ops is the closed operator set for {"$op": name, "args": [...]}.
var V1Ops = map[string]bool{}

func init() {
	for _, o := range strings.Fields(`concat join split lines trim lower upper replace starts_with ends_with contains
regex_match regex_find regex_find_all regex_replace pad_left pad_right substr len to_string to_int to_number
parse_json encode_json parse_csv parse_kv basename dirname ext path_join
get keys values entries from_entries sort sort_by reverse unique slice flatten push count sum min max group_by pluck range
merge deep_merge set_key del_key has
eq ne lt le gt ge and or not add sub mul div mod round if semver_gt is_null type
render_lines render_csv render_md_table render_md_list`) {
		V1Ops[o] = true
	}
	for a := range v1Keys {
		CatalogActions[a] = true
	}
}

type v1Err struct{ code, msg string }

func (e *v1Err) Error() string {
	b, _ := json.Marshal(map[string]string{"code": e.code, "message": e.msg})
	return string(b)
}
func v1invalid(f string, a ...interface{}) error { return &v1Err{"PLAN_INVALID", fmt.Sprintf(f, a...)} }
func v1runtime(f string, a ...interface{}) error { return &v1Err{"PLAN_RUNTIME", fmt.Sprintf(f, a...)} }

// grantRoots parses exact grants: filesystem:read=<root>, filesystem:write=<root>.
func grantRoots(grants []string) (reads, writes []string) {
	for _, g := range grants {
		if strings.HasPrefix(g, "filesystem:read=") {
			reads = append(reads, strings.TrimPrefix(g, "filesystem:read="))
		}
		if strings.HasPrefix(g, "filesystem:write=") {
			writes = append(writes, strings.TrimPrefix(g, "filesystem:write="))
		}
	}
	return
}
func underRoot(p string, roots []string) bool {
	if !cleanRelative(p) {
		return false
	}
	for _, r := range roots {
		if p == r || strings.HasPrefix(p, r+"/") {
			return true
		}
	}
	return false
}

// symlinkFree reports whether every existing component of p under root is
// not a symlink.
func symlinkFree(root, p string) bool {
	cur := root
	for _, part := range strings.Split(p, "/") {
		cur = filepath.Join(cur, part)
		fi, e := os.Lstat(cur)
		if e != nil {
			return true
		}
		if fi.Mode()&os.ModeSymlink != 0 {
			return false
		}
	}
	return true
}

// ValidatePlanV1 checks structure and the closed key/op sets. Grant checks
// happen earlier (planGrantCheck) and again per effect at runtime.
func ValidatePlanV1(candidate []byte) (map[string]interface{}, error) {
	var plan map[string]interface{}
	if e := strict(candidate, &plan); e != nil {
		return nil, v1invalid("%v", e)
	}
	for k := range plan {
		if k != "schema" && k != "steps" {
			return nil, v1invalid("unknown top-level key %q", k)
		}
	}
	if plan["schema"] != PlanV1Schema && plan["schema"] != "howlframe.exp.plan/v0" {
		return nil, v1invalid("schema must be %s", PlanV1Schema)
	}
	steps, ok := plan["steps"].([]interface{})
	if !ok {
		return nil, v1invalid("steps must be an array")
	}
	count := 0
	var walkSteps func([]interface{}, int) error
	var walkExpr func(interface{}, int) error
	walkExpr = func(v interface{}, d int) error {
		if d > 64 {
			return v1invalid("expression too deep")
		}
		switch x := v.(type) {
		case map[string]interface{}:
			if op, ok := x["$op"]; ok {
				s, _ := op.(string)
				if !V1Ops[s] {
					return v1invalid("unknown $op %v", op)
				}
				for k := range x {
					if k != "$op" && k != "args" {
						return v1invalid("$op object has unknown key %q", k)
					}
				}
				args, ok := x["args"].([]interface{})
				if !ok && x["args"] != nil {
					return v1invalid("args must be an array")
				}
				for _, a := range args {
					if e := walkExpr(a, d+1); e != nil {
						return e
					}
				}
				return nil
			}
			if _, ok := x["$var"]; ok {
				if len(x) != 1 {
					return v1invalid("$var must be sole key")
				}
				if _, ok := x["$var"].(string); !ok {
					return v1invalid("$var must be a string")
				}
				return nil
			}
			for _, e := range x {
				if err := walkExpr(e, d+1); err != nil {
					return err
				}
			}
		case []interface{}:
			for _, e := range x {
				if err := walkExpr(e, d+1); err != nil {
					return err
				}
			}
		}
		return nil
	}
	walkSteps = func(ss []interface{}, depth int) error {
		if depth > V1MaxDepth {
			return v1invalid("nesting exceeds %d", V1MaxDepth)
		}
		for _, raw := range ss {
			count++
			if count > V1MaxSteps {
				return v1invalid("more than %d steps", V1MaxSteps)
			}
			s, ok := raw.(map[string]interface{})
			if !ok {
				return v1invalid("step must be an object")
			}
			act, _ := s["action"].(string)
			keys, ok := v1Keys[act]
			if !ok {
				return v1invalid("unknown action %q", s["action"])
			}
			allowed := map[string]bool{"action": true}
			for _, k := range keys {
				allowed[k] = true
			}
			for k := range s {
				if !allowed[k] {
					return v1invalid("%s: unknown key %q", act, k)
				}
			}
			for _, k := range []string{"as", "to", "from"} {
				if v, ok := s[k]; ok {
					n, ok := v.(string)
					if !ok || !validName(strings.SplitN(n, ".", 2)[0]) {
						return v1invalid("%s: %s must be a variable name", act, k)
					}
				}
			}
			switch act {
			case "read_file":
				f, _ := s["format"].(string)
				if !map[string]bool{"text": true, "lines": true, "json": true, "kv": true, "csv": true}[f] {
					return v1invalid("read_file format must be text, lines, json, kv or csv")
				}
			case "for_each":
				body, ok := s["steps"].([]interface{})
				if !ok {
					return v1invalid("for_each needs steps")
				}
				if e := walkSteps(body, depth+1); e != nil {
					return e
				}
			case "if":
				th, ok1 := s["then"].([]interface{})
				el, ok2 := s["else"].([]interface{})
				if !ok1 || !ok2 {
					return v1invalid("if needs then and else arrays")
				}
				if e := walkSteps(th, depth+1); e != nil {
					return e
				}
				if e := walkSteps(el, depth+1); e != nil {
					return e
				}
			}
			for _, k := range []string{"path", "value", "record", "summary", "cond"} {
				if v, ok := s[k]; ok {
					if e := walkExpr(v, 0); e != nil {
						return e
					}
				}
			}
			for _, k := range []string{"as", "from", "format"} {
				if v, ok := s[k]; ok {
					if _, ok := v.(string); !ok {
						return v1invalid("%s must be a string", k)
					}
				}
			}
			if act != "set" && act != "append" && act != "if" && act != "for_each" && act != "filter" && act != "request_approval" {
				if _, ok := s["path"]; !ok {
					return v1invalid("%s needs path", act)
				}
			}
		}
		return nil
	}
	if e := walkSteps(steps, 1); e != nil {
		return nil, e
	}
	return plan, nil
}

type v1Exec struct {
	root      string
	t         Task
	reads     []string
	writes    []string
	receipt   []ReceiptEffect
	ops       int
	written   int
	readOK    func(string) bool
	regexps   map[string]*regexp.Regexp
	stopped   bool
	refusalOp string
}

func (x *v1Exec) tick() error {
	x.ops++
	if x.ops > V1MaxOps {
		return v1runtime("operation budget %d exceeded", V1MaxOps)
	}
	return nil
}

// refuse records a denied receipt entry; execution stops (runtime refusal).
func (x *v1Exec) refuse(op, p string) error {
	x.receipt = append(x.receipt, ReceiptEffect{op, "filesystem", p, "denied"})
	return &v1Err{"CAPABILITY_DENIED", op + " " + p}
}

func (x *v1Exec) readPath(p string) (string, error) {
	if !underRoot(p, x.reads) || !symlinkFree(x.root, p) {
		return "", x.refuse("READ_FILE", p)
	}
	return filepath.Join(x.root, filepath.FromSlash(p)), nil
}

func (x *v1Exec) eval(v interface{}, vars map[string]interface{}) (interface{}, error) {
	if e := x.tick(); e != nil {
		return nil, e
	}
	switch m := v.(type) {
	case map[string]interface{}:
		if op, ok := m["$op"].(string); ok {
			raw, _ := m["args"].([]interface{})
			args := make([]interface{}, len(raw))
			if op == "if" {
				if len(raw) != 3 {
					return nil, v1runtime("if needs 3 args")
				}
				c, e := x.eval(raw[0], vars)
				if e != nil {
					return nil, e
				}
				if truthy(c) {
					return x.eval(raw[1], vars)
				}
				return x.eval(raw[2], vars)
			}
			for i, a := range raw {
				r, e := x.eval(a, vars)
				if e != nil {
					return nil, e
				}
				args[i] = r
			}
			return x.op(op, args)
		}
		if name, ok := m["$var"].(string); ok && len(m) == 1 {
			return varValue(vars, name)
		}
		out := map[string]interface{}{}
		for k, e := range m {
			r, err := x.eval(e, vars)
			if err != nil {
				return nil, err
			}
			out[k] = r
		}
		return out, nil
	case []interface{}:
		out := make([]interface{}, len(m))
		for i, e := range m {
			r, err := x.eval(e, vars)
			if err != nil {
				return nil, err
			}
			out[i] = r
		}
		return out, nil
	}
	return v, nil
}

func truthy(v interface{}) bool {
	switch x := v.(type) {
	case nil:
		return false
	case bool:
		return x
	case float64:
		return x != 0
	case string:
		return x != ""
	case []interface{}:
		return len(x) > 0
	case map[string]interface{}:
		return len(x) > 0
	}
	return true
}

func str(v interface{}) (string, error) {
	s, ok := v.(string)
	if !ok {
		return "", v1runtime("expected string, got %T", v)
	}
	return s, nil
}
func num(v interface{}) (float64, error) {
	f, ok := v.(float64)
	if !ok {
		return 0, v1runtime("expected number, got %T", v)
	}
	return f, nil
}
func list(v interface{}) ([]interface{}, error) {
	l, ok := v.([]interface{})
	if !ok {
		return nil, v1runtime("expected list, got %T", v)
	}
	return l, nil
}
func obj(v interface{}) (map[string]interface{}, error) {
	o, ok := v.(map[string]interface{})
	if !ok {
		return nil, v1runtime("expected object, got %T", v)
	}
	return o, nil
}
func toString(v interface{}) string {
	switch x := v.(type) {
	case string:
		return x
	case float64:
		return strconv.FormatFloat(x, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(x)
	case nil:
		return ""
	}
	b, _ := json.Marshal(v)
	return string(b)
}
func less(a, b interface{}) bool {
	if x, ok := a.(float64); ok {
		if y, ok := b.(float64); ok {
			return x < y
		}
	}
	return toString(a) < toString(b)
}

func (x *v1Exec) re(p string) (*regexp.Regexp, error) {
	if len(p) > V1MaxRegex {
		return nil, v1runtime("regex too long")
	}
	if r, ok := x.regexps[p]; ok {
		return r, nil
	}
	r, e := regexp.Compile(p)
	if e != nil {
		return nil, v1runtime("bad regex: %v", e)
	}
	x.regexps[p] = r
	return r, nil
}

func deepMerge(a, b interface{}) interface{} {
	am, ok1 := a.(map[string]interface{})
	bm, ok2 := b.(map[string]interface{})
	if !ok1 || !ok2 {
		return b
	}
	out := map[string]interface{}{}
	for k, v := range am {
		out[k] = v
	}
	for k, v := range bm {
		if old, ok := out[k]; ok {
			out[k] = deepMerge(old, v)
		} else {
			out[k] = v
		}
	}
	return out
}

func parseCSV(s string) ([]interface{}, error) {
	r := csv.NewReader(strings.NewReader(s))
	r.FieldsPerRecord = -1
	rows, e := r.ReadAll()
	if e != nil {
		return nil, v1runtime("csv: %v", e)
	}
	out := []interface{}{}
	if len(rows) == 0 {
		return out, nil
	}
	for _, row := range rows[1:] {
		rec := map[string]interface{}{}
		for i, h := range rows[0] {
			if i < len(row) {
				rec[h] = row[i]
			} else {
				rec[h] = ""
			}
		}
		out = append(out, rec)
	}
	return out, nil
}

func cell(rec interface{}, col string) string {
	if m, ok := rec.(map[string]interface{}); ok {
		return toString(m[col])
	}
	return ""
}
func columns(v interface{}) ([]string, error) {
	l, e := list(v)
	if e != nil {
		return nil, e
	}
	out := []string{}
	for _, c := range l {
		s, e := str(c)
		if e != nil {
			return nil, e
		}
		out = append(out, s)
	}
	return out, nil
}

func (x *v1Exec) op(op string, a []interface{}) (interface{}, error) {
	need := func(n int) error {
		if len(a) < n {
			return v1runtime("%s needs %d args", op, n)
		}
		return nil
	}
	s0 := func() (string, error) {
		if e := need(1); e != nil {
			return "", e
		}
		return str(a[0])
	}
	switch op {
	case "concat":
		var b strings.Builder
		for _, v := range a {
			b.WriteString(toString(v))
		}
		return b.String(), nil
	case "join":
		if e := need(2); e != nil {
			return nil, e
		}
		l, e := list(a[0])
		if e != nil {
			return nil, e
		}
		sep, e := str(a[1])
		if e != nil {
			return nil, e
		}
		parts := make([]string, len(l))
		for i, v := range l {
			parts[i] = toString(v)
		}
		return strings.Join(parts, sep), nil
	case "split":
		if e := need(2); e != nil {
			return nil, e
		}
		s, e := str(a[0])
		if e != nil {
			return nil, e
		}
		sep, e := str(a[1])
		if e != nil {
			return nil, e
		}
		out := []interface{}{}
		for _, p := range strings.Split(s, sep) {
			out = append(out, p)
		}
		return out, nil
	case "lines":
		s, e := s0()
		if e != nil {
			return nil, e
		}
		out := []interface{}{}
		for _, l := range strings.Split(strings.TrimSuffix(s, "\n"), "\n") {
			out = append(out, strings.TrimSuffix(l, "\r"))
		}
		if s == "" {
			out = []interface{}{}
		}
		return out, nil
	case "trim", "lower", "upper", "basename", "dirname", "ext":
		s, e := s0()
		if e != nil {
			return nil, e
		}
		return map[string]func(string) string{"trim": strings.TrimSpace, "lower": strings.ToLower, "upper": strings.ToUpper,
			"basename": path.Base, "dirname": path.Dir, "ext": path.Ext}[op](s), nil
	case "replace":
		if e := need(3); e != nil {
			return nil, e
		}
		s, e1 := str(a[0])
		o, e2 := str(a[1])
		n, e3 := str(a[2])
		if e1 != nil || e2 != nil || e3 != nil {
			return nil, v1runtime("replace needs strings")
		}
		return strings.ReplaceAll(s, o, n), nil
	case "starts_with", "ends_with":
		if e := need(2); e != nil {
			return nil, e
		}
		s, e1 := str(a[0])
		p, e2 := str(a[1])
		if e1 != nil || e2 != nil {
			return nil, v1runtime("%s needs strings", op)
		}
		if op == "starts_with" {
			return strings.HasPrefix(s, p), nil
		}
		return strings.HasSuffix(s, p), nil
	case "contains":
		if e := need(2); e != nil {
			return nil, e
		}
		switch c := a[0].(type) {
		case string:
			p, e := str(a[1])
			if e != nil {
				return nil, e
			}
			return strings.Contains(c, p), nil
		case []interface{}:
			for _, v := range c {
				if reflect.DeepEqual(v, a[1]) {
					return true, nil
				}
			}
			return false, nil
		case map[string]interface{}:
			k, e := str(a[1])
			if e != nil {
				return nil, e
			}
			_, ok := c[k]
			return ok, nil
		}
		return nil, v1runtime("contains needs string, list or object")
	case "regex_match", "regex_find", "regex_find_all", "regex_replace":
		if e := need(2); e != nil {
			return nil, e
		}
		s, e1 := str(a[0])
		p, e2 := str(a[1])
		if e1 != nil || e2 != nil {
			return nil, v1runtime("%s needs strings", op)
		}
		r, e := x.re(p)
		if e != nil {
			return nil, e
		}
		switch op {
		case "regex_match":
			return r.MatchString(s), nil
		case "regex_find":
			m := r.FindStringSubmatch(s)
			if m == nil {
				return nil, nil
			}
			out := []interface{}{}
			for _, g := range m {
				out = append(out, g)
			}
			return out, nil
		case "regex_find_all":
			out := []interface{}{}
			for _, m := range r.FindAllStringSubmatch(s, 10000) {
				g := []interface{}{}
				for _, x := range m {
					g = append(g, x)
				}
				out = append(out, g)
			}
			return out, nil
		}
		if e := need(3); e != nil {
			return nil, e
		}
		rep, e := str(a[2])
		if e != nil {
			return nil, e
		}
		return r.ReplaceAllString(s, rep), nil
	case "pad_left", "pad_right":
		if e := need(2); e != nil {
			return nil, e
		}
		s := toString(a[0])
		w, e := num(a[1])
		if e != nil || w > 10000 {
			return nil, v1runtime("%s width", op)
		}
		ch := " "
		if len(a) > 2 {
			ch = toString(a[2])
		}
		if ch == "" {
			ch = " "
		}
		for len([]rune(s)) < int(w) {
			if op == "pad_left" {
				s = ch + s
			} else {
				s += ch
			}
		}
		return s, nil
	case "substr", "slice":
		if e := need(2); e != nil {
			return nil, e
		}
		st, e := num(a[1])
		if e != nil {
			return nil, e
		}
		var n int
		var runes []rune
		var l []interface{}
		if s, ok := a[0].(string); ok {
			runes = []rune(s)
			n = len(runes)
		} else if l, e = list(a[0]); e == nil {
			n = len(l)
		} else {
			return nil, e
		}
		end := float64(n)
		if len(a) > 2 && a[2] != nil {
			if end, e = num(a[2]); e != nil {
				return nil, e
			}
		}
		i, j := clamp(int(st), n), clamp(int(end), n)
		if j < i {
			j = i
		}
		if runes != nil || n == 0 && l == nil {
			return string(runes[i:j]), nil
		}
		return append([]interface{}{}, l[i:j]...), nil
	case "len":
		if e := need(1); e != nil {
			return nil, e
		}
		switch c := a[0].(type) {
		case string:
			return float64(len([]rune(c))), nil
		case []interface{}:
			return float64(len(c)), nil
		case map[string]interface{}:
			return float64(len(c)), nil
		}
		return nil, v1runtime("len needs string, list or object")
	case "to_string":
		if e := need(1); e != nil {
			return nil, e
		}
		return toString(a[0]), nil
	case "to_int", "to_number":
		if e := need(1); e != nil {
			return nil, e
		}
		var f float64
		switch c := a[0].(type) {
		case float64:
			f = c
		case string:
			v, e := strconv.ParseFloat(strings.TrimSpace(c), 64)
			if e != nil {
				return nil, v1runtime("%s: %q is not a number", op, c)
			}
			f = v
		default:
			return nil, v1runtime("%s needs string or number", op)
		}
		if op == "to_int" {
			f = math.Trunc(f)
		}
		return f, nil
	case "parse_json":
		s, e := s0()
		if e != nil {
			return nil, e
		}
		var v interface{}
		if e := json.Unmarshal([]byte(s), &v); e != nil {
			return nil, v1runtime("parse_json: %v", e)
		}
		return v, nil
	case "encode_json":
		if e := need(1); e != nil {
			return nil, e
		}
		var b []byte
		var e error
		if len(a) > 1 && truthy(a[1]) {
			b, e = json.MarshalIndent(a[0], "", "  ")
		} else {
			b, e = json.Marshal(a[0])
		}
		return string(b), e
	case "parse_csv":
		s, e := s0()
		if e != nil {
			return nil, e
		}
		return parseCSV(s)
	case "parse_kv":
		s, e := s0()
		if e != nil {
			return nil, e
		}
		sep := "="
		if len(a) > 1 {
			sep = toString(a[1])
		}
		out := map[string]interface{}{}
		for _, l := range strings.Split(s, "\n") {
			if p := strings.SplitN(l, sep, 2); len(p) == 2 {
				out[strings.TrimSpace(p[0])] = strings.TrimSpace(p[1])
			}
		}
		return out, nil
	case "path_join":
		parts := []string{}
		for _, v := range a {
			parts = append(parts, toString(v))
		}
		return path.Join(parts...), nil
	case "get":
		if e := need(2); e != nil {
			return nil, e
		}
		var def interface{}
		if len(a) > 2 {
			def = a[2]
		}
		switch c := a[0].(type) {
		case map[string]interface{}:
			if v, ok := c[toString(a[1])]; ok {
				return v, nil
			}
		case []interface{}:
			i, e := num(a[1])
			if e != nil {
				return nil, e
			}
			if int(i) < 0 {
				i += float64(len(c))
			}
			if int(i) >= 0 && int(i) < len(c) {
				return c[int(i)], nil
			}
		}
		return def, nil
	case "keys", "values", "entries":
		if e := need(1); e != nil {
			return nil, e
		}
		o, e := obj(a[0])
		if e != nil {
			return nil, e
		}
		ks := make([]string, 0, len(o))
		for k := range o {
			ks = append(ks, k)
		}
		sort.Strings(ks)
		out := []interface{}{}
		for _, k := range ks {
			switch op {
			case "keys":
				out = append(out, k)
			case "values":
				out = append(out, o[k])
			default:
				out = append(out, map[string]interface{}{"key": k, "value": o[k]})
			}
		}
		return out, nil
	case "from_entries":
		if e := need(1); e != nil {
			return nil, e
		}
		l, e := list(a[0])
		if e != nil {
			return nil, e
		}
		out := map[string]interface{}{}
		for _, v := range l {
			if m, ok := v.(map[string]interface{}); ok {
				out[toString(m["key"])] = m["value"]
			} else if p, ok := v.([]interface{}); ok && len(p) == 2 {
				out[toString(p[0])] = p[1]
			} else {
				return nil, v1runtime("from_entries needs {key,value} or [k,v]")
			}
		}
		return out, nil
	case "sort", "sort_by", "reverse", "unique", "flatten", "count", "sum", "min", "max", "pluck", "group_by":
		if e := need(1); e != nil {
			return nil, e
		}
		l, e := list(a[0])
		if e != nil {
			return nil, e
		}
		cp := append([]interface{}{}, l...)
		fieldOf := func(v interface{}) interface{} {
			if len(a) < 2 {
				return v
			}
			if m, ok := v.(map[string]interface{}); ok {
				return m[toString(a[1])]
			}
			return nil
		}
		switch op {
		case "sort", "sort_by":
			sort.SliceStable(cp, func(i, j int) bool { return less(fieldOf(cp[i]), fieldOf(cp[j])) })
			if len(a) > 2 && truthy(a[2]) {
				for i, j := 0, len(cp)-1; i < j; i, j = i+1, j-1 {
					cp[i], cp[j] = cp[j], cp[i]
				}
			}
			return cp, nil
		case "reverse":
			for i, j := 0, len(cp)-1; i < j; i, j = i+1, j-1 {
				cp[i], cp[j] = cp[j], cp[i]
			}
			return cp, nil
		case "unique":
			out := []interface{}{}
			for _, v := range cp {
				dup := false
				for _, w := range out {
					if reflect.DeepEqual(v, w) {
						dup = true
						break
					}
				}
				if !dup {
					out = append(out, v)
				}
			}
			return out, nil
		case "flatten":
			out := []interface{}{}
			for _, v := range cp {
				if inner, ok := v.([]interface{}); ok {
					out = append(out, inner...)
				} else {
					out = append(out, v)
				}
			}
			return out, nil
		case "count":
			return float64(len(cp)), nil
		case "pluck":
			out := []interface{}{}
			for _, v := range cp {
				out = append(out, fieldOf(v))
			}
			return out, nil
		case "group_by":
			out := map[string]interface{}{}
			for _, v := range cp {
				k := toString(fieldOf(v))
				g, _ := out[k].([]interface{})
				out[k] = append(g, v)
			}
			return out, nil
		}
		if len(cp) == 0 {
			if op == "sum" {
				return 0.0, nil
			}
			return nil, nil
		}
		acc := 0.0
		best := cp[0]
		for _, v := range cp {
			f, e := num(fieldOf(v))
			if e != nil {
				return nil, e
			}
			acc += f
			bf, _ := num(fieldOf(best))
			if (op == "min" && f < bf) || (op == "max" && f > bf) {
				best = v
			}
		}
		if op == "sum" {
			return acc, nil
		}
		return fieldOf(best), nil
	case "push":
		if e := need(2); e != nil {
			return nil, e
		}
		l, e := list(a[0])
		if e != nil {
			return nil, e
		}
		return append(append([]interface{}{}, l...), a[1:]...), nil
	case "range":
		if e := need(1); e != nil {
			return nil, e
		}
		n, e := num(a[0])
		if e != nil || n > 100000 {
			return nil, v1runtime("range bound")
		}
		out := []interface{}{}
		for i := 0; i < int(n); i++ {
			out = append(out, float64(i))
		}
		return out, nil
	case "merge", "deep_merge":
		var acc interface{} = map[string]interface{}{}
		for _, v := range a {
			if _, e := obj(v); e != nil {
				return nil, e
			}
			if op == "deep_merge" {
				acc = deepMerge(acc, v)
			} else {
				m := map[string]interface{}{}
				for k, x := range acc.(map[string]interface{}) {
					m[k] = x
				}
				for k, x := range v.(map[string]interface{}) {
					m[k] = x
				}
				acc = m
			}
		}
		return acc, nil
	case "set_key", "del_key", "has":
		if e := need(2); e != nil {
			return nil, e
		}
		o, e := obj(a[0])
		if e != nil {
			return nil, e
		}
		k := toString(a[1])
		if op == "has" {
			_, ok := o[k]
			return ok, nil
		}
		m := map[string]interface{}{}
		for kk, v := range o {
			m[kk] = v
		}
		if op == "del_key" {
			delete(m, k)
		} else {
			if e := need(3); e != nil {
				return nil, e
			}
			m[k] = a[2]
		}
		return m, nil
	case "eq", "ne":
		if e := need(2); e != nil {
			return nil, e
		}
		eq := reflect.DeepEqual(a[0], a[1])
		return eq == (op == "eq"), nil
	case "lt", "le", "gt", "ge":
		if e := need(2); e != nil {
			return nil, e
		}
		l, g := less(a[0], a[1]), less(a[1], a[0])
		return map[string]bool{"lt": l, "le": !g, "gt": g, "ge": !l}[op], nil
	case "and":
		for _, v := range a {
			if !truthy(v) {
				return false, nil
			}
		}
		return true, nil
	case "or":
		for _, v := range a {
			if truthy(v) {
				return true, nil
			}
		}
		return false, nil
	case "not":
		if e := need(1); e != nil {
			return nil, e
		}
		return !truthy(a[0]), nil
	case "add", "sub", "mul", "div", "mod":
		if e := need(2); e != nil {
			return nil, e
		}
		p, e1 := num(a[0])
		q, e2 := num(a[1])
		if e1 != nil || e2 != nil {
			return nil, v1runtime("%s needs numbers", op)
		}
		switch op {
		case "add":
			return p + q, nil
		case "sub":
			return p - q, nil
		case "mul":
			return p * q, nil
		}
		if q == 0 {
			return nil, v1runtime("division by zero")
		}
		if op == "div" {
			return p / q, nil
		}
		return math.Mod(p, q), nil
	case "round":
		if e := need(1); e != nil {
			return nil, e
		}
		p, e := num(a[0])
		if e != nil {
			return nil, e
		}
		d := 0.0
		if len(a) > 1 {
			if d, e = num(a[1]); e != nil {
				return nil, e
			}
		}
		m := math.Pow(10, d)
		return math.Round(p*m) / m, nil
	case "semver_gt":
		if e := need(2); e != nil {
			return nil, e
		}
		return compare(a[0], a[1], "semver_gt")
	case "is_null":
		if e := need(1); e != nil {
			return nil, e
		}
		return a[0] == nil, nil
	case "type":
		if e := need(1); e != nil {
			return nil, e
		}
		switch a[0].(type) {
		case nil:
			return "null", nil
		case bool:
			return "bool", nil
		case float64:
			return "number", nil
		case string:
			return "string", nil
		case []interface{}:
			return "list", nil
		}
		return "object", nil
	case "render_lines", "render_md_list":
		if e := need(1); e != nil {
			return nil, e
		}
		l, e := list(a[0])
		if e != nil {
			return nil, e
		}
		var b strings.Builder
		for _, v := range l {
			if op == "render_md_list" {
				b.WriteString("- ")
			}
			b.WriteString(toString(v))
			b.WriteString("\n")
		}
		return b.String(), nil
	case "render_csv", "render_md_table":
		if e := need(2); e != nil {
			return nil, e
		}
		l, e := list(a[0])
		if e != nil {
			return nil, e
		}
		cols, e := columns(a[1])
		if e != nil {
			return nil, e
		}
		if op == "render_csv" {
			var b bytes.Buffer
			w := csv.NewWriter(&b)
			_ = w.Write(cols)
			for _, r := range l {
				row := make([]string, len(cols))
				for i, c := range cols {
					row[i] = cell(r, c)
				}
				_ = w.Write(row)
			}
			w.Flush()
			return b.String(), nil
		}
		var b strings.Builder
		b.WriteString("| " + strings.Join(cols, " | ") + " |\n|")
		for range cols {
			b.WriteString(" --- |")
		}
		b.WriteString("\n")
		for _, r := range l {
			row := make([]string, len(cols))
			for i, c := range cols {
				row[i] = strings.ReplaceAll(cell(r, c), "|", "\\|")
			}
			b.WriteString("| " + strings.Join(row, " | ") + " |\n")
		}
		return b.String(), nil
	}
	return nil, v1runtime("unknown op %q", op)
}

func clamp(i, n int) int {
	if i < 0 {
		i += n
	}
	if i < 0 {
		return 0
	}
	if i > n {
		return n
	}
	return i
}

func (x *v1Exec) pathOf(v interface{}, vars map[string]interface{}) (string, error) {
	p, e := x.eval(v, vars)
	if e != nil {
		return "", e
	}
	return str(p)
}

func (x *v1Exec) readFile(p, format string) (interface{}, error) {
	full, e := x.readPath(p)
	if e != nil {
		return nil, e
	}
	fi, e := os.Stat(full)
	if e != nil || !fi.Mode().IsRegular() {
		return nil, v1runtime("read_file: %s is not a readable file", p)
	}
	b, e := os.ReadFile(full)
	if e != nil {
		return nil, v1runtime("read_file: %v", e)
	}
	x.receipt = append(x.receipt, ReceiptEffect{"READ_FILE", "filesystem", p, "allowed"})
	s := string(b)
	switch format {
	case "text":
		return s, nil
	case "lines":
		out := []interface{}{}
		for _, l := range strings.Split(strings.TrimSuffix(s, "\n"), "\n") {
			if l != "" {
				out = append(out, l)
			}
		}
		return out, nil
	case "json":
		var v interface{}
		if e := strict(b, &v); e != nil {
			return nil, v1runtime("read_file json: %v", e)
		}
		return v, nil
	case "csv":
		return parseCSV(s)
	}
	m := map[string]interface{}{}
	for _, line := range strings.Split(s, "\n") {
		if line == "" {
			continue
		}
		pair := strings.SplitN(line, "=", 2)
		if len(pair) != 2 {
			return nil, v1runtime("invalid kv line")
		}
		if _, ok := m[pair[0]]; ok {
			return nil, v1runtime("duplicate kv key")
		}
		m[pair[0]] = pair[1]
	}
	return m, nil
}

func (x *v1Exec) write(p string, b []byte) error {
	if !underRoot(p, x.writes) || !symlinkFree(x.root, p) {
		return x.refuse("WRITE_FILE", p)
	}
	full := filepath.Join(x.root, filepath.FromSlash(p))
	if _, e := os.Stat(filepath.Dir(full)); e != nil {
		if !underRoot(path.Dir(p), x.writes) {
			return x.refuse("MKDIR", path.Dir(p))
		}
		if e := os.MkdirAll(filepath.Dir(full), 0700); e != nil {
			return v1runtime("mkdir: %v", e)
		}
		x.receipt = append(x.receipt, ReceiptEffect{"MKDIR", "filesystem", path.Dir(p), "allowed"})
	}
	x.written += len(b)
	if x.written > V1MaxWriteSize {
		return v1runtime("write budget exceeded")
	}
	if e := os.WriteFile(full, b, 0600); e != nil {
		return v1runtime("write: %v", e)
	}
	x.receipt = append(x.receipt, ReceiptEffect{"WRITE_FILE", "filesystem", p, "allowed"})
	return nil
}

func (x *v1Exec) run(steps []interface{}, vars map[string]interface{}) error {
	for _, raw := range steps {
		if e := x.tick(); e != nil {
			return e
		}
		s := raw.(map[string]interface{})
		act := s["action"].(string)
		switch act {
		case "read_file":
			p, e := x.pathOf(s["path"], vars)
			if e != nil {
				return e
			}
			v, e := x.readFile(p, s["format"].(string))
			if e != nil {
				return e
			}
			vars[s["as"].(string)] = v
		case "list_dir":
			p, e := x.pathOf(s["path"], vars)
			if e != nil {
				return e
			}
			full, e := x.readPath(p)
			if e != nil {
				return e
			}
			out := []interface{}{}
			rec := truthy(s["recursive"])
			e = filepath.Walk(full, func(q string, fi os.FileInfo, err error) error {
				if err != nil {
					return err
				}
				if fi.Mode()&os.ModeSymlink != 0 {
					return nil
				}
				if fi.IsDir() && q != full && !rec {
					return filepath.SkipDir
				}
				if fi.Mode().IsRegular() {
					rel, _ := filepath.Rel(x.root, q)
					out = append(out, filepath.ToSlash(rel))
				}
				return nil
			})
			if e != nil {
				return v1runtime("list_dir: %v", e)
			}
			sort.Slice(out, func(i, j int) bool { return out[i].(string) < out[j].(string) })
			x.receipt = append(x.receipt, ReceiptEffect{"LIST_DIR", "filesystem", p, "allowed"})
			vars[s["as"].(string)] = out
		case "set":
			v, e := x.eval(s["value"], vars)
			if e != nil {
				return e
			}
			vars[s["as"].(string)] = v
		case "append":
			name := s["to"].(string)
			cur, ok := vars[name]
			if !ok {
				cur = []interface{}{}
			}
			l, e := list(cur)
			if e != nil {
				return e
			}
			v, e := x.eval(s["value"], vars)
			if e != nil {
				return e
			}
			vars[name] = append(append([]interface{}{}, l...), v)
		case "filter":
			xs, e := records(vars[s["from"].(string)])
			if e != nil {
				return e
			}
			w, _ := s["where"].(map[string]interface{})
			fname, _ := w["field"].(string)
			opName, _ := w["op"].(string)
			want, e := x.eval(w["value"], vars)
			if e != nil {
				return e
			}
			out := []interface{}{}
			for _, r := range xs {
				if e := x.tick(); e != nil {
					return e
				}
				v, e := field(r, fname)
				if e != nil {
					return e
				}
				var ok bool
				switch opName {
				case "starts_with", "ends_with", "regex_match":
					res, e := x.op(opName, []interface{}{v, want})
					if e != nil {
						return e
					}
					ok = truthy(res)
				default:
					if ok, e = compare(v, want, opName); e != nil {
						return e
					}
				}
				if ok {
					out = append(out, r)
				}
			}
			vars[s["as"].(string)] = out
		case "for_each":
			l, e := list(vars[s["from"].(string)])
			if e != nil {
				return e
			}
			name := s["as"].(string)
			prev, had := vars[name]
			for _, item := range l {
				vars[name] = item
				if e := x.run(s["steps"].([]interface{}), vars); e != nil {
					return e
				}
			}
			if had {
				vars[name] = prev
			} else {
				delete(vars, name)
			}
		case "if":
			var ok bool
			if c, isMap := s["cond"].(map[string]interface{}); isMap && c["$op"] == nil && c["$var"] == nil {
				b, _ := json.Marshal(c)
				var legacy Condition
				if e := json.Unmarshal(b, &legacy); e != nil {
					return v1invalid("cond: %v", e)
				}
				var e error
				if ok, e = condition(legacy, vars); e != nil {
					return e
				}
			} else {
				v, e := x.eval(s["cond"], vars)
				if e != nil {
					return e
				}
				ok = truthy(v)
			}
			branch := s["else"].([]interface{})
			if ok {
				branch = s["then"].([]interface{})
			}
			if e := x.run(branch, vars); e != nil {
				return e
			}
		case "write_text":
			p, e := x.pathOf(s["path"], vars)
			if e != nil {
				return e
			}
			v, e := x.eval(s["value"], vars)
			if e != nil {
				return e
			}
			if e := x.write(p, []byte(toString(v))); e != nil {
				return e
			}
		case "write_record", "request_approval":
			p := "out/approval_request.json"
			var rec interface{}
			var e error
			if act == "write_record" {
				if p, e = x.pathOf(s["path"], vars); e != nil {
					return e
				}
				rec, e = x.eval(s["record"], vars)
			} else {
				rec, e = x.eval(map[string]interface{}{"reason": s["reason"], "summary": s["summary"]}, vars)
			}
			if e != nil {
				return e
			}
			if _, ok := rec.(map[string]interface{}); !ok {
				return v1runtime("record must be an object")
			}
			b, e := json.MarshalIndent(rec, "", "  ")
			if e != nil {
				return e
			}
			if e := x.write(p, append(b, '\n')); e != nil {
				return e
			}
		}
	}
	return nil
}

// ExecutePlanV1 runs a validated plan under the exact grant list.
func ExecutePlanV1(plan map[string]interface{}, root string, grants []string) ([]ReceiptEffect, error) {
	reads, writes := grantRoots(grants)
	x := &v1Exec{root: root, reads: reads, writes: writes, receipt: []ReceiptEffect{}, regexps: map[string]*regexp.Regexp{}}
	e := x.run(plan["steps"].([]interface{}), map[string]interface{}{})
	return x.receipt, e
}
