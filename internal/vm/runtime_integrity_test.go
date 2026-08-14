package vm

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/howlcipher/howlframe/internal/bytecode"
	"github.com/howlcipher/howlframe/internal/capability"
	"github.com/howlcipher/howlframe/internal/checker"
	"github.com/howlcipher/howlframe/internal/hfir"
)

func captureVMError(run func()) (failure *VMError) {
	defer func() {
		if recovered := recover(); recovered != nil {
			failure, _ = recovered.(*VMError)
		}
	}()
	run()
	return nil
}

func capturePanic(run func()) (recovered any) {
	defer func() { recovered = recover() }()
	run()
	return nil
}

func fileStoreURI(t *testing.T, name string) string {
	t.Helper()
	return "file://" + filepath.Join(t.TempDir(), name)
}

func runStoreOpen(t *testing.T, uri string, caps ...capability.Capability) *VMError {
	t.Helper()
	vm := newStoreTestVM()
	vm.AllowedCaps = caps
	return captureVMError(func() {
		vm.run([]bytecode.BCInstruction{storeInstruction(bytecode.OpStoreOpen, "STORE_OPEN", "store", uri)}, vm.env)
	})
}

func TestFileStoreCapabilitiesAreIndependent(t *testing.T) {
	fileURI := fileStoreURI(t, "store.json")
	filePath := strings.TrimPrefix(fileURI, "file://")

	if failure := runStoreOpen(t, fileURI, capability.Database); failure == nil || failure.Code != "CAPABILITY_DENIED" {
		t.Fatalf("database-only file open failure = %#v, want CAPABILITY_DENIED", failure)
	}
	if _, err := os.Stat(filePath); !os.IsNotExist(err) {
		t.Fatalf("database-only open changed filesystem: stat error = %v", err)
	}
	if failure := runStoreOpen(t, fileURI, capability.Filesystem); failure == nil || failure.Code != "CAPABILITY_DENIED" {
		t.Fatalf("filesystem-only file open failure = %#v, want database CAPABILITY_DENIED", failure)
	}
	if failure := runStoreOpen(t, "memory://session", capability.Database); failure != nil {
		t.Fatalf("memory store with database failed: %#v", failure)
	}
	if failure := runStoreOpen(t, "memory://session", capability.Filesystem); failure == nil || failure.Code != "CAPABILITY_DENIED" {
		t.Fatalf("filesystem-only memory open failure = %#v, want CAPABILITY_DENIED", failure)
	}
	if failure := runStoreOpen(t, fileURI, capability.Database, capability.Filesystem); failure != nil {
		t.Fatalf("file store with both capabilities failed: %#v", failure)
	}
}

func TestDatabaseOnlyCannotReadOrModifyExistingFile(t *testing.T) {
	fileURI := fileStoreURI(t, "existing.json")
	filePath := strings.TrimPrefix(fileURI, "file://")
	original := []byte(`{"existing":{"value":"before"}}`)
	if err := os.WriteFile(filePath, original, 0o600); err != nil {
		t.Fatal(err)
	}

	vm := newStoreTestVM()
	failure := captureVMError(func() {
		vm.run([]bytecode.BCInstruction{storeInstruction(bytecode.OpStoreOpen, "STORE_OPEN", "store", fileURI)}, vm.env)
	})
	if failure == nil || failure.Code != "CAPABILITY_DENIED" {
		t.Fatalf("database-only existing-file failure = %#v, want CAPABILITY_DENIED", failure)
	}
	got, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, original) {
		t.Fatalf("database-only open modified existing file: got %q, want %q", got, original)
	}
}

func TestFileStorePersistsAndDeleteCommits(t *testing.T) {
	uri := fileStoreURI(t, "persistent.json")
	first := newStoreTestVM()
	first.AllowedCaps = []capability.Capability{capability.Database, capability.Filesystem}
	putStoreRecord(first, "store", uri, "task:1", map[string]any{"status": "open"})

	second := newStoreTestVM()
	second.AllowedCaps = []capability.Capability{capability.Database, capability.Filesystem}
	second.run([]bytecode.BCInstruction{
		storeInstruction(bytecode.OpStoreOpen, "STORE_OPEN", "store", uri),
		{Op: bytecode.OpLoadConst, OpString: "LOAD_CONST", ValueOperand: "task:1"},
		storeInstruction(bytecode.OpStoreGet, "STORE_GET", "store", ""),
	}, second.env)
	if got := second.pop(bytecode.OpStoreGet); !reflect.DeepEqual(got, map[string]any{"status": "open"}) {
		t.Fatalf("reopened file store record = %#v", got)
	}

	second.run([]bytecode.BCInstruction{
		{Op: bytecode.OpLoadConst, OpString: "LOAD_CONST", ValueOperand: "task:1"},
		storeInstruction(bytecode.OpStoreDelete, "STORE_DELETE", "store", ""),
	}, second.env)
	contents, err := os.ReadFile(strings.TrimPrefix(uri, "file://"))
	if err != nil {
		t.Fatal(err)
	}
	var persisted map[string]any
	if err := json.Unmarshal(contents, &persisted); err != nil {
		t.Fatal(err)
	}
	if len(persisted) != 0 {
		t.Fatalf("deleted file-store records = %#v, want empty object", persisted)
	}
}

