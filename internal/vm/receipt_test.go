package vm

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/howlcipher/howlframe/internal/bytecode"
	"github.com/howlcipher/howlframe/internal/capability"
)

func receiptRun(t *testing.T, source string, policy ExecutionPolicy, caps ...capability.Capability) (ExecutionReceipt, string, string) {
	t.Helper()
	_, prog := parseAndCompile(t, source)
	var out, stderr bytes.Buffer
	code, r := RunBytecodeWithReceipt(prog, nil, policy, caps, strings.NewReader(""), &out, &stderr, "trusted-hash", "test-version")
	if code != r.Exit.Code {
		t.Fatalf("code %d != receipt %d", code, r.Exit.Code)
	}
	b, err := r.JSON()
	if err != nil {
		t.Fatal(err)
	}
	return r, string(b), out.String()
}
func TestReceiptEmptyAndDeterministic(t *testing.T) {
	for _, source := range []string{`(cli_app)`, `(cli_app (print "hello"))`} {
		r, a, _ := receiptRun(t, source, DefaultExecutionPolicy())
		_, b, _ := receiptRun(t, source, DefaultExecutionPolicy())
		if a != b || !strings.Contains(a, `"effects": []`) || !strings.Contains(a, `"grant": []`) || r.Exit.Status != "ok" || !strings.HasSuffix(a, "\n") {
			t.Fatalf("receipt: %s / %s", a, b)
		}
	}
}
func TestReceiptFetchRedaction(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, "ok") }))
	defer server.Close()
	testReceiptFetch(t, server.URL)
}
func TestReceiptFetchRedactionInMemory(t *testing.T) {
	old := http.DefaultClient
	http.DefaultClient = &http.Client{Transport: receiptRoundTripFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader("ok")), Header: make(http.Header)}, nil
	})}
	defer func() { http.DefaultClient = old }()
	testReceiptFetch(t, "http://127.0.0.1:12345")
}
func testReceiptFetch(t *testing.T, origin string) {
	t.Helper()
	address := strings.Replace(origin, "http://", "http://user:pw@", 1) + "/path?token=secret#frag"
	r, raw, _ := receiptRun(t, fmt.Sprintf(`(cli_app (fetch %q "GET"))`, address), DefaultExecutionPolicy(), capability.Network)
	if r.Exit.Code != 0 || len(r.Effects) != 1 || r.Effects[0] != (ReceiptEffect{"FETCH", "network", origin, "allowed"}) {
		t.Fatalf("receipt: %s", raw)
	}
	for _, secret := range []string{"secret", "token", "pw", "/path", "user:"} {
		if strings.Contains(raw, secret) {
			t.Fatalf("leaked %q: %s", secret, raw)
		}
	}
}
func TestReceiptEnvironmentAndExec(t *testing.T) {
	t.Setenv("HF_RECEIPT_TEST", "secret-value-123")
	r, raw, _ := receiptRun(t, `(cli_app (env "HF_RECEIPT_TEST") (exec "/bin/echo" "secret-argument-456"))`, DefaultExecutionPolicy(), capability.Environment, capability.Process, capability.Environment)
	if r.Exit.Code != 0 || len(r.Effects) != 2 || r.Effects[0].TargetSummary != "HF_RECEIPT_TEST" || r.Effects[1].TargetSummary != "echo" || len(r.Grant) != 2 {
		t.Fatalf("receipt: %s", raw)
	}
	for _, secret := range []string{"secret-value-123", "secret-argument-456"} {
		if strings.Contains(raw, secret) {
			t.Fatalf("leaked %s", raw)
		}
	}
}
func TestReceiptDeniedAndLimit(t *testing.T) {
	r, raw, _ := receiptRun(t, `(cli_app (fetch "http://user:pw@example.com/path?token=secret" "GET"))`, DefaultExecutionPolicy())
	if r.Exit.Code != 1 || r.Exit.Error == nil || r.Exit.Error.Code != "CAPABILITY_DENIED" || len(r.Effects) != 1 || r.Effects[0].Decision != "denied" || r.Effects[0].TargetSummary != "http://example.com" {
		t.Fatal(raw)
	}
	p := DefaultExecutionPolicy()
	p.Limits.MaxInstructions = 1
	r, raw, _ = receiptRun(t, `(cli_app (print 42))`, p)
	if r.Exit.Error == nil || r.Exit.Error.Code != "LIMIT_EXCEEDED" || r.InstructionsUsed != 1 {
		t.Fatal(raw)
	}
}
func TestReceiptSpawnAgent(t *testing.T) {
	t.Setenv("HF_RECEIPT_CHILD", "child-secret")
	r, raw, _ := receiptRun(t, `(cli_app (spawn_agent "child" (task "work" (env "HF_RECEIPT_CHILD"))))`, DefaultExecutionPolicy(), capability.Process, capability.Environment)
	if len(r.Effects) != 2 || r.Effects[0].Op != "SPAWN_AGENT" || r.Effects[1].Op != "ENV" || r.Effects[1].TargetSummary != "HF_RECEIPT_CHILD" || r.InstructionsUsed < 4 {
		t.Fatal(raw)
	}
	if strings.Contains(raw, "child-secret") || strings.Contains(raw, "work") {
		t.Fatal(raw)
	}
}
func TestReceiptStoreDecisions(t *testing.T) {
	file := t.TempDir() + "/store.json"
	source := fmt.Sprintf(`(cli_app (store_open data %q) (store_put data "secret-key" (dict ("x" "secret-value"))) (store_get data "secret-key"))`, "file://"+file)
	r, raw, _ := receiptRun(t, source, DefaultExecutionPolicy(), capability.Database, capability.Filesystem)
	if r.Exit.Code != 0 || len(r.Effects) != 6 {
		t.Fatal(raw)
	}
	for i := 0; i < len(r.Effects); i += 2 {
		if r.Effects[i].Capability != "database" || r.Effects[i+1].Capability != "filesystem" {
			t.Fatal(raw)
		}
	}
	if strings.Contains(raw, "secret-key") || strings.Contains(raw, "secret-value") {
		t.Fatal(raw)
	}
}
func TestReceiptExitAndFailure(t *testing.T) {
	for _, source := range []string{`(cli_app (exit 7))`, `(cli_app (return 7))`} {
		r, raw, _ := receiptRun(t, source, DefaultExecutionPolicy())
		if r.Exit.Code != 7 || r.Exit.Status != "error" || r.Exit.Error != nil {
			t.Fatal(raw)
		}
	}
	r, raw, _ := receiptRun(t, `(cli_app (read_file "/missing/secret-file"))`, DefaultExecutionPolicy(), capability.Filesystem)
	if r.Exit.Error == nil || r.Exit.Error.Code != "IO_ERROR" || strings.Contains(r.Exit.Error.Message, "secret-file") {
		t.Fatal(raw)
	}
}
func TestReceiptCapAndFinalization(t *testing.T) {
	recorder := &receiptRecorder{effects: []ReceiptEffect{}}
	vm := &BCVM{receipt: recorder}
	for i := 0; i < maxReceiptEffects+1; i++ {
		vm.receiptDecision = &effectDecision{summary: "name"}
		vm.recordEffect(capability.Environment, bytecode.OpEnv, "allowed")
		vm.recordEffect(capability.Environment, bytecode.OpEnv, "allowed")
	}
	if len(recorder.effects) != maxReceiptEffects || !recorder.truncated {
		t.Fatal("cap or dedup failed")
	}
	recorder.finish(3)
	vm.receiptDecision = &effectDecision{}
	vm.recordEffect(capability.Network, bytecode.OpFetch, "denied")
	if recorder.instructions != 3 || len(recorder.effects) != maxReceiptEffects {
		t.Fatal("late effect recorded")
	}
}

