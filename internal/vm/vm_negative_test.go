package vm

import (
	"bytes"
	"database/sql"
	"database/sql/driver"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/howlcipher/howlframe/internal/bytecode"
	"github.com/howlcipher/howlframe/internal/capability"
	"github.com/howlcipher/howlframe/internal/httpreq"
)

// Deterministic in-process database/sql drivers used to exercise the VM's
// structured error paths without touching real databases or the network.
func init() {
	sql.Register("vmtest_open_fail", &openFailDriver{})
	sql.Register("vmtest_query_fail", &queryFailDriver{})
}

type openFailDriver struct{}

func (openFailDriver) Open(name string) (driver.Conn, error) {
	return nil, errors.New("open failed")
}

type queryFailDriver struct{}

func (queryFailDriver) Open(name string) (driver.Conn, error) {
	return &queryFailConn{}, nil
}

type queryFailConn struct{}

func (*queryFailConn) Prepare(query string) (driver.Stmt, error) {
	return nil, errors.New("query failed")
}

func (*queryFailConn) Close() error { return nil }
func (*queryFailConn) Begin() (driver.Tx, error) {
	return nil, errors.New("transactions not supported")
}

// errReadCloser is an io.ReadCloser that always returns a configured error.
// Used to exercise parse_json request-body read failures deterministically.
type errReadCloser struct {
	err error
}

func (e errReadCloser) Read(p []byte) (int, error) { return 0, e.err }
func (e errReadCloser) Close() error               { return nil }

// runVMExpectingPanicWithCaps constructs a BCVM for insts, applies setupEnv to its
// environment (nil is a no-op), and asserts that running it panics. If
// wantCode is non-empty, the panic must be a *VMError with that Code, and if
// wantMsgContains is non-empty the message must contain that substring.
// Shared by every negative test in this file that drives the VM directly
// (rather than through RunBytecodeWithEvidence) to assert a specific failure mode.
func runVMExpectingPanicWithCaps(t *testing.T, insts []bytecode.BCInstruction, caps []capability.Capability, setupEnv func(*BcEnv), wantCode string, wantMsgContains string) {
	t.Helper()
	env := NewBcEnv(nil)
	if setupEnv != nil {
		setupEnv(env)
	}
	vm := &BCVM{
		prog:        &bytecode.BCProgram{Main: insts},
		env:         env,
		insts:       insts,
		stores:      newBCStoreRegistry(),
		Limits:      DefaultLimits,
		AllowedCaps: caps,
	}
	defer func() {
		r := recover()
		if r == nil {
			t.Fatalf("expected VM to panic, but it completed silently")
		}
		if wantCode != "" {
			vmerr, ok := r.(*VMError)
			if !ok || vmerr.Code != wantCode {
				t.Fatalf("expected %s, got %v", wantCode, r)
			}
			if wantMsgContains != "" && !strings.Contains(vmerr.Message, wantMsgContains) {
				t.Fatalf("expected message to contain %q, got %q", wantMsgContains, vmerr.Message)
			}
		}
	}()
	vm.run(vm.insts, vm.env)
}

// runVMExpectingPanic is the capability-defaulting wrapper used by older
// negative tests; it grants network capability and does not assert a message
// substring.
func runVMExpectingPanic(t *testing.T, insts []bytecode.BCInstruction, setupEnv func(*BcEnv), wantCode string) {
	t.Helper()
	runVMExpectingPanicWithCaps(t, insts, []capability.Capability{capability.Network}, setupEnv, wantCode, "")
}

func TestVMNegativeStackUnderflowHandling(t *testing.T) {
	// Program with OpBinop but empty stack - must panic recoverably or error cleanly
	runVMExpectingPanic(t, []bytecode.BCInstruction{
		{Op: bytecode.OpBinop, StringOperand: "+"},
	}, nil, "")
}

func TestVMNegativeUndefinedVariable(t *testing.T) {
	runVMExpectingPanicWithCaps(t, []bytecode.BCInstruction{
		{Op: bytecode.OpLoadVar, StringOperand: "non_existent_variable_xyz"},
	}, nil, nil, "UNDEFINED_VAR", "undefined variable")
}

func TestVMNegativeSetUndefinedVariable(t *testing.T) {
	runVMExpectingPanicWithCaps(t, []bytecode.BCInstruction{
		{Op: bytecode.OpLoadConst, OpString: "LOAD_CONST", ValueOperand: float64(42)},
		{Op: bytecode.OpSetVar, StringOperand: "non_existent_variable_xyz"},
	}, nil, nil, "UNDEFINED_VAR", "undefined variable")
}

func TestVMNegativeMissingHTTPContext(t *testing.T) {
	t.Run("res requires response writer", func(t *testing.T) {
		runVMExpectingPanic(t, []bytecode.BCInstruction{
			{Op: bytecode.OpLoadConst, OpString: "LOAD_CONST", ValueOperand: float64(200)},
			{Op: bytecode.OpLoadConst, OpString: "LOAD_CONST", ValueOperand: "text/plain"},
			{Op: bytecode.OpLoadConst, OpString: "LOAD_CONST", ValueOperand: "body"},
			{Op: bytecode.OpRes, OpString: "RES"},
		}, nil, "RUNTIME_ERROR")
	})

	t.Run("res_json requires response writer", func(t *testing.T) {
		runVMExpectingPanic(t, []bytecode.BCInstruction{
			{Op: bytecode.OpLoadConst, OpString: "LOAD_CONST", ValueOperand: float64(200)},
			{Op: bytecode.OpLoadConst, OpString: "LOAD_CONST", ValueOperand: "data"},
			{Op: bytecode.OpResJson, OpString: "RES_JSON"},
		}, nil, "RUNTIME_ERROR")
	})

	t.Run("http_res_header requires response writer", func(t *testing.T) {
		runVMExpectingPanic(t, []bytecode.BCInstruction{
			{Op: bytecode.OpLoadConst, OpString: "LOAD_CONST", ValueOperand: "Header-Name"},
			{Op: bytecode.OpLoadConst, OpString: "LOAD_CONST", ValueOperand: "Header-Val"},
			{Op: bytecode.OpHttpResHeader, OpString: "HTTP_RES_HEADER"},
		}, nil, "RUNTIME_ERROR")
	})

	t.Run("http_req_method requires request context", func(t *testing.T) {
		runVMExpectingPanic(t, []bytecode.BCInstruction{
			{Op: bytecode.OpHttpReqMethod, OpString: "HTTP_REQ_METHOD"},
		}, nil, "RUNTIME_ERROR")
	})
}

