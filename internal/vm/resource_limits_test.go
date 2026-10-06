package vm

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/howlcipher/howlframe/internal/bytecode"
	"github.com/howlcipher/howlframe/internal/capability"
)

func runResourceTest(t *testing.T, source string, policy ExecutionPolicy) *bytecode.RuntimeFailure {
	t.Helper()
	_, prog := parseAndCompile(t, source)
	var out, stderr bytes.Buffer
	ev := RunBytecodeWithEvidence(prog, nil, policy, []capability.Capability{capability.Network, capability.Filesystem, capability.Process}, nil, &out, &stderr, 0)
	if ev.ExitCode != 0 {
		t.Fatalf("unexpected exit %d: %s", ev.ExitCode, &stderr)
	}
	return ev.RuntimeFailure
}
func requireResourceFailure(t *testing.T, failure *bytecode.RuntimeFailure, message string) {
	t.Helper()
	if failure == nil || failure.Code != "LIMIT_EXCEEDED" || !strings.Contains(failure.Message, message) {
		t.Fatalf("want LIMIT_EXCEEDED %q, got %#v", message, failure)
	}
}
func TestResourceDefaults(t *testing.T) {
	for _, limits := range []VMLimits{DefaultLimits, DefaultExecutionPolicy().Limits} {
		if limits.MaxMemoryBytes != 64*1024*1024 || limits.MaxFetchBodyBytes != 10*1024*1024 || limits.MaxExecOutputBytes != 10*1024*1024 {
			t.Fatalf("defaults: %+v", limits)
		}
	}
	if DefaultExecutionPolicy().Deadline != 0 {
		t.Fatal("deadline must be optional")
	}
}
func TestMemoryAllocationLimits(t *testing.T) {
	file := t.TempDir() + "/bytes"
	if err := os.WriteFile(file, []byte("0123456789"), 0600); err != nil {
		t.Fatal(err)
	}
	for name, source := range map[string]string{
		"concat":      `(cli_app (print (+ "abcd" "efgh")))`,
		"doubling":    `(cli_app (let (s "ab") (do (while (= 1 1) (set s (+ s s))))))`,
		"list":        `(cli_app (print (list 1 2)))`,
		"dict":        `(cli_app (print (dict ("key" 1))))`,
		"append":      `(cli_app (let (xs (list)) (append xs "abcdefgh")))`,
		"split":       `(cli_app (print (str_split "a,b,c" ",")))`,
		"join":        `(cli_app (print (str_join (list) "abcdefgh")) (print (str_join (str_split "ab" "") "abcdefgh")))`,
		"read_file":   fmt.Sprintf(`(cli_app (read_file %q))`, file),
		"encode_json": `(cli_app (print (encode_json "abcdefgh")))`,
	} {
		t.Run(name, func(t *testing.T) {
			policy := DefaultExecutionPolicy()
			policy.Limits.MaxMemoryBytes = 1
			requireResourceFailure(t, runResourceTest(t, source, policy), "memory limit")
		})
	}
	policy := DefaultExecutionPolicy()
	policy.Limits.MaxMemoryBytes = 8
	if e := runResourceTest(t, `(cli_app (print (+ "abcd" "efgh")) (print (+ 1 2)))`, policy); e != nil {
		t.Fatalf("under limit: %#v", e)
	}
}
func TestMemorySharedWithSpawn(t *testing.T) {
	policy := DefaultExecutionPolicy()
	policy.Limits.MaxMemoryBytes = 12
	requireResourceFailure(t, runResourceTest(t, `(cli_app (print (+ "abcd" "efgh")) (spawn_agent "child" (task "work" (print (+ "abcd" "efgh")))))`, policy), "memory limit")
}
func TestFetchBodyCap(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, "0123456789") }))
	defer server.Close()
	source := fmt.Sprintf(`(cli_app (fetch %q "GET"))`, server.URL)
	for _, max := range []int{-1, 0, 9, 10, 11} {
		policy := DefaultExecutionPolicy()
		policy.Limits.MaxFetchBodyBytes = max
		e := runResourceTest(t, source, policy)
		if max < 10 {
			requireResourceFailure(t, e, "fetch body")
		} else if e != nil {
			t.Fatalf("max %d: %#v", max, e)
		}
	}
}
func TestExecOutputCap(t *testing.T) {
	if _, err := os.Stat("/bin/sh"); err != nil {
		t.Skip("/bin/sh unavailable")
	}
	for _, max := range []int{-1, 0, 9, 10} {
		policy := DefaultExecutionPolicy()
		policy.Limits.MaxExecOutputBytes = max
		e := runResourceTest(t, `(cli_app (exec "/bin/sh" "-c" "printf 01234; printf 56789 >&2"))`, policy)
		if max < 10 {
			requireResourceFailure(t, e, "exec output")
		} else if e != nil {
			t.Fatalf("under cap: %#v", e)
		}
	}
}
func TestDeadlineSleepAndLoop(t *testing.T) {
	for _, source := range []string{`(cli_app (sleep 5000))`, `(cli_app (while (= 1 1) (+ 1 1)))`, `(cli_app (exec "/bin/sh" "-c" "sleep 5"))`} {
		policy := DefaultExecutionPolicy()
		policy.Deadline = 50 * time.Millisecond
		policy.Limits.MaxInstructions = int(^uint(0) >> 1)
		start := time.Now()
		requireResourceFailure(t, runResourceTest(t, source, policy), "deadline")
		if time.Since(start) > 2*time.Second {
			t.Fatal("deadline was not prompt")
		}
	}
	policy := DefaultExecutionPolicy()
	policy.Deadline = 5 * time.Second
	if e := runResourceTest(t, `(cli_app (sleep 10))`, policy); e != nil {
		t.Fatalf("short sleep: %#v", e)
	}
}
func TestDeadlineFetch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { <-r.Context().Done() }))
	defer server.Close()
	policy := DefaultExecutionPolicy()
	policy.Deadline = 50 * time.Millisecond
	requireResourceFailure(t, runResourceTest(t, fmt.Sprintf(`(cli_app (fetch %q "GET"))`, server.URL), policy), "deadline")
}
func TestDeadlineModel(t *testing.T) {
	// Cancelled context must stop a model request before it reaches the host.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	machine := &BCVM{ctx: ctx}
	defer func() {
		e, ok := recover().(*VMError)
		if !ok || e.Code != "LIMIT_EXCEEDED" || !strings.Contains(e.Message, "deadline") {
			t.Fatalf("failure: %#v", e)
		}
	}()
	machine.postModel("http://localhost:11434/api/generate", nil, 0, bytecode.OpConfidence)
}

