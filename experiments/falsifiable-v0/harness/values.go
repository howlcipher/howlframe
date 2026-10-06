package harness

import (
	"encoding/json"
	"reflect"
	"strconv"
	"strings"
)

func varValue(vars map[string]interface{}, name string) (interface{}, error) {
	parts := strings.Split(name, ".")
	v, ok := vars[parts[0]]
	if !ok {
		return nil, invalid("unknown variable %q", name)
	}
	for _, p := range parts[1:] {
		m, ok := v.(map[string]interface{})
		if !ok {
			return nil, invalid("not an object: %s", name)
		}
		v, ok = m[p]
		if !ok {
			return nil, invalid("missing field %s", name)
		}
	}
	return v, nil
}
func resolve(v interface{}, vars map[string]interface{}) (interface{}, error) {
	switch x := v.(type) {
	case map[string]interface{}:
		if name, ok := x["$var"]; ok {
			if len(x) != 1 {
				return nil, invalid("$var must be sole key")
			}
			s, ok := name.(string)
			if !ok {
				return nil, invalid("$var must be string")
			}
			return varValue(vars, s)
		}
		out := map[string]interface{}{}
		for k, v := range x {
			r, e := resolve(v, vars)
			if e != nil {
				return nil, e
			}
			out[k] = r
		}
		return out, nil
	case []interface{}:
		out := make([]interface{}, len(x))
		for i, v := range x {
			r, e := resolve(v, vars)
			if e != nil {
				return nil, e
			}
			out[i] = r
		}
		return out, nil
	}
	return v, nil
}
func validateRefs(v interface{}, vars map[string]bool) error {
	switch x := v.(type) {
	case map[string]interface{}:
		if v, ok := x["$var"]; ok {
			s, ok := v.(string)
			if !ok || len(x) != 1 || !vars[strings.Split(s, ".")[0]] {
				return invalid("invalid variable reference")
			}
			return nil
		}
		for _, v := range x {
			if e := validateRefs(v, vars); e != nil {
				return e
			}
		}
	case []interface{}:
		for _, v := range x {
			if e := validateRefs(v, vars); e != nil {
				return e
			}
		}
	}
	return nil
}
func semver(s interface{}) ([3]int, error) {
	var v [3]int
	x, ok := s.(string)
	if !ok {
		return v, invalid("version must be string")
	}
	parts := strings.Split(x, ".")
	if len(parts) != 3 {
		return v, invalid("version must be x.y.z")
	}
	for i, p := range parts {
		n, e := strconv.Atoi(p)
		if e != nil || n < 0 || strconv.Itoa(n) != p {
			return v, invalid("invalid version")
		}
		v[i] = n
	}
	return v, nil
}
func compare(a, b interface{}, op string) (bool, error) {
	switch op {
	case "eq":
		return reflect.DeepEqual(a, b), nil
	case "ne":
		return !reflect.DeepEqual(a, b), nil
	case "contains":
		x, ok := a.(string)
		y, ok2 := b.(string)
		if !ok || !ok2 {
			return false, invalid("contains needs strings")
		}
		return strings.Contains(x, y), nil
	case "in":
		xs, ok := b.([]interface{})
		if !ok {
			return false, invalid("in needs array")
		}
		for _, x := range xs {
			if reflect.DeepEqual(a, x) {
				return true, nil
			}
		}
		return false, nil
	case "semver_gt":
		x, e := semver(a)
		if e != nil {
			return false, e
		}
		y, e := semver(b)
		if e != nil {
			return false, e
		}
		for i := 0; i < 3; i++ {
			if x[i] != y[i] {
				return x[i] > y[i], nil
			}
		}
		return false, nil
	}
	x, ok := a.(float64)
	y, ok2 := b.(float64)
	if !ok || !ok2 {
		return false, invalid("numeric comparison needs numbers")
	}
	switch op {
	case "gt":
		return x > y, nil
	case "lt":
		return x < y, nil
	case "ge":
		return x >= y, nil
	case "le":
		return x <= y, nil
	}
	return false, invalid("unknown comparison")
}
func records(v interface{}) ([]interface{}, error) {
	if xs, ok := v.([]interface{}); ok {
		return xs, nil
	}
	if x, ok := v.(map[string]interface{}); ok {
		return []interface{}{x}, nil
	}
	return nil, invalid("expected records")
}
func field(v interface{}, name string) (interface{}, error) {
	if name == "" {
		return v, nil
	}
	m, ok := v.(map[string]interface{})
	if !ok {
		return nil, invalid("record must be object")
	}
	x, ok := m[name]
	if !ok {
		return nil, invalid("missing field %q", name)
	}
	return x, nil
}
func condition(c Condition, vars map[string]interface{}) (bool, error) {
	a, e := varValue(vars, c.Var)
	if e != nil {
		return false, e
	}
	b, e := resolve(c.Value, vars)
	if e != nil {
		return false, e
	}
	switch c.Op {
	case "equals":
		return compare(a, b, "eq")
	case "semver_gt":
		return compare(a, b, "semver_gt")
	case "count":
		xs, e := records(a)
		if e != nil {
			return false, e
		}
		return compare(float64(len(xs)), b, "eq")
	case "all", "any":
		xs, e := records(a)
		if e != nil {
			return false, e
		}
		for _, x := range xs {
			v, e := field(x, c.Field)
			if e != nil {
				return false, e
			}
			match, e := compare(v, b, "eq")
			if e != nil {
				return false, e
			}
			if c.Op == "all" && !match {
				return false, nil
			}
			if c.Op == "any" && match {
				return true, nil
			}
		}
		return c.Op == "all", nil
	}
	return false, invalid("unknown condition")
}

// CanonicalJSON compares JSON values independently of indentation/key ordering.
func CanonicalJSON(b []byte) ([]byte, error) {
	var v interface{}
	if e := strict(b, &v); e != nil {
		return nil, e
	}
	return json.Marshal(v)
}

func requireKeys(b []byte, keys []string) error {
	var m map[string]json.RawMessage
	if e := json.Unmarshal(b, &m); e != nil {
		return e
	}
	for _, k := range keys {
		if _, ok := m[k]; !ok {
			return invalid("missing field %q", k)
		}
	}
	return nil
}
func (p *Predicate) UnmarshalJSON(b []byte) error {
	type plain Predicate
	var x plain
	if e := strict(b, &x); e != nil {
		return e
	}
	if e := requireKeys(b, []string{"field", "op", "value"}); e != nil {
		return e
	}
	*p = Predicate(x)
	return nil
}
func (c *Condition) UnmarshalJSON(b []byte) error {
	type plain Condition
	var x plain
	if e := strict(b, &x); e != nil {
		return e
	}
	if e := requireKeys(b, []string{"op", "var", "value"}); e != nil {
		return e
	}
	*c = Condition(x)
	return nil
}