func TestVMFileAndNetworkTypeAssertions(t *testing.T) {
	cases := []struct {
		name            string
		caps            []capability.Capability
		insts           []bytecode.BCInstruction
		wantMsgContains string
	}{
		{
			name: "read_file with number",
			caps: []capability.Capability{capability.Filesystem},
			insts: []bytecode.BCInstruction{
				{Op: bytecode.OpLoadConst, OpString: "LOAD_CONST", ValueOperand: float64(42)},
				{Op: bytecode.OpReadFile, OpString: "READ_FILE"},
			},
			wantMsgContains: "read_file expected string",
		},
		{
			name: "read_file with boolean",
			caps: []capability.Capability{capability.Filesystem},
			insts: []bytecode.BCInstruction{
				{Op: bytecode.OpLoadConst, OpString: "LOAD_CONST", ValueOperand: true},
				{Op: bytecode.OpReadFile, OpString: "READ_FILE"},
			},
			wantMsgContains: "read_file expected string",
		},
		{
			name: "write_file with number path",
			caps: []capability.Capability{capability.Filesystem},
			insts: []bytecode.BCInstruction{
				{Op: bytecode.OpLoadConst, OpString: "LOAD_CONST", ValueOperand: float64(42)},
				{Op: bytecode.OpLoadConst, OpString: "LOAD_CONST", ValueOperand: "content"},
				{Op: bytecode.OpWriteFile, OpString: "WRITE_FILE"},
			},
			wantMsgContains: "write_file expected string path",
		},
		{
			name: "write_file with nil path",
			caps: []capability.Capability{capability.Filesystem},
			insts: []bytecode.BCInstruction{
				{Op: bytecode.OpLoadConst, OpString: "LOAD_CONST", ValueOperand: nil},
				{Op: bytecode.OpLoadConst, OpString: "LOAD_CONST", ValueOperand: "content"},
				{Op: bytecode.OpWriteFile, OpString: "WRITE_FILE"},
			},
			wantMsgContains: "write_file expected string path",
		},
		{
			name: "write_file with number data",
			caps: []capability.Capability{capability.Filesystem},
			insts: []bytecode.BCInstruction{
				{Op: bytecode.OpLoadConst, OpString: "LOAD_CONST", ValueOperand: "output.txt"},
				{Op: bytecode.OpLoadConst, OpString: "LOAD_CONST", ValueOperand: float64(42)},
				{Op: bytecode.OpWriteFile, OpString: "WRITE_FILE"},
			},
			wantMsgContains: "write_file expected string or byte list data",
		},
		{
			name: "write_file with nil data",
			caps: []capability.Capability{capability.Filesystem},
			insts: []bytecode.BCInstruction{
				{Op: bytecode.OpLoadConst, OpString: "LOAD_CONST", ValueOperand: "output.txt"},
				{Op: bytecode.OpLoadConst, OpString: "LOAD_CONST", ValueOperand: nil},
				{Op: bytecode.OpWriteFile, OpString: "WRITE_FILE"},
			},
			wantMsgContains: "write_file expected string or byte list data",
		},
		{
			name: "write_file with boolean data",
			caps: []capability.Capability{capability.Filesystem},
			insts: []bytecode.BCInstruction{
				{Op: bytecode.OpLoadConst, OpString: "LOAD_CONST", ValueOperand: "output.txt"},
				{Op: bytecode.OpLoadConst, OpString: "LOAD_CONST", ValueOperand: true},
				{Op: bytecode.OpWriteFile, OpString: "WRITE_FILE"},
			},
			wantMsgContains: "write_file expected string or byte list data",
		},
		{
			name: "write_file with string in byte list",
			caps: []capability.Capability{capability.Filesystem},
			insts: []bytecode.BCInstruction{
				{Op: bytecode.OpLoadConst, OpString: "LOAD_CONST", ValueOperand: "output.txt"},
				{Op: bytecode.OpLoadConst, OpString: "LOAD_CONST", ValueOperand: []any{"invalid"}},
				{Op: bytecode.OpWriteFile, OpString: "WRITE_FILE"},
			},
			wantMsgContains: "write_file byte list element expected number",
		},
		{
			name: "mkdir with number path",
			caps: []capability.Capability{capability.Filesystem},
			insts: []bytecode.BCInstruction{
				{Op: bytecode.OpLoadConst, OpString: "LOAD_CONST", ValueOperand: float64(42)},
				{Op: bytecode.OpMkdir, OpString: "MKDIR"},
			},
			wantMsgContains: "mkdir expected string",
		},
		{
			name: "mkdir with slice path",
			caps: []capability.Capability{capability.Filesystem},
			insts: []bytecode.BCInstruction{
				{Op: bytecode.OpLoadConst, OpString: "LOAD_CONST", ValueOperand: []any{"nested"}},
				{Op: bytecode.OpMkdir, OpString: "MKDIR"},
			},
			wantMsgContains: "mkdir expected string",
		},
		{
			name: "fetch with number url",
			caps: []capability.Capability{capability.Network},
			insts: []bytecode.BCInstruction{
				{Op: bytecode.OpLoadConst, OpString: "LOAD_CONST", ValueOperand: float64(42)},
				{Op: bytecode.OpLoadConst, OpString: "LOAD_CONST", ValueOperand: "GET"},
				{Op: bytecode.OpFetch, OpString: "FETCH"},
			},
			wantMsgContains: "fetch expected string url",
		},
		{
			name: "fetch with number method",
			caps: []capability.Capability{capability.Network},
			insts: []bytecode.BCInstruction{
				{Op: bytecode.OpLoadConst, OpString: "LOAD_CONST", ValueOperand: "http://127.0.0.1:0"},
				{Op: bytecode.OpLoadConst, OpString: "LOAD_CONST", ValueOperand: float64(42)},
				{Op: bytecode.OpFetch, OpString: "FETCH"},
			},
			wantMsgContains: "fetch expected string method",
		},
		{
			name: "fetch with both number url and method",
			caps: []capability.Capability{capability.Network},
			insts: []bytecode.BCInstruction{
				{Op: bytecode.OpLoadConst, OpString: "LOAD_CONST", ValueOperand: float64(100)},
				{Op: bytecode.OpLoadConst, OpString: "LOAD_CONST", ValueOperand: float64(200)},
				{Op: bytecode.OpFetch, OpString: "FETCH"},
			},
			wantMsgContains: "fetch expected string method",
		},
		{
			name: "llm_generate with number prompt",
			caps: []capability.Capability{capability.Network},
			insts: []bytecode.BCInstruction{
				{Op: bytecode.OpLoadConst, OpString: "LOAD_CONST", ValueOperand: float64(42)},
				{Op: bytecode.OpLlmGenerate, OpString: "LLM_GENERATE", StringOperand: "llama3"},
			},
			wantMsgContains: "llm_generate expected string prompt",
		},
		{
			name: "res with number content type",
			caps: []capability.Capability{capability.Network},
			insts: []bytecode.BCInstruction{
				{Op: bytecode.OpLoadConst, OpString: "LOAD_CONST", ValueOperand: float64(200)},
				{Op: bytecode.OpLoadConst, OpString: "LOAD_CONST", ValueOperand: float64(42)},
				{Op: bytecode.OpLoadConst, OpString: "LOAD_CONST", ValueOperand: "hello"},
				{Op: bytecode.OpRes, OpString: "RES"},
			},
			wantMsgContains: "res expected string content type",
		},
		{
			name: "res with string status",
			caps: []capability.Capability{capability.Network},
			insts: []bytecode.BCInstruction{
				{Op: bytecode.OpLoadConst, OpString: "LOAD_CONST", ValueOperand: "200"},
				{Op: bytecode.OpLoadConst, OpString: "LOAD_CONST", ValueOperand: "text/plain"},
				{Op: bytecode.OpLoadConst, OpString: "LOAD_CONST", ValueOperand: "hello"},
				{Op: bytecode.OpRes, OpString: "RES"},
			},
			wantMsgContains: "res expected number status",
		},
		{
			name: "res_json with string status",
			caps: []capability.Capability{capability.Network},
			insts: []bytecode.BCInstruction{
				{Op: bytecode.OpLoadConst, OpString: "LOAD_CONST", ValueOperand: "200"},
				{Op: bytecode.OpLoadConst, OpString: "LOAD_CONST", ValueOperand: map[string]any{"ok": true}},
				{Op: bytecode.OpResJson, OpString: "RES_JSON"},
			},
			wantMsgContains: "res_json expected number status",
		},
		{
			name: "http_res_header with number name",
			caps: []capability.Capability{capability.Network},
			insts: []bytecode.BCInstruction{
				{Op: bytecode.OpLoadConst, OpString: "LOAD_CONST", ValueOperand: float64(42)},
				{Op: bytecode.OpLoadConst, OpString: "LOAD_CONST", ValueOperand: "application/json"},
				{Op: bytecode.OpHttpResHeader, OpString: "HTTP_RES_HEADER"},
			},
			wantMsgContains: "http_res_header expected string name",
		},
		{
			name: "http_res_header with number value",
			caps: []capability.Capability{capability.Network},
			insts: []bytecode.BCInstruction{
				{Op: bytecode.OpLoadConst, OpString: "LOAD_CONST", ValueOperand: "Content-Type"},
				{Op: bytecode.OpLoadConst, OpString: "LOAD_CONST", ValueOperand: float64(42)},
				{Op: bytecode.OpHttpResHeader, OpString: "HTTP_RES_HEADER"},
			},
			wantMsgContains: "http_res_header expected string value",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			prog := &bytecode.BCProgram{
				Main: tc.insts,
			}
			var outBuf, errBuf bytes.Buffer
			evidence := RunBytecodeWithEvidence(prog, nil, DefaultExecutionPolicy(), tc.caps, strings.NewReader(""), &outBuf, &errBuf, 0)
			if evidence.RuntimeFailure == nil {
				t.Fatalf("expected runtime failure, but succeeded with exit code %d", evidence.ExitCode)
			}
			if evidence.RuntimeFailure.Code != "TYPE_ERROR" {
				t.Fatalf("got failure code %q, want %q; message: %s", evidence.RuntimeFailure.Code, "TYPE_ERROR", evidence.RuntimeFailure.Message)
			}
			if tc.wantMsgContains != "" && !strings.Contains(evidence.RuntimeFailure.Message, tc.wantMsgContains) {
				t.Fatalf("expected message to contain %q, got %q", tc.wantMsgContains, evidence.RuntimeFailure.Message)
			}
		})
	}

	t.Run("capability check precedes type check", func(t *testing.T) {
		p := &bytecode.BCProgram{
			Main: []bytecode.BCInstruction{
				{Op: bytecode.OpReadFile, OpString: "READ_FILE"},
			},
		}
		res := RunBytecodeWithEvidence(p, nil, DefaultExecutionPolicy(), nil, nil, nil, nil, 0)
		if res.RuntimeFailure == nil || res.RuntimeFailure.Code != "CAPABILITY_DENIED" {
			t.Fatalf("expected CAPABILITY_DENIED, got %#v", res.RuntimeFailure)
		}
	})
}

