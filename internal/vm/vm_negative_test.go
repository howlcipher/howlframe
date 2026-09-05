package vm

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/howlcipher/howlframe/internal/bytecode"
	"github.com/howlcipher/howlframe/internal/capability"
)

// runVMExpectingPanic constructs a BCVM for insts, applies setupEnv to its
// environment (nil is a no-op), and asserts that running it panics. If
// wantCode is non-empty, the panic must be a *VMError with that Code. Shared
// by every negative test in this file that drives the VM directly (rather
// than through RunBytecodeWithEvidence) to assert a specific failure mode.
func runVMExpectingPanic(t *testing.T, insts []bytecode.BCInstruction, setupEnv func(*BcEnv), wantCode string) {
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
		AllowedCaps: []capability.Capability{capability.Network},
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
		}
	}()
	vm.run(vm.insts, vm.env)
}

func TestVMNegativeStackUnderflowHandling(t *testing.T) {
	// Program with OpBinop but empty stack - must panic recoverably or error cleanly
	runVMExpectingPanic(t, []bytecode.BCInstruction{
		{Op: bytecode.OpBinop, StringOperand: "+"},
	}, nil, "")
}

func TestVMNegativeUndefinedVariable(t *testing.T) {
	runVMExpectingPanic(t, []bytecode.BCInstruction{
		{Op: bytecode.OpLoadVar, StringOperand: "non_existent_variable_xyz"},
	}, nil, "")
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
	t.Run("http_route with invalid mux in env", func(t *testing.T) {
		runVMExpectingPanic(t, []bytecode.BCInstruction{
			{Op: bytecode.OpHttpRoute, OpString: "HTTP_ROUTE", StringOperand: "/test", StringOperand2: "req", IntOperand: 0},
		}, func(env *BcEnv) {
			env.vars["__http_mux"] = "not_a_serve_mux"
		}, "TYPE_ERROR")
	})

	t.Run("http_server_serve with invalid port in env", func(t *testing.T) {
		runVMExpectingPanic(t, []bytecode.BCInstruction{
			{Op: bytecode.OpHttpServerServe, OpString: "HTTP_SERVER_SERVE"},
		}, func(env *BcEnv) {
			env.vars["__http_mux"] = http.NewServeMux()
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
