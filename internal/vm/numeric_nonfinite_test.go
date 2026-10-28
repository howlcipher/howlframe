package vm

import (
	"bytes"
	"io"
	"math"
	"net"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/howlcipher/howlframe/internal/ast"
	"github.com/howlcipher/howlframe/internal/capability"
)

// NUMERIC_CONTRACT section 7: +, - and * never return NaN or an infinity.
// Float overflow used to return +Inf on the interpreter and the VM, and
// Inf - Inf returned NaN, which then passed a range check such as
// (or (< x 0.0) (> x 100.0)).

func interpBinop(op string, a, b any) (result any, reason string) {
	defer func() {
		if r := recover(); r != nil {
			e, ok := r.(interpError)
			if !ok {
				panic(r)
			}
			reason = e.reason
		}
	}()
	return NumericBinop(op, a, b, &ast.Node{}), ""
}

func vmBinop(op string, a, b any) (result any, vmErr *VMError) {
	defer func() {
		if r := recover(); r != nil {
			e, ok := r.(*VMError)
			if !ok {
				panic(r)
			}
			vmErr = e
		}
	}()
	return BcBinop(op, a, b), nil
}

func TestFloatArithmeticRefusesNonFiniteResults(t *testing.T) {
	max := math.MaxFloat64
	cases := []struct {
		op   string
		a, b any
	}{
		{"*", 1e308, 10.0},
		{"+", max, max},
		{"-", -max, max},
		{"*", int64(math.MaxInt64), 1e300},
		{"*", 1e300, int64(math.MinInt64)},
		// Non-finite operands cannot be built from source any more; the
		// operators still refuse them rather than propagate them.
		{"+", math.Inf(1), 1.0},
		{"-", math.Inf(1), math.Inf(1)},
		{"*", math.NaN(), int64(2)},
		{"*", math.Inf(-1), 0.0},
	}
	for _, c := range cases {
		if res, reason := interpBinop(c.op, c.a, c.b); !strings.Contains(reason, "floating-point overflow") {
			t.Errorf("interpreter (%s %v %v) = %v, error %q; want floating-point overflow", c.op, c.a, c.b, res, reason)
		}
		res, err := vmBinop(c.op, c.a, c.b)
		if err == nil || err.Code != "RUNTIME_ERROR" || !strings.Contains(err.Message, "floating-point overflow") {
			t.Errorf("VM (%s %v %v) = %v, error %+v; want RUNTIME_ERROR floating-point overflow", c.op, c.a, c.b, res, err)
		}
	}
}

func TestFloatArithmeticKeepsFiniteResults(t *testing.T) {
	max := math.MaxFloat64
	// Go folds constant expressions exactly, so IEEE results are written out.
	cases := []struct {
		op   string
		a, b any
		want float64
	}{
		{"+", 0.1, 0.2, 0.30000000000000004},
		{"+", max, 0.0, max},
		{"*", max, 1.0, max},
		{"-", -max, 0.0, -max},
		{"-", max, max, 0},
		{"+", max * 0.5, max * 0.5, max},
		{"*", 1e154, 1e154, 1.0000000000000001e308},
		{"*", 1e-200, 1e-200, 0},
		{"*", math.SmallestNonzeroFloat64, 0.5, 0},
		{"*", -1.0, 0.0, math.Copysign(0, -1)},
		{"*", int64(math.MaxInt64), 1.0, float64(math.MaxInt64)},
		{"*", 2.5, int64(4), 10},
		{"-", int64(1), 0.5, 0.5},
	}
	for _, c := range cases {
		for name, run := range map[string]func() (any, string){
			"interpreter": func() (any, string) { return interpBinop(c.op, c.a, c.b) },
			"VM": func() (any, string) {
				res, err := vmBinop(c.op, c.a, c.b)
				if err != nil {
					return res, err.Message
				}
				return res, ""
			},
		} {
			res, reason := run()
			got, ok := res.(float64)
			if reason != "" || !ok || math.Float64bits(got) != math.Float64bits(c.want) {
				t.Errorf("%s (%s %v %v) = %v (%T), error %q; want %v", name, c.op, c.a, c.b, res, res, reason, c.want)
			}
		}
	}
}