// tryLetInstruction builds the OpTryLet header shared by every case in
// TestVMFileAndNetworkTypeAssertionTryLetCatchable: it always binds "res"/
// "err" over a 2-instruction catch body, varying only the guarded body's
// length (how many instructions it takes to push the operands and invoke the
// fallible opcode before the catchable failure point).
func tryLetInstruction(bodyLen int64) bytecode.BCInstruction {
	return bytecode.BCInstruction{
		Op:             bytecode.OpTryLet,
		StringOperand:  "res",
		StringOperand2: "err",
		IntOperand:     bodyLen,
		IntOperand2:    2,
		IntOperand3:    0,
	}
}

func TestVMFileAndNetworkTypeAssertionTryLetCatchable(t *testing.T) {
	cases := []struct {
		name  string
		caps  []capability.Capability
		insts []bytecode.BCInstruction
	}{
		{
			name: "read_file",
			caps: []capability.Capability{capability.Filesystem},
			insts: []bytecode.BCInstruction{
				tryLetInstruction(2),
				{Op: bytecode.OpLoadConst, OpString: "LOAD_CONST", ValueOperand: float64(123)},
				{Op: bytecode.OpReadFile, OpString: "READ_FILE"},
				{Op: bytecode.OpLoadVar, OpString: "LOAD_VAR", StringOperand: "err"},
				{Op: bytecode.OpPrint, OpString: "PRINT", IntOperand: 1},
			},
		},
		{
			name: "write_file",
			caps: []capability.Capability{capability.Filesystem},
			insts: []bytecode.BCInstruction{
				tryLetInstruction(3),
				{Op: bytecode.OpLoadConst, OpString: "LOAD_CONST", ValueOperand: "target.txt"},
				{Op: bytecode.OpLoadConst, OpString: "LOAD_CONST", ValueOperand: float64(42)},
				{Op: bytecode.OpWriteFile, OpString: "WRITE_FILE"},
				{Op: bytecode.OpLoadVar, OpString: "LOAD_VAR", StringOperand: "err"},
				{Op: bytecode.OpPrint, OpString: "PRINT", IntOperand: 1},
			},
		},
		{
			name: "mkdir",
			caps: []capability.Capability{capability.Filesystem},
			insts: []bytecode.BCInstruction{
				tryLetInstruction(2),
				{Op: bytecode.OpLoadConst, OpString: "LOAD_CONST", ValueOperand: float64(123)},
				{Op: bytecode.OpMkdir, OpString: "MKDIR"},
				{Op: bytecode.OpLoadVar, OpString: "LOAD_VAR", StringOperand: "err"},
				{Op: bytecode.OpPrint, OpString: "PRINT", IntOperand: 1},
			},
		},
		{
			name: "fetch",
			caps: []capability.Capability{capability.Network},
			insts: []bytecode.BCInstruction{
				tryLetInstruction(3),
				{Op: bytecode.OpLoadConst, OpString: "LOAD_CONST", ValueOperand: float64(123)},
				{Op: bytecode.OpLoadConst, OpString: "LOAD_CONST", ValueOperand: "GET"},
				{Op: bytecode.OpFetch, OpString: "FETCH"},
				{Op: bytecode.OpLoadVar, OpString: "LOAD_VAR", StringOperand: "err"},
				{Op: bytecode.OpPrint, OpString: "PRINT", IntOperand: 1},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			prog := &bytecode.BCProgram{
				Main: tc.insts,
			}
			var outBuf, errBuf bytes.Buffer
			evidence := RunBytecodeWithEvidence(prog, nil, DefaultExecutionPolicy(), tc.caps, strings.NewReader(""), &outBuf, &errBuf, 0)
			if evidence.RuntimeFailure != nil {
				t.Fatalf("expected try_let to catch error, but VM failed: %#v", evidence.RuntimeFailure)
			}
			if !strings.Contains(outBuf.String(), "TYPE_ERROR") {
				t.Fatalf("expected printed caught error to contain TYPE_ERROR, got: %q", outBuf.String())
			}
		})
	}
}

