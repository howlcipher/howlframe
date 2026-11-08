package agent

import (
	"bytes"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/howlcipher/howlframe/internal/construct"
)

func response(t *testing.T, args ...string) (Response, int, []byte) {
	t.Helper()
	var b bytes.Buffer
	status := Run(args, "unused", &b)
	var r Response
	if err := json.Unmarshal(b.Bytes(), &r); err != nil {
		t.Fatal(err)
	}
	return r, status, b.Bytes()
}
func TestRegistryAndDeterminism(t *testing.T) {
	r, status, first := response(t, "capabilities")
	_, _, second := response(t, "capabilities")
	if status != 0 || !bytes.Equal(first, second) || len(r.Constructs) != len(construct.Table()) {
		t.Fatalf("non-deterministic or incomplete catalog: %d", status)
	}
	if r.Schema != Schema || len(r.ActualGrants) != 0 {
		t.Fatal(r)
	}
	for i, e := range construct.Table() {
		got := r.Constructs[i]
		if got.Name != e.Name || got.Support != e.Support.String() {
			t.Fatal(got)
		}
		a, ok := construct.LookupArity(e.Name)
		if ok && (got.Arity == nil || got.Arity.Min != a.Min || got.Arity.Max != a.Max) {
			t.Fatal(got)
		}
		if !ok && got.ArityExemption == "" {
			t.Fatal(got)
		}
	}
}
func TestAuthorityAndUnknownCoverage(t *testing.T) {

	for name, want := range map[string]string{"read_file": "filesystem", "fetch": "network:fetch=host[:port]", "store_open": "database; filesystem", "exec": "process:exec=<absolute program path>"} {
		r, status, _ := response(t, "describe", name)
		if status != 0 || len(r.ActualGrants) != 0 || !strings.HasPrefix(r.Constructs[0].Requirement, want) {
			t.Fatal(r)
		}
	}

	r, status, _ := response(t, "capabilities", "--target", "wasmgc")
	if status != 0 || r.Coverage != "unknown" || len(r.Constructs) != 0 || len(r.Examples) != 0 {
		t.Fatal(r)
	}
}
func TestArgumentFailures(t *testing.T) {
	for _, tc := range []struct {
		args []string
		code string
	}{
		{[]string{}, "AGENT_ARGUMENT"}, {[]string{"describe"}, "AGENT_ARGUMENT"}, {[]string{"check", "missing"}, "AGENT_ARGUMENT"}, {[]string{"check", "--json", "--target", "typo", "missing"}, "AGENT_UNKNOWN_TARGET"}, {[]string{"check", "--json", "--target", "go", "missing"}, "AGENT_COVERAGE_UNKNOWN"}, {[]string{"describe", "invented"}, "AGENT_UNKNOWN_CONSTRUCT"}, {[]string{"explain", "INVENTED"}, "AGENT_UNKNOWN_CODE"}, {[]string{"check", "--json", "-source"}, "AGENT_ARGUMENT"}, {[]string{"capabilities", "--allow-caps", "filesystem"}, "AGENT_ARGUMENT"},
	} {
		r, status, _ := response(t, tc.args...)
		if status != 1 || len(r.Diagnostics) != 1 || r.Diagnostics[0].Code != tc.code {
			t.Fatalf("%v: %+v", tc.args, r)
		}
	}
}
func TestNormalizeStrict(t *testing.T) {
	d := normalize([]byte(`{"reason":"ARITY_ERROR: append expects 2","line":2,"column":3}`), "a.howl")
	if len(d) != 1 || d[0].Code != "ARITY_ERROR" || d[0].Line != 2 || d[0].Column != 3 {
		t.Fatal(d)
	}
	if normalize([]byte(`{"reason":"x"} trailing`), "a") != nil {
		t.Fatal("accepted mixed output")
	}
	got := normalize([]byte(`[{"code":"HFIR_TARGET_INFEASIBLE","message":"unsupported","location":{"filename":"a","line":4,"column":8}}]`), "b")
	if len(got) != 1 || got[0].Filename != "a" {
		t.Fatal(got)
	}
	if !reflect.DeepEqual(normalize([]byte(`{"reason":"old error","line":1}`), "a")[0].Classification, "legacy-uncoded") {
		t.Fatal("uncoded error overclaimed")
	}
}