// In-memory transport exercises the HTTP paths even where loopback is denied.
type resourceTransport func(*http.Request) (*http.Response, error)

func (f resourceTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestFetchLimitsInMemory(t *testing.T) {
	old := http.DefaultClient
	defer func() { http.DefaultClient = old }()
	http.DefaultClient = &http.Client{Transport: resourceTransport(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path == "/block" {
			<-r.Context().Done()
			return nil, r.Context().Err()
		}
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader("0123456789"))}, nil
	})}
	for _, max := range []int{-1, 0, 9, 10, 11} {
		policy := DefaultExecutionPolicy()
		policy.Limits.MaxFetchBodyBytes = max
		e := runResourceTest(t, `(cli_app (fetch "http://test/body" "GET"))`, policy)
		if max < 10 {
			requireResourceFailure(t, e, "fetch body")
		} else if e != nil {
			t.Fatalf("under cap: %#v", e)
		}
	}
	policy := DefaultExecutionPolicy()
	policy.Limits.MaxMemoryBytes = 89
	requireResourceFailure(t, runResourceTest(t, `(cli_app (fetch "http://test/body" "GET"))`, policy), "memory limit")
	policy = DefaultExecutionPolicy()
	policy.Deadline = 50 * time.Millisecond
	requireResourceFailure(t, runResourceTest(t, `(cli_app (fetch "http://test/block" "GET"))`, policy), "deadline")
}
func TestAllocationOpcodeBoundaries(t *testing.T) {
	for _, tc := range []struct {
		source, opcode string
		limit          int
	}{
		{`(cli_app (print (str_join (list "abcd" "efgh") "")))`, "STR_JOIN", 24},
		{`(cli_app (let (xs (list 1)) (append xs "ab")))`, "APPEND", 8},
		{`(cli_app (print (str_split "a,b" ",")))`, "STR_SPLIT", 1},
		{`(cli_app (print (dict ("key" 1))))`, "MAKE_DICT", 1},
		{`(cli_app (print (list 1)))`, "MAKE_LIST", 1},
	} {
		policy := DefaultExecutionPolicy()
		policy.Limits.MaxMemoryBytes = tc.limit
		e := runResourceTest(t, tc.source, policy)
		requireResourceFailure(t, e, "memory limit")
		if e.Opcode != tc.opcode {
			t.Fatalf("want %s, got %#v", tc.opcode, e)
		}
	}
}