func TestVMFileAndNetworkPositiveFetch(t *testing.T) {
	var receivedMethod, receivedPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedMethod = r.Method
		receivedPath = r.URL.Path
		w.Write([]byte("ok"))
	}))
	defer srv.Close()

	prog := &bytecode.BCProgram{
		Main: []bytecode.BCInstruction{
			{Op: bytecode.OpLoadConst, OpString: "LOAD_CONST", ValueOperand: srv.URL + "/test-endpoint"},
			{Op: bytecode.OpLoadConst, OpString: "LOAD_CONST", ValueOperand: "POST"},
			{Op: bytecode.OpFetch, OpString: "FETCH"},
		},
	}
	var outBuf, errBuf bytes.Buffer
	evidence := RunBytecodeWithEvidence(prog, nil, DefaultExecutionPolicy(), []capability.Capability{capability.Network}, strings.NewReader(""), &outBuf, &errBuf, 0)
	if evidence.RuntimeFailure != nil {
		t.Fatalf("unexpected runtime failure: %#v", evidence.RuntimeFailure)
	}
	if receivedMethod != "POST" || receivedPath != "/test-endpoint" {
		t.Fatalf("expected POST /test-endpoint, got %s %s", receivedMethod, receivedPath)
	}
}

func TestVMFetchDeniedBeforeRequest(t *testing.T) {
	t.Setenv("NO_PROXY", "127.0.0.1,localhost")
	t.Setenv("no_proxy", "127.0.0.1,localhost")
	const secretBody = "phase2d-secret-bytes"
	var hits atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		_, _ = w.Write([]byte(secretBody))
	}))
	defer srv.Close()

	prog := &bytecode.BCProgram{
		Main: []bytecode.BCInstruction{
			{Op: bytecode.OpLoadConst, OpString: "LOAD_CONST", ValueOperand: srv.URL + "/phase2d-secret-url"},
			{Op: bytecode.OpLoadConst, OpString: "LOAD_CONST", ValueOperand: "GET"},
			{Op: bytecode.OpFetch, OpString: "FETCH"},
		},
	}
	run := func(caps []capability.Capability) (string, *bytecode.RuntimeFailure) {
		t.Helper()
		var outBuf, errBuf bytes.Buffer
		evidence := RunBytecodeWithEvidence(prog, nil, DefaultExecutionPolicy(), caps, strings.NewReader(""), &outBuf, &errBuf, 0)
		return outBuf.String() + errBuf.String(), evidence.RuntimeFailure
	}

	for _, tc := range []struct {
		name string
		caps []capability.Capability
	}{
		{"empty grant", nil},
		{"filesystem grant", []capability.Capability{capability.Filesystem}},
	} {
		text, failure := run(tc.caps)
		if failure == nil || failure.Code != "CAPABILITY_DENIED" {
			t.Fatalf("%s: expected CAPABILITY_DENIED, got %#v", tc.name, failure)
		}
		if hits.Load() != 0 {
			t.Fatalf("%s: denial performed %d HTTP request(s)", tc.name, hits.Load())
		}
		if strings.Contains(text, secretBody) || strings.Contains(text, "phase2d-secret-url") || strings.Contains(failure.Message, "phase2d-secret-url") {
			t.Fatalf("%s: denial leaked the response or URL: %s (%s)", tc.name, text, failure.Message)
		}
	}

	text, failure := run([]capability.Capability{capability.Network})
	if failure != nil {
		t.Fatalf("network grant failed: %#v\n%s", failure, text)
	}
	if hits.Load() != 1 {
		t.Fatalf("network grant performed %d HTTP request(s), want 1", hits.Load())
	}
}

func TestVMFileAndNetworkPositiveFileOps(t *testing.T) {
	tmpDir := t.TempDir()
	subDir := filepath.Join(tmpDir, "testsubdir")
	filePath1 := filepath.Join(subDir, "test1.txt")
	filePath2 := filepath.Join(subDir, "test2.txt")

	// 1. Mkdir
	progMkdir := &bytecode.BCProgram{
		Main: []bytecode.BCInstruction{
			{Op: bytecode.OpLoadConst, OpString: "LOAD_CONST", ValueOperand: subDir},
			{Op: bytecode.OpMkdir, OpString: "MKDIR"},
		},
	}
	ev := RunBytecodeWithEvidence(progMkdir, nil, DefaultExecutionPolicy(), []capability.Capability{capability.Filesystem}, strings.NewReader(""), nil, nil, 0)
	if ev.RuntimeFailure != nil {
		t.Fatalf("mkdir failed: %#v", ev.RuntimeFailure)
	}

	// 2. WriteFile string
	progWrite1 := &bytecode.BCProgram{
		Main: []bytecode.BCInstruction{
			{Op: bytecode.OpLoadConst, OpString: "LOAD_CONST", ValueOperand: filePath1},
			{Op: bytecode.OpLoadConst, OpString: "LOAD_CONST", ValueOperand: "hello world"},
			{Op: bytecode.OpWriteFile, OpString: "WRITE_FILE"},
		},
	}
	ev = RunBytecodeWithEvidence(progWrite1, nil, DefaultExecutionPolicy(), []capability.Capability{capability.Filesystem}, strings.NewReader(""), nil, nil, 0)
	if ev.RuntimeFailure != nil {
		t.Fatalf("write_file string failed: %#v", ev.RuntimeFailure)
	}

	// 3. ReadFile
	progRead := &bytecode.BCProgram{
		Main: []bytecode.BCInstruction{
			{Op: bytecode.OpLoadConst, OpString: "LOAD_CONST", ValueOperand: filePath1},
			{Op: bytecode.OpReadFile, OpString: "READ_FILE"},
		},
	}
	ev = RunBytecodeWithEvidence(progRead, nil, DefaultExecutionPolicy(), []capability.Capability{capability.Filesystem}, strings.NewReader(""), nil, nil, 0)
	if ev.RuntimeFailure != nil {
		t.Fatalf("read_file failed: %#v", ev.RuntimeFailure)
	}

	// 4. WriteFile byte list ([]any)
	progWrite2 := &bytecode.BCProgram{
		Main: []bytecode.BCInstruction{
			{Op: bytecode.OpLoadConst, OpString: "LOAD_CONST", ValueOperand: filePath2},
			{Op: bytecode.OpLoadConst, OpString: "LOAD_CONST", ValueOperand: []any{float64(65), int64(66), int(67)}},
			{Op: bytecode.OpWriteFile, OpString: "WRITE_FILE"},
		},
	}
	ev = RunBytecodeWithEvidence(progWrite2, nil, DefaultExecutionPolicy(), []capability.Capability{capability.Filesystem}, strings.NewReader(""), nil, nil, 0)
	if ev.RuntimeFailure != nil {
		t.Fatalf("write_file byte list failed: %#v", ev.RuntimeFailure)
	}
	data, err := os.ReadFile(filePath2)
	if err != nil || string(data) != "ABC" {
		t.Fatalf("expected ABC, got %q (err: %v)", string(data), err)
	}
}

