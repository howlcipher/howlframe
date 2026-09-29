package vm

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/howlcipher/howlframe/internal/bytecode"
	"github.com/howlcipher/howlframe/internal/capability"
	"github.com/howlcipher/howlframe/internal/httpreq"
	"github.com/howlcipher/howlframe/internal/lexer"
	"github.com/howlcipher/howlframe/internal/parser"
)

func TestRequestReadsAgreeAndGrantNothing(t *testing.T) {
	for _, op := range []bytecode.Opcode{bytecode.OpHttpReqQuery, bytecode.OpHttpReqHeader, bytecode.OpHttpReqPath} {
		spec := bytecode.Registry[op]
		if spec.Capability != capability.None {
			t.Fatalf("%s capability = %q, want none", spec.Name, spec.Capability)
		}
		if got := capability.ForConstruct(strings.ToLower(spec.Name)); got != capability.None {
			t.Fatalf("ForConstruct(%s) = %q, want none", spec.Name, got)
		}
	}
	for _, name := range []string{"req_query", "req_header", "req_path"} {
		if got := capability.ForConstruct(name); got != capability.None {
			t.Fatalf("ForConstruct(%s) = %q, want none", name, got)
		}
	}

	req := httptest.NewRequest(http.MethodGet, "/tasks/9?status=open&status=closed&empty=", nil)
	req.Header.Add("Authorization", "Bearer z")
	req.Header.Add("Authorization", "Bearer later")
	req = httpreq.WithParams(req, map[string]string{"id": "9"})

	cases := []struct {
		expr string
		want string
	}{
		{`(req_query req "status")`, "open"},
		{`(req_query req "missing")`, ""},
		{`(req_query req "empty")`, ""},
		{`(req_query req "Status")`, ""},
		{`(req_header req "authorization")`, "Bearer z"},
		{`(req_header req "X-Missing")`, ""},
		{`(req_path req "id")`, "9"},
		{`(req_path req "nope")`, ""},
	}
	for _, tc := range cases {
		t.Run(tc.expr, func(t *testing.T) {
			if got := evalRequestInterp(t, tc.expr, req); got != tc.want {
				t.Fatalf("interpreter = %q, want %q", got, tc.want)
			}
			// Nil grants: the read must not require network, database, or filesystem.
			if got := evalRequestVM(t, tc.expr, req, nil); got != tc.want {
				t.Fatalf("vm = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestRequestReadsFailClosed(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	bad := []string{
		`(req_query "nope" "status")`,
		`(req_header 1 "Authorization")`,
		`(req_path req 1)`,
		`(req_query req "")`,
		`(req_header req "")`,
		`(req_path req "")`,
	}
	for _, expr := range bad {
		t.Run(expr, func(t *testing.T) {
			interpReason := evalRequestInterpError(t, expr, req)
			if !strings.Contains(interpReason, "TYPE_ERROR") {
				t.Fatalf("interpreter error = %q, want TYPE_ERROR", interpReason)
			}
			vmErr := evalRequestVMError(t, expr, req)
			if vmErr == nil || vmErr.Code != "TYPE_ERROR" {
				t.Fatalf("vm error = %#v, want TYPE_ERROR", vmErr)
			}
		})
	}
}

func TestHTTPRouteReadsQueryHeaderAndPath(t *testing.T) {
	const src = `(http_server 0
  (route "/health" (lambda (req)
    (res 200 "text/plain" (req_query req "ready"))))
  (route "/tasks/new" (lambda (req)
    (res 200 "text/plain" "literal")))
  (route "/tasks/{id}" (lambda (req)
    (res 200 "text/plain" (str_join (list (req_path req "id") (req_query req "status") (req_query req "missing") (req_header req "authorization") (req_header req "X-Missing")) "|"))))
  (route "/tasks/{id}/notes/{note}" (lambda (req)
    (res 200 "text/plain" (str_join (list (req_path req "id") (req_path req "note")) "/")))))`

	_, prog := parseAndCompile(t, src)
	sawRead := false
	for _, inst := range prog.Main {
		switch inst.Op {
		case bytecode.OpHttpReqQuery, bytecode.OpHttpReqHeader, bytecode.OpHttpReqPath:
			sawRead = true
			if bytecode.Registry[inst.Op].Capability != capability.None {
				t.Fatalf("%s grew a capability", bytecode.Registry[inst.Op].Name)
			}
		}
	}
	if !sawRead {
		t.Fatal("compiled program has no request-read opcode")
	}

	env := NewBcEnv(nil)
	var errOut bytes.Buffer
	machine := &BCVM{
		prog:        prog,
		env:         env,
		insts:       prog.Main,
		stores:      newBCStoreRegistry(),
		Limits:      DefaultLimits,
		AllowedCaps: []capability.Capability{capability.Network},
		Out:         &bytes.Buffer{},
		ErrOut:      &errOut,
	}
	serve := len(prog.Main)
	for index, inst := range prog.Main {
		if inst.Op == bytecode.OpHttpServerServe {
			serve = index
			break
		}
	}
	machine.run(prog.Main[:serve], env)
	dispatch, ok := env.vars["__http_dispatch"].(*httpreq.Server)
	if !ok {
		t.Fatalf("dispatcher = %T", env.vars["__http_dispatch"])
	}

	health := httptest.NewRecorder()
	dispatch.ServeHTTP(health, httptest.NewRequest(http.MethodGet, "/health?ready=yes", nil))
	if health.Body.String() != "yes" {
		t.Fatalf("health = %q", health.Body.String())
	}
	// Literals stay on the stock mux.
	mux := env.vars["__http_mux"].(*http.ServeMux)
	viaMux := httptest.NewRecorder()
	mux.ServeHTTP(viaMux, httptest.NewRequest(http.MethodGet, "/health?ready=yes", nil))
	if viaMux.Body.String() != "yes" {
		t.Fatalf("mux health = %q", viaMux.Body.String())
	}

	literal := httptest.NewRecorder()
	dispatch.ServeHTTP(literal, httptest.NewRequest(http.MethodGet, "/tasks/new", nil))
	if literal.Body.String() != "literal" {
		t.Fatalf("literal won = %q", literal.Body.String())
	}

	task := httptest.NewRecorder()
	taskReq := httptest.NewRequest(http.MethodGet, "/tasks/foo/../9?status=open", nil)
	taskReq.Header.Set("Authorization", "Bearer z")
	dispatch.ServeHTTP(task, taskReq)
	if task.Body.String() != "9|open||Bearer z|" {
		t.Fatalf("task = %q", task.Body.String())
	}

	notes := httptest.NewRecorder()
	dispatch.ServeHTTP(notes, httptest.NewRequest(http.MethodGet, "/tasks/9/notes/abc", nil))
	if notes.Body.String() != "9/abc" {
		t.Fatalf("notes = %q", notes.Body.String())
	}

	slash := httptest.NewRecorder()
	dispatch.ServeHTTP(slash, httptest.NewRequest(http.MethodGet, "/tasks/9/", nil))
	if slash.Code != http.StatusNotFound {
		t.Fatalf("trailing slash status = %d, body %q", slash.Code, slash.Body.String())
	}
}

func TestHTTPRoutePatternFailClosed(t *testing.T) {
	t.Run("malformed pattern", func(t *testing.T) {
		runVMExpectingPanic(t, []bytecode.BCInstruction{
			{Op: bytecode.OpHttpServerStart, OpString: "HTTP_SERVER_START", StringOperand: "0"},
			{Op: bytecode.OpHttpRoute, OpString: "HTTP_ROUTE", StringOperand: "/tasks/{", StringOperand2: "req", IntOperand: 0},
		}, nil, "TYPE_ERROR")
	})

	t.Run("duplicate skeleton", func(t *testing.T) {
		const src = `(http_server 0
  (route "/tasks/{id}" (lambda (req) (res 200 "text/plain" "a")))
  (route "/tasks/{name}" (lambda (req) (res 200 "text/plain" "b"))))`
		_, prog := parseAndCompile(t, src)
		env := NewBcEnv(nil)
		machine := &BCVM{
			prog:        prog,
			env:         env,
			insts:       prog.Main,
			stores:      newBCStoreRegistry(),
			Limits:      DefaultLimits,
			AllowedCaps: []capability.Capability{capability.Network},
			Out:         &bytes.Buffer{},
			ErrOut:      &bytes.Buffer{},
		}
		defer func() {
			recovered := recover()
			vmErr, ok := recovered.(*VMError)
			if !ok || vmErr.Code != "RUNTIME_ERROR" || !strings.Contains(vmErr.Message, "duplicate path pattern") {
				t.Fatalf("recovered = %#v, want RUNTIME_ERROR duplicate path pattern", recovered)
			}
		}()
		serve := len(prog.Main) - 1
		machine.run(prog.Main[:serve], env)
		t.Fatal("duplicate pattern did not fail")
	})
}

func evalRequestInterp(t *testing.T, expr string, req *http.Request) string {
	t.Helper()
	node := parser.NewParser(lexer.NewLexer(expr), "request.howl").ParseExpression()
	interp := &Interpreter{Out: &bytes.Buffer{}, ErrOut: &bytes.Buffer{}}
	env := NewInterpEnv(nil)
	env.vars["req"] = req
	got := interp.eval(node, env)
	text, ok := got.(string)
	if !ok {
		t.Fatalf("interpreter result = %T %v", got, got)
	}
	return text
}

func evalRequestInterpError(t *testing.T, expr string, req *http.Request) (reason string) {
	t.Helper()
	defer func() {
		recovered := recover()
		err, ok := recovered.(interpError)
		if !ok {
			t.Fatalf("interpreter panic = %#v", recovered)
		}
		reason = err.reason
	}()
	_ = evalRequestInterp(t, expr, req)
	t.Fatal("interpreter succeeded")
	return ""
}

func evalRequestVM(t *testing.T, expr string, req *http.Request, caps []capability.Capability) string {
	t.Helper()
	_, prog := parseAndCompile(t, "(cli_app (print "+expr+"))")
	env := NewBcEnv(nil)
	env.vars["req"] = req
	var out bytes.Buffer
	machine := &BCVM{
		prog:        prog,
		env:         env,
		insts:       prog.Main,
		stores:      newBCStoreRegistry(),
		Limits:      DefaultLimits,
		AllowedCaps: caps,
		Out:         &out,
		ErrOut:      &bytes.Buffer{},
	}
	machine.run(prog.Main, env)
	return strings.TrimSpace(out.String())
}

func evalRequestVMError(t *testing.T, expr string, req *http.Request) (vmErr *VMError) {
	t.Helper()
	_, prog := parseAndCompile(t, "(cli_app (print "+expr+"))")
	env := NewBcEnv(nil)
	env.vars["req"] = req
	machine := &BCVM{
		prog:   prog,
		env:    env,
		insts:  prog.Main,
		stores: newBCStoreRegistry(),
		Limits: DefaultLimits,
		Out:    &bytes.Buffer{},
		ErrOut: &bytes.Buffer{},
	}
	defer func() {
		recovered := recover()
		if recovered == nil {
			t.Fatal("vm succeeded")
		}
		var ok bool
		vmErr, ok = recovered.(*VMError)
		if !ok {
			t.Fatalf("vm panic = %#v", recovered)
		}
	}()
	machine.run(prog.Main, env)
	return nil
}