type receiptRoundTripFunc func(*http.Request) (*http.Response, error)

func (f receiptRoundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestReceiptDeniedSummaryPolicy(t *testing.T) {
	for _, tc := range []struct {
		op       bytecode.Opcode
		operands []any
		inst     bytecode.BCInstruction
		want     string
	}{
		{op: bytecode.OpFetch, operands: []any{"%%%", "GET"}, want: "<invalid-url>"},
		{op: bytecode.OpWriteFile, operands: []any{"a/../file", "content-secret"}, want: "a/../file"},
		{op: bytecode.OpReadFile, operands: []any{"file"}, want: "file"},
		{op: bytecode.OpMkdir, operands: []any{"directory"}, want: "directory"},
		{op: bytecode.OpExec, operands: []any{"/bin/echo", "arg-secret"}, inst: bytecode.BCInstruction{IntOperand: 1}, want: "echo"},
		{op: bytecode.OpDbConnect, inst: bytecode.BCInstruction{StringOperand: "db", StringOperand2: "sqlite", StringOperand3: "dsn-secret"}, want: "sqlite"},
		{op: bytecode.OpSqlQuery, inst: bytecode.BCInstruction{StringOperand: "db", StringOperand2: "query-secret"}, want: "db"},
		{op: bytecode.OpHttpRoute, inst: bytecode.BCInstruction{StringOperand: "/route"}, want: "/route"},
		{op: bytecode.OpResJson, operands: []any{int64(200), "body-secret"}, want: ""},
		{op: bytecode.OpLlmGenerate, operands: []any{"prompt-secret"}, inst: bytecode.BCInstruction{StringOperand: "model"}, want: ""},
		{op: bytecode.OpEnv, operands: []any{strings.Repeat("x", 300)}, want: strings.Repeat("x", 256)},
	} {
		t.Run(bytecode.Registry[tc.op].Name, func(t *testing.T) {
			prog := &bytecode.BCProgram{}
			for _, value := range tc.operands {
				prog.Main = append(prog.Main, bytecode.BCInstruction{Op: bytecode.OpLoadConst, ValueOperand: value})
			}
			inst := tc.inst
			inst.Op = tc.op
			prog.Main = append(prog.Main, inst)
			var out, stderr bytes.Buffer
			_, r := RunBytecodeWithReceipt(prog, nil, DefaultExecutionPolicy(), nil, strings.NewReader(""), &out, &stderr, "hash", "version")
			if r.Exit.Error == nil || r.Exit.Error.Code != "CAPABILITY_DENIED" || len(r.Effects) != 1 || r.Effects[0].TargetSummary != tc.want {
				t.Fatalf("receipt %+v", r)
			}
			raw, err := r.JSON()
			if err != nil {
				t.Fatal(err)
			}
			if bytes.Contains(raw, []byte("secret")) {
				t.Fatal(string(raw))
			}
		})
	}
}

func TestReceiptConcurrentRecorder(t *testing.T) {
	recorder := &receiptRecorder{effects: []ReceiptEffect{}}
	var workers sync.WaitGroup
	for i := 0; i < 8; i++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			child := &BCVM{receipt: recorder}
			for j := 0; j < 100; j++ {
				child.receiptDecision = &effectDecision{summary: "name"}
				child.recordEffect(capability.Environment, bytecode.OpEnv, "allowed")
			}
		}()
	}
	workers.Wait()
	recorder.finish(0)
	if len(recorder.effects) != 800 {
		t.Fatalf("lost effects: %d", len(recorder.effects))
	}
}