func TestVMNetworkEnvironmentTypeAssertions(t *testing.T) {
	t.Run("http_route with invalid dispatcher in env", func(t *testing.T) {
		runVMExpectingPanic(t, []bytecode.BCInstruction{
			{Op: bytecode.OpHttpRoute, OpString: "HTTP_ROUTE", StringOperand: "/test", StringOperand2: "req", IntOperand: 0},
		}, func(env *BcEnv) {
			env.vars["__http_dispatch"] = "not_a_dispatcher"
		}, "TYPE_ERROR")
	})

	t.Run("http_server_serve with invalid port in env", func(t *testing.T) {
		runVMExpectingPanic(t, []bytecode.BCInstruction{
			{Op: bytecode.OpHttpServerServe, OpString: "HTTP_SERVER_SERVE"},
		}, func(env *BcEnv) {
			mux := http.NewServeMux()
			env.vars["__http_mux"] = mux
			env.vars["__http_dispatch"] = httpreq.NewServer(mux)
			env.vars["__http_port"] = float64(8080) // not a string
		}, "TYPE_ERROR")
	})

	t.Run("http_req_method with invalid req in env", func(t *testing.T) {
		runVMExpectingPanic(t, []bytecode.BCInstruction{
			{Op: bytecode.OpHttpReqMethod, OpString: "HTTP_REQ_METHOD"},
		}, func(env *BcEnv) {
			env.vars["req"] = "not_a_request"
		}, "TYPE_ERROR")
	})

	t.Run("http_res_header with invalid w in env", func(t *testing.T) {
		runVMExpectingPanic(t, []bytecode.BCInstruction{
			{Op: bytecode.OpLoadConst, OpString: "LOAD_CONST", ValueOperand: "Header-Name"},
			{Op: bytecode.OpLoadConst, OpString: "LOAD_CONST", ValueOperand: "Header-Val"},
			{Op: bytecode.OpHttpResHeader, OpString: "HTTP_RES_HEADER"},
		}, func(env *BcEnv) {
			env.vars["w"] = "not_a_response_writer"
		}, "TYPE_ERROR")
	})

	t.Run("res with invalid w in env", func(t *testing.T) {
		runVMExpectingPanic(t, []bytecode.BCInstruction{
			{Op: bytecode.OpLoadConst, OpString: "LOAD_CONST", ValueOperand: float64(200)},
			{Op: bytecode.OpLoadConst, OpString: "LOAD_CONST", ValueOperand: "text/plain"},
			{Op: bytecode.OpLoadConst, OpString: "LOAD_CONST", ValueOperand: "body"},
			{Op: bytecode.OpRes, OpString: "RES"},
		}, func(env *BcEnv) {
			env.vars["w"] = 12345
		}, "TYPE_ERROR")
	})
}

func TestVMTaskAndSpawnAgent(t *testing.T) {
	task := bytecode.BCInstruction{Op: bytecode.OpTask, OpString: "TASK", StringOperand: "work"}
	spawn := bytecode.BCInstruction{Op: bytecode.OpSpawnAgent, OpString: "SPAWN_AGENT", StringOperand: "Worker"}
	spawnBody := spawn
	spawnBody.IntOperand = 2
	body := []bytecode.BCInstruction{task, spawnBody, {Op: bytecode.OpLoadConst, ValueOperand: "worker-body"}, {Op: bytecode.OpPrint, IntOperand: 1}}
	number := bytecode.BCInstruction{Op: bytecode.OpLoadConst, OpString: "LOAD_CONST", ValueOperand: float64(123)}
	for _, tc := range []struct {
		name         string
		insts        []bytecode.BCInstruction
		caps         []capability.Capability
		output, code string
	}{
		{"task pushes description without capabilities", []bytecode.BCInstruction{task, {Op: bytecode.OpPrint, IntOperand: 1}}, nil, "work\n", ""},
		{"spawn completes synchronously", []bytecode.BCInstruction{task, spawn}, []capability.Capability{capability.Process}, "[Swarm VM] Spawning agent \"Worker\" for task: \"work\"\n[Swarm VM] Agent \"Worker\" completed task: \"work\"\n", ""},
		{"spawn executes body synchronously", body, []capability.Capability{capability.Process}, "[Swarm VM] Spawning agent \"Worker\" for task: \"work\"\nworker-body\n[Swarm VM] Agent \"Worker\" completed task: \"work\"\n", ""},
		{"capability check precedes stack pop", []bytecode.BCInstruction{spawn}, nil, "", "CAPABILITY_DENIED"},
		{"capability check precedes type check", []bytecode.BCInstruction{number, spawn}, nil, "", "CAPABILITY_DENIED"},
		{"non-string task", []bytecode.BCInstruction{number, spawn}, []capability.Capability{capability.Process}, "", "TYPE_ERROR"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var out bytes.Buffer
			ev := RunBytecodeWithEvidence(&bytecode.BCProgram{Main: tc.insts}, nil, DefaultExecutionPolicy(), tc.caps, nil, &out, nil, 0)
			if tc.code == "" {
				if ev.RuntimeFailure != nil {
					t.Fatalf("unexpected failure: %#v", ev.RuntimeFailure)
				}
			} else if ev.RuntimeFailure == nil || ev.RuntimeFailure.Code != tc.code || ev.RuntimeFailure.Opcode != "SPAWN_AGENT" || ev.RuntimeFailure.Instruction != len(tc.insts)-1 {
				t.Fatalf("expected %s at SPAWN_AGENT, got %#v", tc.code, ev.RuntimeFailure)
			}
			if tc.code == "TYPE_ERROR" && ev.RuntimeFailure.Message != "spawn_agent expected string task, got float64" {
				t.Fatalf("unexpected type error: %#v", ev.RuntimeFailure)
			}
			if out.String() != tc.output {
				t.Fatalf("stdout %q, want %q", out.String(), tc.output)
			}
		})
	}
	t.Run("body captures environment", func(t *testing.T) {
		prog := &bytecode.BCProgram{Main: []bytecode.BCInstruction{
			{Op: bytecode.OpLoadConst, ValueOperand: "captured"},
			{Op: bytecode.OpStoreVar, StringOperand: "message"},
			task, spawnBody,
			{Op: bytecode.OpLoadVar, StringOperand: "message"},
			{Op: bytecode.OpPrint, IntOperand: 1},
		}}
		var out bytes.Buffer
		ev := RunBytecodeWithEvidence(prog, nil, DefaultExecutionPolicy(), []capability.Capability{capability.Process}, nil, &out, nil, 0)
		want := "[Swarm VM] Spawning agent \"Worker\" for task: \"work\"\ncaptured\n[Swarm VM] Agent \"Worker\" completed task: \"work\"\n"
		if ev.RuntimeFailure != nil || out.String() != want {
			t.Fatalf("failure %#v, stdout %q; want %q", ev.RuntimeFailure, out.String(), want)
		}
	})
	t.Run("body capability failure is isolated", func(t *testing.T) {
		insts := append(append([]bytecode.BCInstruction{}, body[:2]...), bytecode.BCInstruction{Op: bytecode.OpLoadConst, ValueOperand: "PATH"}, bytecode.BCInstruction{Op: bytecode.OpEnv})
		insts = append(insts, spawnTestPrint("parent after")...)
		var out, err bytes.Buffer
		ev := RunBytecodeWithEvidence(&bytecode.BCProgram{Main: insts}, nil, DefaultExecutionPolicy(), []capability.Capability{capability.Process}, nil, &out, &err, 0)
		if ev.RuntimeFailure != nil || out.String() != spawnTestStart("Worker")+"parent after\n" || err.String() != "[Swarm VM] Agent \"Worker\" failed task: \"work\": CAPABILITY_DENIED: capability denied: environment\n" {
			t.Fatalf("failure %#v stdout %q stderr %q", ev.RuntimeFailure, out.String(), err.String())
		}
	})
	t.Run("try_let catches type error", func(t *testing.T) {
		prog := &bytecode.BCProgram{Main: []bytecode.BCInstruction{
			tryLetInstruction(2), number, spawn,
			{Op: bytecode.OpLoadVar, OpString: "LOAD_VAR", StringOperand: "err"},
			{Op: bytecode.OpPrint, OpString: "PRINT", IntOperand: 1},
		}}
		var out bytes.Buffer
		ev := RunBytecodeWithEvidence(prog, nil, DefaultExecutionPolicy(), []capability.Capability{capability.Process}, nil, &out, nil, 0)
		if ev.RuntimeFailure != nil {
			t.Fatalf("try_let failed: %#v", ev.RuntimeFailure)
		}
		if !strings.Contains(out.String(), "TYPE_ERROR") || !strings.Contains(out.String(), "spawn_agent expected string task, got float64") {
			t.Fatalf("unexpected caught error: %q", out.String())
		}
	})
}