func TestFileStorePersistenceErrorsFailClosed(t *testing.T) {
	t.Run("invalid JSON", func(t *testing.T) {
		uri := fileStoreURI(t, "invalid.json")
		if err := os.WriteFile(strings.TrimPrefix(uri, "file://"), []byte("not-json"), 0o600); err != nil {
			t.Fatal(err)
		}
		failure := runStoreOpen(t, uri, capability.Database, capability.Filesystem)
		if failure == nil || failure.Code != "STORE_INVALID_PERSISTED_JSON" {
			t.Fatalf("invalid JSON failure = %#v", failure)
		}
	})

	t.Run("unreadable path", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "directory")
		if err := os.Mkdir(path, 0o700); err != nil {
			t.Fatal(err)
		}
		failure := runStoreOpen(t, "file://"+path, capability.Database, capability.Filesystem)
		if failure == nil || failure.Code != "STORE_PERSISTENCE_READ_FAILED" {
			t.Fatalf("unreadable path failure = %#v", failure)
		}
	})

	t.Run("write failure", func(t *testing.T) {
		uri := "file://" + filepath.Join(t.TempDir(), "missing", "store.json")
		vm := newStoreTestVM()
		vm.AllowedCaps = []capability.Capability{capability.Database, capability.Filesystem}
		failure := captureVMError(func() {
			vm.run([]bytecode.BCInstruction{
				storeInstruction(bytecode.OpStoreOpen, "STORE_OPEN", "store", uri),
				{Op: bytecode.OpLoadConst, OpString: "LOAD_CONST", ValueOperand: "key"},
				{Op: bytecode.OpLoadConst, OpString: "LOAD_CONST", ValueOperand: map[string]any{"value": "new"}},
				storeInstruction(bytecode.OpStorePut, "STORE_PUT", "store", ""),
			}, vm.env)
		})
		if failure == nil || failure.Code != "STORE_PERSISTENCE_WRITE_FAILED" {
			t.Fatalf("write failure = %#v", failure)
		}
		store := vm.env.vars["store"].(*bcMemoryStore)
		if len(store.records) != 0 {
			t.Fatalf("failed write mutated in-memory records: %#v", store.records)
		}
	})
}

func TestBooleanLiteralsAcrossExecutionPaths(t *testing.T) {
	source := `(cli_app (if true (print true) (print false)) (if false (print false) (print true)))`
	root, legacy := parseAndCompile(t, source)
	var interpreted bytes.Buffer
	if exit := Interpret(root, nil, strings.NewReader(""), &interpreted, &bytes.Buffer{}); exit != 0 {
		t.Fatalf("interpreter exit = %d", exit)
	}
	if interpreted.String() != "true\ntrue\n" {
		t.Fatalf("interpreter boolean output = %q", interpreted.String())
	}
	var bytecodeOutput bytes.Buffer
	if exit := RunBytecode(legacy, nil, nil, strings.NewReader(""), &bytecodeOutput, &bytes.Buffer{}); exit != 0 {
		t.Fatalf("bytecode exit = %d", exit)
	}
	if bytecodeOutput.String() != interpreted.String() {
		t.Fatalf("bytecode boolean output = %q, interpreter = %q", bytecodeOutput.String(), interpreted.String())
	}

	analysis := checker.Check(root)
	if len(analysis.Diagnostics) != 0 {
		t.Fatalf("boolean checker diagnostics = %#v", analysis.Diagnostics)
	}
	graph, err := hfir.LowerAST(root, "boolean_test.howl")
	if err != nil {
		t.Fatal(err)
	}
	direct, diagnostics := hfir.LowerToBytecode(graph)
	if len(diagnostics) != 0 {
		t.Fatalf("direct boolean diagnostics = %#v", diagnostics)
	}
	var directOutput bytes.Buffer
	if exit := RunBytecode(direct, nil, nil, strings.NewReader(""), &directOutput, &bytes.Buffer{}); exit != 0 {
		t.Fatalf("direct bytecode exit = %d", exit)
	}
	if directOutput.String() != interpreted.String() {
		t.Fatalf("direct boolean output = %q, interpreter = %q", directOutput.String(), interpreted.String())
	}

	_, storeProgram := parseAndCompile(t, `(cli_app
  (store_open flags "memory://boolean-test")
  (store_put flags "value" (dict ("ok" true)))
  (let (record (store_get flags "value")) (print (map_get record "ok"))))`)
	var storeOutput bytes.Buffer
	evidence := RunBytecodeWithEvidence(storeProgram, nil, DefaultExecutionPolicy(), []capability.Capability{capability.Database}, strings.NewReader(""), &storeOutput, &bytes.Buffer{}, 0)
	if evidence.RuntimeFailure != nil {
		t.Fatalf("boolean store bytecode failed: %#v", evidence.RuntimeFailure)
	}
	if evidence.ExitCode != 0 {
		t.Fatalf("boolean store bytecode exit = %d", evidence.ExitCode)
	}
	if storeOutput.String() != "true\n" {
		t.Fatalf("boolean store roundtrip output = %q", storeOutput.String())
	}
}