// Issue #158: an ordering comparison with a NaN operand is false. Source
// programs can no longer produce NaN, so the guarantee is kept here, at the
// operators themselves.
func TestNaNOrderingIsFalseAtTheOperators(t *testing.T) {
	nan := math.NaN()
	for _, op := range []string{"<", "<=", ">", ">="} {
		for _, pair := range [][2]any{{nan, int64(1)}, {int64(1), nan}, {nan, 1.0}, {1.0, nan}, {nan, nan}} {
			if res, reason := interpBinop(op, pair[0], pair[1]); res != false || reason != "" {
				t.Errorf("interpreter (%s %v %v) = %v, %q; want false", op, pair[0], pair[1], res, reason)
			}
			if res, err := vmBinop(op, pair[0], pair[1]); res != false || err != nil {
				t.Errorf("VM (%s %v %v) = %v, %v; want false", op, pair[0], pair[1], res, err)
			}
		}
	}
}

// startModelHostResponder binds the hard-coded model host and answers every
// request with the current value of response.
func startModelHostResponder(t *testing.T, response *atomic.Value) {
	t.Helper()
	t.Setenv("NO_PROXY", "127.0.0.1,localhost")
	t.Setenv("no_proxy", "127.0.0.1,localhost")
	ln, err := net.Listen("tcp", "127.0.0.1:11434")
	if err != nil {
		t.Skipf("model host port unavailable (%v)", err)
	}
	srv := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"response":`+response.Load().(string)+`}`)
	})}
	go func() { _ = srv.Serve(ln) }()
	t.Cleanup(func() { _ = srv.Close() })
}

// confidence parses model text. "NaN", "Inf" and "1e400" used to reach the
// program as NaN or an infinity (NUMERIC_CONTRACT section 2.3).
func TestConfidenceRefusesNonFiniteModelResponse(t *testing.T) {
	var response atomic.Value
	startModelHostResponder(t, &response)
	node, program := parseAndCompile(t, `(cli_app (print (confidence "the sky is blue")))`)
	network := []capability.Capability{capability.Network}

	for _, text := range []string{`"NaN"`, `"nan"`, `"Inf"`, `"-Infinity"`, `"+inf"`, `" 1e400 "`, `"-1e400"`} {
		response.Store(text)
		outcome := runBytecodeOutcome(program, "", network)
		if outcome.vmError == nil || outcome.vmError.Code != "CONVERSION_ERROR" || !strings.Contains(outcome.vmError.Message, "not a finite number") || outcome.stdout != "" {
			t.Errorf("VM confidence with model response %s: %+v, want CONVERSION_ERROR and no output", text, outcome)
		}
		var stdout, stderr bytes.Buffer
		if code := Interpret(node, nil, network, strings.NewReader(""), &stdout, &stderr); code == 0 || stdout.Len() != 0 ||
			!strings.Contains(stderr.String(), "CONVERSION_ERROR") || !strings.Contains(stderr.String(), "not a finite number") {
			t.Errorf("interpreter confidence with model response %s: exit %d stdout %q stderr %q, want CONVERSION_ERROR", text, code, stdout.String(), stderr.String())
		}
	}

	for text, want := range map[string]string{`"0.75"`: "0.75", `" 0.25 "`: "0.25", `"1"`: "1"} {
		response.Store(text)
		if outcome := runBytecodeOutcome(program, "", network); outcome.vmError != nil || strings.TrimSpace(outcome.stdout) != want {
			t.Errorf("VM confidence with model response %s: %+v, want %s", text, outcome, want)
		}
		var stdout, stderr bytes.Buffer
		if code := Interpret(node, nil, network, strings.NewReader(""), &stdout, &stderr); code != 0 || strings.TrimSpace(stdout.String()) != want {
			t.Errorf("interpreter confidence with model response %s: exit %d stdout %q stderr %q, want %s", text, code, stdout.String(), stderr.String(), want)
		}
	}
}