func TestVMTypeAndRuntimeErrors(t *testing.T) {
	cases := []struct {
		name            string
		caps            []capability.Capability
		setupEnv        func(*testing.T, *BcEnv)
		insts           []bytecode.BCInstruction
		wantCode        string
		wantMsgContains string
	}{
		{
			name: "OpEnv with non-string operand",
			caps: []capability.Capability{capability.Environment},
			insts: []bytecode.BCInstruction{
				{Op: bytecode.OpLoadConst, OpString: "LOAD_CONST", ValueOperand: float64(123)},
				{Op: bytecode.OpEnv, OpString: "ENV"},
			},
			wantCode:        "TYPE_ERROR",
			wantMsgContains: "env expected string name",
		},
		{
			name: "OpSleep with non-number operand",
			caps: nil,
			insts: []bytecode.BCInstruction{
				{Op: bytecode.OpLoadConst, OpString: "LOAD_CONST", ValueOperand: "not a number"},
				{Op: bytecode.OpSleep, OpString: "SLEEP"},
			},
			wantCode:        "TYPE_ERROR",
			wantMsgContains: "sleep requires number",
		},
		{
			name: "OpCliArgsGet with non-number index",
			caps: nil,
			insts: []bytecode.BCInstruction{
				{Op: bytecode.OpLoadConst, OpString: "LOAD_CONST", ValueOperand: "index"},
				{Op: bytecode.OpCliArgsGet, OpString: "CLI_ARGS_GET"},
			},
			wantCode:        "TYPE_ERROR",
			wantMsgContains: "cli_args index must be a number",
		},
		{
			name: "OpForInit with non-list operand",
			caps: nil,
			insts: []bytecode.BCInstruction{
				{Op: bytecode.OpLoadConst, OpString: "LOAD_CONST", ValueOperand: "not a list"},
				{Op: bytecode.OpForInit, OpString: "FOR_INIT"},
			},
			wantCode:        "TYPE_ERROR",
			wantMsgContains: "for requires a list",
		},
		{
			name: "OpForNext with wrong index type",
			caps: nil,
			insts: []bytecode.BCInstruction{
				{Op: bytecode.OpLoadConst, OpString: "LOAD_CONST", ValueOperand: []any{"a", "b"}},
				{Op: bytecode.OpLoadConst, OpString: "LOAD_CONST", ValueOperand: "not a number"},
				{Op: bytecode.OpForNext, OpString: "FOR_NEXT", StringOperand: "x", IntOperand: 0},
			},
			wantCode:        "TYPE_ERROR",
			wantMsgContains: "for index must be a number",
		},
		{
			name: "OpForNext with wrong items type",
			caps: nil,
			insts: []bytecode.BCInstruction{
				{Op: bytecode.OpLoadConst, OpString: "LOAD_CONST", ValueOperand: "not a list"},
				{Op: bytecode.OpLoadConst, OpString: "LOAD_CONST", ValueOperand: float64(0)},
				{Op: bytecode.OpForNext, OpString: "FOR_NEXT", StringOperand: "x", IntOperand: 0},
			},
			wantCode:        "TYPE_ERROR",
			wantMsgContains: "for requires a list",
		},
		{
			name: "OpCall to undefined function",
			caps: nil,
			insts: []bytecode.BCInstruction{
				{Op: bytecode.OpCall, OpString: "CALL", StringOperand: "undefined_function_xyz", IntOperand: 0},
			},
			wantCode:        "RUNTIME_ERROR",
			wantMsgContains: "undefined function",
		},
		{
			name: "OpDbConnect open failure",
			caps: []capability.Capability{capability.Database},
			insts: []bytecode.BCInstruction{
				{Op: bytecode.OpDbConnect, OpString: "DB_CONNECT", StringOperand: "db", StringOperand2: "", StringOperand3: ""},
			},
			wantCode:        "IO_ERROR",
			wantMsgContains: "db_connect failed",
		},
		{
			name: "OpSqlQuery undefined db var",
			caps: []capability.Capability{capability.Database},
			insts: []bytecode.BCInstruction{
				{Op: bytecode.OpSqlQuery, OpString: "SQL_QUERY", StringOperand: "missing_db", StringOperand2: "SELECT 1"},
			},
			wantCode:        "UNDEFINED_VAR",
			wantMsgContains: "undefined db",
		},
		{
			name: "OpSqlQuery non-db value",
			caps: []capability.Capability{capability.Database},
			setupEnv: func(t *testing.T, env *BcEnv) {
				env.vars["db"] = "not a database"
			},
			insts: []bytecode.BCInstruction{
				{Op: bytecode.OpSqlQuery, OpString: "SQL_QUERY", StringOperand: "db", StringOperand2: "SELECT 1"},
			},
			wantCode:        "TYPE_ERROR",
			wantMsgContains: "expected *sql.DB",
		},
		{
			name: "OpSqlQuery execution failure",
			caps: []capability.Capability{capability.Database},
			insts: []bytecode.BCInstruction{
				{Op: bytecode.OpDbConnect, OpString: "DB_CONNECT", StringOperand: "db", StringOperand2: "vmtest_query_fail", StringOperand3: ""},
				{Op: bytecode.OpSqlQuery, OpString: "SQL_QUERY", StringOperand: "db", StringOperand2: "SELECT 1"},
			},
			wantCode:        "IO_ERROR",
			wantMsgContains: "sql_query failed",
		},
		{
			name: "parse_json nil request body",
			caps: nil,
			setupEnv: func(t *testing.T, env *BcEnv) {
				env.vars["req"] = &http.Request{Body: nil}
			},
			insts: []bytecode.BCInstruction{
				{Op: bytecode.OpParseJson, OpString: "PARSE_JSON", StringOperand: "req.body"},
			},
			wantCode:        "RUNTIME_ERROR",
			wantMsgContains: "request body is nil",
		},
		{
			name: "parse_json request body read error",
			caps: nil,
			setupEnv: func(t *testing.T, env *BcEnv) {
				env.vars["req"] = &http.Request{Body: errReadCloser{err: errors.New("body read boom")}}
			},
			insts: []bytecode.BCInstruction{
				{Op: bytecode.OpParseJson, OpString: "PARSE_JSON", StringOperand: "req.body"},
			},
			wantCode:        "IO_ERROR",
			wantMsgContains: "failed to read request body",
		},
		{
			name: "parse_json request body exceeds 10MB",
			caps: nil,
			setupEnv: func(t *testing.T, env *BcEnv) {
				const maxBodyBytes = 10 * 1024 * 1024
				env.vars["req"] = &http.Request{Body: io.NopCloser(bytes.NewReader(make([]byte, maxBodyBytes+1)))}
			},
			insts: []bytecode.BCInstruction{
				{Op: bytecode.OpParseJson, OpString: "PARSE_JSON", StringOperand: "req.body"},
			},
			wantCode:        "LIMIT_EXCEEDED",
			wantMsgContains: "request body exceeds maximum limit",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var setup func(*BcEnv)
			if tc.setupEnv != nil {
				setup = func(env *BcEnv) { tc.setupEnv(t, env) }
			}
			runVMExpectingPanicWithCaps(t, tc.insts, tc.caps, setup, tc.wantCode, tc.wantMsgContains)
		})
	}
}