func TestDirectHFIRParseJSONUsesContentBinding(t *testing.T) {
	root, _ := parseAndCompile(t, `(cli_app (let (content (read_file path)) (let (body (parse_json Any content)) (print (map_get body "value")))))`)
	graph, err := hfir.LowerAST(root, "parse_json_test.howl")
	if err != nil {
		t.Fatal(err)
	}
	program, diagnostics := hfir.LowerToBytecode(graph)
	if len(diagnostics) != 0 {
		t.Fatalf("direct parse_json diagnostics = %#v", diagnostics)
	}
	path := filepath.Join(t.TempDir(), "input.json")
	if err := os.WriteFile(path, []byte(`{"value":"direct"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	env := NewBcEnv(nil)
	env.vars["path"] = path
	var out bytes.Buffer
	vm := &BCVM{prog: program, env: env, insts: program.Main, stores: newBCStoreRegistry(), Limits: DefaultLimits, AllowedCaps: []capability.Capability{capability.Filesystem}, Out: &out, ErrOut: &bytes.Buffer{}}
	vm.run(program.Main, env)
	if out.String() != "direct\n" {
		t.Fatalf("direct parse_json output = %q", out.String())
	}
}

func TestEmptyListJSONIsAnArray(t *testing.T) {
	_, program := parseAndCompile(t, `(cli_app (res_json 200 (dict ("items" (list)))))`)
	recorder := httptest.NewRecorder()
	env := NewBcEnv(nil)
	env.vars["w"] = recorder
	vm := &BCVM{prog: program, env: env, insts: program.Main, stores: newBCStoreRegistry(), Limits: DefaultLimits, AllowedCaps: []capability.Capability{capability.Network}}
	vm.run(vm.insts, env)
	var got map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &got); err != nil {
		t.Fatalf("response JSON = %q: %v", recorder.Body.String(), err)
	}
	items, ok := got["items"].([]any)
	if !ok || items == nil || len(items) != 0 {
		t.Fatalf("items = %#v, want explicit empty array", got["items"])
	}
}

func TestHTTPFunctionContextIsRequestScoped(t *testing.T) {
	_, program := parseAndCompile(t, `(http_server 0
  (defun one () (res_header "X-One" "yes"))
  (defun two () (call one))
  (route "/" (lambda (req) (do (call two) (res_json 200 (dict ("ok" true)))))))`)
	env := NewBcEnv(nil)
	vm := &BCVM{prog: program, env: env, insts: program.Main, stores: newBCStoreRegistry(), Limits: DefaultLimits, AllowedCaps: []capability.Capability{capability.Network}}
	route := program.Main[1]
	if route.Op != bytecode.OpHttpRoute {
		t.Fatalf("program instruction 1 = %#v, want HTTP_ROUTE", route)
	}
	vm.run(program.Main[:2+int(route.IntOperand)], env)
	mux := env.vars["__http_mux"].(*http.ServeMux)

	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			recorder := httptest.NewRecorder()
			mux.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/", nil))
			if recorder.Header().Get("X-One") != "yes" {
				t.Errorf("nested function did not set request header: %#v", recorder.Header())
			}
			if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), `"ok":true`) {
				t.Errorf("response = %d %q", recorder.Code, recorder.Body.String())
			}
		}()
	}
	wg.Wait()

	outside := newStoreTestVM()
	outside.AllowedCaps = []capability.Capability{capability.Network}
	recovered := capturePanic(func() {
		outside.prog = program
		outside.run([]bytecode.BCInstruction{{Op: bytecode.OpCall, OpString: "CALL", StringOperand: "one", IntOperand: 0}}, outside.env)
	})
	if recovered == nil || recovered != "no response writer" {
		t.Fatalf("outside-context response helper panic = %#v, want no response writer", recovered)
	}
}