func TestMemoryAccountingBoundaries(t *testing.T) {
	for _, max := range []int{-1, 0, 8, int(^uint(0) >> 1)} {
		t.Run(fmt.Sprint(max), func(t *testing.T) {
			machine := &BCVM{allocations: &allocationBudget{}, Limits: DefaultLimits}
			machine.Limits.MaxMemoryBytes = max
			machine.chargeAlloc(0, 0, bytecode.OpMakeList)
			machine.chargeAlloc(-1, 0, bytecode.OpMakeList)
			if max > 0 {
				machine.chargeAlloc(max, 0, bytecode.OpMakeList)
			}
			defer func() {
				e, ok := recover().(*VMError)
				if !ok || e.Code != "LIMIT_EXCEEDED" || !strings.Contains(e.Message, "memory limit") {
					t.Fatalf("overflow/fail-closed check: %#v", e)
				}
			}()
			machine.chargeAlloc(1, 0, bytecode.OpMakeList)
		})
	}
}
func TestExecLargeOutputAndExit(t *testing.T) {
	policy := DefaultExecutionPolicy()
	policy.Limits.MaxExecOutputBytes = 100
	requireResourceFailure(t, runResourceTest(t, `(cli_app (exec "/bin/sh" "-c" "printf '%10000s' ''"))`, policy), "exec output")
	e := runResourceTest(t, `(cli_app (exec "/bin/sh" "-c" "printf error >&2; exit 1"))`, DefaultExecutionPolicy())
	if e == nil || e.Code != "IO_ERROR" {
		t.Fatalf("nonzero exit: %#v", e)
	}
	policy = DefaultExecutionPolicy()
	policy.Limits.MaxMemoryBytes = 89
	requireResourceFailure(t, runResourceTest(t, `(cli_app (exec "/bin/sh" "-c" "printf 0123456789"))`, policy), "memory limit")
}

type deadlineBody struct{ ctx context.Context }

func (b deadlineBody) Read(p []byte) (int, error) { <-b.ctx.Done(); return 0, b.ctx.Err() }
func (b deadlineBody) Close() error               { return nil }
func TestDeadlineHTTPResponseBodies(t *testing.T) {
	old := http.DefaultClient
	defer func() { http.DefaultClient = old }()
	http.DefaultClient = &http.Client{Transport: resourceTransport(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: deadlineBody{r.Context()}}, nil
	})}
	for _, source := range []string{
		`(cli_app (fetch "http://test/body" "GET"))`,
		`(cli_app (print (confidence "statement")))`,
		`(cli_app (print (neural_circuit () "instruction")))`,
		`(cli_app (print (ephemeral_circuit () "instruction")))`,
		`(cli_app (lazy_synthesize my_add (a b) "Returns the sum of a and b") (print (call my_add 1 2)))`,
	} {
		policy := DefaultExecutionPolicy()
		policy.Deadline = 50 * time.Millisecond
		start := time.Now()
		requireResourceFailure(t, runResourceTest(t, source, policy), "deadline")
		if time.Since(start) > 2*time.Second {
			t.Fatal("response body ignored deadline")
		}
	}
}

func TestExecWithoutDeadlineWaitsForInheritedOutput(t *testing.T) {
	e := runResourceTest(t, `(cli_app (exec "/bin/sh" "-c" "(sleep 0.2; printf ready) &"))`, DefaultExecutionPolicy())
	if e != nil {
		t.Fatalf("default exec pipe behavior changed: %#v", e)
	}
}