func TestVMOpSleepNumericTypes(t *testing.T) {
	cases := []struct {
		name string
		val  any
	}{
		{"int64", int64(1)},
		{"int", int(1)},
		{"float64", float64(1)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			prog := &bytecode.BCProgram{
				Main: []bytecode.BCInstruction{
					{Op: bytecode.OpLoadConst, OpString: "LOAD_CONST", ValueOperand: tc.val},
					{Op: bytecode.OpSleep, OpString: "SLEEP"},
				},
			}
			ev := RunBytecodeWithEvidence(prog, nil, DefaultExecutionPolicy(), nil, nil, nil, nil, 0)
			if ev.RuntimeFailure != nil {
				t.Fatalf("expected success, got runtime failure: %#v", ev.RuntimeFailure)
			}
		})
	}
}

func TestVMOpSleepNonNumeric(t *testing.T) {
	runVMExpectingPanicWithCaps(t, []bytecode.BCInstruction{
		{Op: bytecode.OpLoadConst, OpString: "LOAD_CONST", ValueOperand: []any{"not a number"}},
		{Op: bytecode.OpSleep, OpString: "SLEEP"},
	}, nil, nil, "TYPE_ERROR", "sleep requires number")
}

func TestVMOpForNextBounds(t *testing.T) {
	cases := []struct {
		name string
		idx  any
	}{
		{"negative", float64(-1)},
		{"past end", float64(3)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			runVMExpectingPanicWithCaps(t, []bytecode.BCInstruction{
				{Op: bytecode.OpLoadConst, OpString: "LOAD_CONST", ValueOperand: []any{"a", "b"}},
				{Op: bytecode.OpLoadConst, OpString: "LOAD_CONST", ValueOperand: tc.idx},
				{Op: bytecode.OpForNext, OpString: "FOR_NEXT", StringOperand: "x", IntOperand: 0},
			}, nil, nil, "RUNTIME_ERROR", "for index")
		})
	}
}

func TestVMOpCallArityMismatch(t *testing.T) {
	cases := []struct {
		name     string
		pushArgs []any
		numArgs  int64
	}{
		{"too few", nil, 0},
		{"too many", []any{"arg1", "arg2"}, 2},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var main []bytecode.BCInstruction
			for _, arg := range tc.pushArgs {
				main = append(main, bytecode.BCInstruction{Op: bytecode.OpLoadConst, OpString: "LOAD_CONST", ValueOperand: arg})
			}
			main = append(main, bytecode.BCInstruction{
				Op:            bytecode.OpCall,
				OpString:      "CALL",
				StringOperand: "f",
				IntOperand:    tc.numArgs,
			})
			prog := &bytecode.BCProgram{
				Functions: map[string]*bytecode.BCFunction{
					"f": {
						Params: []string{"a"},
						Instructions: []bytecode.BCInstruction{
							{Op: bytecode.OpLoadVar, OpString: "LOAD_VAR", StringOperand: "a"},
						},
					},
				},
				Main: main,
			}
			ev := RunBytecodeWithEvidence(prog, nil, DefaultExecutionPolicy(), nil, nil, nil, nil, 0)
			if ev.RuntimeFailure == nil || ev.RuntimeFailure.Code != "RUNTIME_ERROR" {
				t.Fatalf("expected RUNTIME_ERROR, got %#v", ev.RuntimeFailure)
			}
			if !strings.Contains(ev.RuntimeFailure.Message, "expects 1 argument") {
				t.Fatalf("expected arity error message, got %q", ev.RuntimeFailure.Message)
			}
		})
	}
}

func TestVMOpCallLazySynthesisCapability(t *testing.T) {
	prog := &bytecode.BCProgram{
		Functions: map[string]*bytecode.BCFunction{
			"synth": {
				Name:           "synth",
				Params:         []string{"a"},
				LazySynthesize: true,
				Docstring:      "return a",
			},
		},
		Main: []bytecode.BCInstruction{
			{Op: bytecode.OpLoadConst, OpString: "LOAD_CONST", ValueOperand: "hello"},
			{Op: bytecode.OpCall, OpString: "CALL", StringOperand: "synth", IntOperand: 1},
		},
	}
	ev := RunBytecodeWithEvidence(prog, nil, DefaultExecutionPolicy(), nil, nil, nil, nil, 0)
	if ev.RuntimeFailure == nil || ev.RuntimeFailure.Code != "CAPABILITY_DENIED" {
		t.Fatalf("expected CAPABILITY_DENIED, got %#v", ev.RuntimeFailure)
	}
}

func TestVMBytesToStringTypeErrors(t *testing.T) {
	cases := []struct {
		name    string
		val     any
		wantMsg string
	}{
		{"non-list", "not a list", "bytes_to_string expected []any"},
		{"list with string element", []any{"not a number"}, "bytes_to_string byte list element expected number"},
		{"list with bool element", []any{true}, "bytes_to_string byte list element expected number"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			runVMExpectingPanicWithCaps(t, []bytecode.BCInstruction{
				{Op: bytecode.OpLoadConst, OpString: "LOAD_CONST", ValueOperand: tc.val},
				{Op: bytecode.OpConvert, OpString: "CONVERT", StringOperand: "bytes_to_string"},
			}, nil, nil, "TYPE_ERROR", tc.wantMsg)
		})
	}
}