func TestReceiptRegistryAndLazyDecisions(t *testing.T) {
	for op, spec := range bytecode.Registry {
		if spec.Capability == capability.None {
			continue
		}
		t.Run(spec.Name, func(t *testing.T) {
			prog := &bytecode.BCProgram{Main: []bytecode.BCInstruction{{Op: op}}}
			var out, stderr bytes.Buffer
			_, r := RunBytecodeWithReceipt(prog, nil, DefaultExecutionPolicy(), nil, strings.NewReader(""), &out, &stderr, "hash", "version")
			if len(r.Effects) != 1 || r.Effects[0].Op != spec.Name || r.Effects[0].Decision != "denied" || r.Exit.Error == nil || r.Exit.Error.Code != "CAPABILITY_DENIED" {
				t.Fatalf("receipt %+v", r)
			}
		})
	}
	prog := &bytecode.BCProgram{Functions: map[string]*bytecode.BCFunction{"lazy": {Name: "lazy", LazySynthesize: true, Docstring: "prompt-secret"}}, Main: []bytecode.BCInstruction{{Op: bytecode.OpCall, StringOperand: "lazy"}}}
	var out, stderr bytes.Buffer
	_, r := RunBytecodeWithReceipt(prog, nil, DefaultExecutionPolicy(), nil, strings.NewReader(""), &out, &stderr, "hash", "version")
	if len(r.Effects) != 1 || r.Effects[0].Op != "CALL" || r.Effects[0].Capability != "network" || r.Effects[0].Decision != "denied" || r.Effects[0].TargetSummary != "" {
		t.Fatalf("lazy receipt %+v", r)
	}
}

func TestReceiptHTTPChild(t *testing.T) {
	recorder := &receiptRecorder{effects: []ReceiptEffect{}}
	prog := &bytecode.BCProgram{Main: []bytecode.BCInstruction{
		{Op: bytecode.OpHttpServerStart, StringOperand: ":12345"},
		{Op: bytecode.OpHttpRoute, StringOperand: "/test", StringOperand2: "req", IntOperand: 2},
		{Op: bytecode.OpLoadConst, ValueOperand: "HF_RECEIPT_HTTP"},
		{Op: bytecode.OpEnv},
	}}
	env := NewBcEnv(nil)
	machine := &BCVM{prog: prog, env: env, receipt: recorder, Limits: DefaultLimits, AllowedCaps: []capability.Capability{capability.Network, capability.Environment}, Out: io.Discard, ErrOut: io.Discard}
	machine.run(prog.Main, env)
	mux, _ := env.get("__http_mux")
	mux.(*http.ServeMux).ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "http://localhost/test", nil))
	recorder.finish(machine.executed)
	if len(recorder.effects) != 3 || recorder.effects[2].Op != "ENV" || recorder.effects[2].TargetSummary != "HF_RECEIPT_HTTP" {
		t.Fatalf("HTTP child effects: %+v", recorder.effects)
	}
}

func TestReceiptDeadlineUnits(t *testing.T) {
	for _, deadline := range []time.Duration{0, time.Nanosecond, time.Millisecond, time.Millisecond + 1} {
		policy := DefaultExecutionPolicy()
		policy.Deadline = deadline
		r, _, _ := receiptRun(t, `(cli_app)`, policy)
		want := int64(0)
		if deadline > 0 {
			want = int64((deadline-1)/time.Millisecond) + 1
		}
		if r.Limits.DeadlineMS != want {
			t.Fatalf("deadline %s: %d want %d", deadline, r.Limits.DeadlineMS, want)
		}
	}
}