func TestVMNegativeHelperStructuredErrors(t *testing.T) {
	t.Run("BcToBool non-boolean panics TYPE_ERROR", func(t *testing.T) {
		// OpJumpIfFalse calls BcToBool, which must produce a structured TYPE_ERROR
		runVMExpectingPanicWithCaps(t, []bytecode.BCInstruction{
			{Op: bytecode.OpLoadConst, OpString: "LOAD_CONST", ValueOperand: "not a bool"},
			{Op: bytecode.OpJumpIfFalse, OpString: "JUMP_IF_FALSE", IntOperand: 1},
		}, nil, nil, "TYPE_ERROR", "expected boolean")
	})

	t.Run("division by zero panics RUNTIME_ERROR", func(t *testing.T) {
		runVMExpectingPanicWithCaps(t, []bytecode.BCInstruction{
			{Op: bytecode.OpLoadConst, OpString: "LOAD_CONST", ValueOperand: float64(1)},
			{Op: bytecode.OpLoadConst, OpString: "LOAD_CONST", ValueOperand: float64(0)},
			{Op: bytecode.OpBinop, OpString: "BINOP", StringOperand: "/"},
		}, nil, nil, "RUNTIME_ERROR", "division by zero")
	})

	t.Run("non-numeric binop operands panic TYPE_ERROR", func(t *testing.T) {
		runVMExpectingPanicWithCaps(t, []bytecode.BCInstruction{
			{Op: bytecode.OpLoadConst, OpString: "LOAD_CONST", ValueOperand: "not a number"},
			{Op: bytecode.OpLoadConst, OpString: "LOAD_CONST", ValueOperand: float64(1)},
			{Op: bytecode.OpBinop, OpString: "BINOP", StringOperand: "+"},
		}, nil, nil, "TYPE_ERROR", "requires numeric operands")
	})

	t.Run("parse_json invalid JSON panics RUNTIME_ERROR", func(t *testing.T) {
		runVMExpectingPanicWithCaps(t, []bytecode.BCInstruction{
			{Op: bytecode.OpParseJson, OpString: "PARSE_JSON", StringOperand: "data"},
		}, nil, func(env *BcEnv) {
			env.vars["data"] = "not valid json {"
		}, "RUNTIME_ERROR", "parse_json failed")
	})
}

func TestVMNegativeRawPanicsAreStructured(t *testing.T) {
	// Verify that all converted panic sites produce structured RuntimeFailure
	// through RunBytecodeWithEvidence (not VM_INTERNAL).
	cases := []struct {
		name     string
		insts    []bytecode.BCInstruction
		caps     []capability.Capability
		setupEnv func(*BcEnv)
		wantCode string
	}{
		{
			name: "OpLoadVar undefined yields UNDEFINED_VAR",
			insts: []bytecode.BCInstruction{
				{Op: bytecode.OpLoadVar, OpString: "LOAD_VAR", StringOperand: "missing_var"},
			},
			wantCode: "UNDEFINED_VAR",
		},
		{
			name: "OpSetVar undefined yields UNDEFINED_VAR",
			insts: []bytecode.BCInstruction{
				{Op: bytecode.OpLoadConst, OpString: "LOAD_CONST", ValueOperand: float64(1)},
				{Op: bytecode.OpSetVar, OpString: "SET_VAR", StringOperand: "missing_var"},
			},
			wantCode: "UNDEFINED_VAR",
		},
		{
			name: "OpRes no writer yields RUNTIME_ERROR",
			insts: []bytecode.BCInstruction{
				{Op: bytecode.OpLoadConst, OpString: "LOAD_CONST", ValueOperand: float64(200)},
				{Op: bytecode.OpLoadConst, OpString: "LOAD_CONST", ValueOperand: "text/html"},
				{Op: bytecode.OpLoadConst, OpString: "LOAD_CONST", ValueOperand: "hello"},
				{Op: bytecode.OpRes, OpString: "RES"},
			},
			caps:     []capability.Capability{capability.Network},
			wantCode: "RUNTIME_ERROR",
		},
		{
			name: "OpHttpReqMethod no req yields RUNTIME_ERROR",
			insts: []bytecode.BCInstruction{
				{Op: bytecode.OpHttpReqMethod, OpString: "HTTP_REQ_METHOD"},
			},
			caps:     []capability.Capability{capability.Network},
			wantCode: "RUNTIME_ERROR",
		},
		{
			name: "division by zero yields RUNTIME_ERROR",
			insts: []bytecode.BCInstruction{
				{Op: bytecode.OpLoadConst, OpString: "LOAD_CONST", ValueOperand: float64(10)},
				{Op: bytecode.OpLoadConst, OpString: "LOAD_CONST", ValueOperand: float64(0)},
				{Op: bytecode.OpBinop, OpString: "BINOP", StringOperand: "/"},
			},
			wantCode: "RUNTIME_ERROR",
		},
		{
			name: "BcToBool non-bool yields TYPE_ERROR",
			insts: []bytecode.BCInstruction{
				{Op: bytecode.OpLoadConst, OpString: "LOAD_CONST", ValueOperand: float64(42)},
				{Op: bytecode.OpJumpIfFalse, OpString: "JUMP_IF_FALSE", IntOperand: 1},
			},
			wantCode: "TYPE_ERROR",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			prog := &bytecode.BCProgram{Main: tc.insts}
			caps := tc.caps
			ev := RunBytecodeWithEvidence(prog, nil, DefaultExecutionPolicy(), caps, nil, &strings.Builder{}, &strings.Builder{}, 0)
			if ev.RuntimeFailure == nil {
				t.Fatalf("expected RuntimeFailure, got none")
			}
			if ev.RuntimeFailure.Code != tc.wantCode {
				t.Fatalf("expected code %s, got %s (message: %s)", tc.wantCode, ev.RuntimeFailure.Code, ev.RuntimeFailure.Message)
			}
			if ev.RuntimeFailure.Code == "VM_INTERNAL" {
				t.Fatalf("panic fell through to VM_INTERNAL: %s", ev.RuntimeFailure.Message)
			}
		})
	}
}

// TestInterpExitToIntSafety verifies that the ToInt conversion used by the
// interpreter's (exit) handler safely rejects non-numeric types instead of
// relying on a raw Go type assertion that would panic on type mismatch.
// This is a regression guard for the fix that replaced val.(int64) with ToInt.
func TestInterpExitToIntSafety(t *testing.T) {
	cases := []struct {
		name    string
		val     any
		wantErr bool
		wantVal int64
	}{
		{"int64", int64(42), false, 42},
		{"float64", float64(3.0), false, 3},
		{"int", int(7), false, 7},
		{"string_numeric", "10", false, 10},
		{"string_non_numeric", "not_a_number", true, 0},
		{"bool", true, true, 0},
		{"nil", nil, true, 0},
		{"list", []any{"a"}, true, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			val, err := ToInt(tc.val)
			if tc.wantErr && err == nil {
				t.Fatalf("expected error for %T, got nil", tc.val)
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("unexpected error for %T: %v", tc.val, err)
			}
			if !tc.wantErr && val != tc.wantVal {
				t.Fatalf("expected %d, got %d", tc.wantVal, val)
			}
		})
	}
}
